package website

import (
	"compress/gzip"
	"crypto/tls"
	"darkwebscraper/utils"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/proxy"
)

const rhysidaOnion = "http://rhysidafc6lm7qa2mkiukbezh7zuth3i4wof4mh2audkymscjm6yegad.onion/"
const rhysidaArchive = rhysidaOnion + "archive.php"

var rhysidaClient *http.Client

func initRhysidaClient() error {
	if rhysidaClient != nil {
		return nil
	}

	torDialer, err := proxy.SOCKS5("tcp", "localhost:9050", nil, nil)
	if err != nil {
		return fmt.Errorf("proxy.SOCKS5: %w", err)
	}

	transport := &http.Transport{
		DialContext: torDialer.(proxy.ContextDialer).DialContext,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	rhysidaClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}

	return nil
}

func Rhysida(channel chan string, chanDataForDb chan utils.DataForDb) {
	if err := initRhysidaClient(); err != nil {
		fmt.Println("[Rhysida] init failed:", err)
		return
	}

	req, err := http.NewRequest("GET", rhysidaArchive, nil)
	if err != nil {
		fmt.Println("[Rhysida] request build failed:", err)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := rhysidaClient.Do(req)
	if err != nil {
		fmt.Println("[Rhysida] request failed:", err)
		return
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			fmt.Println("[Rhysida] gzip reader failed:", err)
			return
		}
		bodyBytes, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			fmt.Println("[Rhysida] read gzip body failed:", err)
			return
		}
	} else {
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("[Rhysida] read body failed:", err)
			return
		}
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytes)))
	if err != nil {
		fmt.Println("[Rhysida] HTML parse failed:", err)
		return
	}

	type cardEntry struct {
		company     string
		website     string
		summary     string
		dataCatalog string
		note        string
		docs        []string
	}
	var cards []cardEntry

	var innerText func(*html.Node) string
	innerText = func(n *html.Node) string {
		if n.Type == html.TextNode {
			return n.Data
		}
		var sb strings.Builder
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			sb.WriteString(innerText(c))
		}
		return sb.String()
	}

	var normalizeURL = func(v string) string {
		v = strings.TrimSpace(v)
		if v == "" {
			return ""
		}
		if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
			return v
		}
		if strings.HasPrefix(v, "/") {
			return rhysidaOnion + strings.TrimPrefix(v, "/")
		}
		return rhysidaOnion + v
	}

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "border") && hasClass(n, "p-2") {
			entry := cardEntry{}

			var walk func(*html.Node)
			walk = func(c *html.Node) {
				if c.Type == html.ElementNode {
					if c.Data == "div" && hasClass(c, "h4") {
						for a := c.FirstChild; a != nil; a = a.NextSibling {
							if a.Type == html.ElementNode && a.Data == "a" {
								entry.company = strings.TrimSpace(innerText(a))
								for _, attr := range a.Attr {
									if attr.Key == "href" {
										entry.website = normalizeURL(attr.Val)
									}
								}
								break
							}
						}
					}

					if c.Data == "a" {
						label := strings.TrimSpace(innerText(c))
						if strings.Contains(strings.ToLower(label), "documents") {
							for _, attr := range c.Attr {
								if attr.Key == "href" {
									url := normalizeURL(attr.Val)
									if url != "" {
										entry.docs = append(entry.docs, url)
									}
								}
							}
						}
					}

					if c.Data == "p" {
						pText := strings.TrimSpace(innerText(c))
						if strings.Contains(pText, "Data Catalog:") {
							entry.dataCatalog = pText
						}
						if strings.Contains(strings.ToLower(pText), "all files was uploaded") {
							entry.note = pText
						}
					}

					if c.Data == "div" && hasClass(c, "m-2") && !hasClass(c, "h4") {
						text := strings.TrimSpace(innerText(c))
						if text != "" && !strings.Contains(strings.ToLower(text), "data catalog:") && !strings.Contains(strings.ToLower(text), "all files was uploaded") {
							if entry.summary == "" || len(text) > len(entry.summary) {
								entry.summary = text
							}
						}
					}
				}

				for child := c.FirstChild; child != nil; child = child.NextSibling {
					walk(child)
				}
			}
			walk(n)

			if entry.company != "" {
				cards = append(cards, entry)
			}
			return
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	for query := range channel {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}
		needle := strings.ToLower(query)
		for _, card := range cards {
			haystack := strings.ToLower(card.company + " " + card.summary + " " + card.website)
			if !strings.Contains(haystack, needle) {
				continue
			}

			url := card.website
			if len(card.docs) > 0 {
				url = card.docs[0]
			}
			if url == "" {
				url = rhysidaArchive
			}

			descParts := []string{}
			if card.summary != "" {
				descParts = append(descParts, card.summary)
			}
			if card.dataCatalog != "" {
				descParts = append(descParts, card.dataCatalog)
			}
			if card.note != "" {
				descParts = append(descParts, card.note)
			}
			for i, d := range card.docs {
				descParts = append(descParts, fmt.Sprintf("doc%d: %s", i+1, d))
			}

			chanDataForDb <- utils.DataForDb{
				Source: "rhysida",
				Key:    query,
				Url:    url,
				Desc:   strings.Join(descParts, " | "),
			}
			fmt.Println("[Rhysida] Results found:", card.company, url)
		}
	}
}
