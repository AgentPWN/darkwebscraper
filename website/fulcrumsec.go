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

const fulcrumsecOnion = "http://4e3p3in2bl67hxchuwza7qvnpe7pyeloyztr5fnh257fxkovfhappjyd.onion/"

var fulcrumsecClient *http.Client
var bodyBytesFulcrumsec []byte

func initFulcrumsecClient() error {
	if fulcrumsecClient != nil {
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

	fulcrumsecClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Fulcrumsec(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}

	if err := initFulcrumsecClient(); err != nil {
		fmt.Println("[Fulcrumsec] init failed:", err)
		return
	}

	req, _ := http.NewRequest("GET", fulcrumsecOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := fulcrumsecClient.Do(req)
	if err != nil {
		fmt.Println("[Fulcrumsec] request failed:", err)
		return
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			fmt.Println("[Fulcrumsec] gzip reader error:", err)
			return
		}
		bodyBytesFulcrumsec, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			fmt.Println("[Fulcrumsec] gzip read error:", err)
			return
		}
	} else {
		bodyBytesFulcrumsec, err = io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("[Fulcrumsec] body read error:", err)
			return
		}
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytesFulcrumsec)))
	if err != nil {
		panic(err)
	}

	type cardEntry struct {
		company string
		link    string
		desc    string
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

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" && hasClass(n, "card") {
			entry := cardEntry{}

			for _, attr := range n.Attr {
				if attr.Key == "href" {
					entry.link = strings.TrimSpace(attr.Val)
				}
			}

			var walk func(*html.Node)
			walk = func(c *html.Node) {
				if c.Type == html.ElementNode {
					if c.Data == "img" {
						for _, attr := range c.Attr {
							if attr.Key == "alt" && entry.company == "" {
								entry.company = strings.TrimSpace(attr.Val)
							}
						}
					}
					if entry.company == "" && c.Data == "div" {
						text := strings.TrimSpace(innerText(c))
						if text != "" {
							entry.company = text
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

		for _, card := range cards {
			if strings.Contains(strings.ToLower(card.company), strings.ToLower(query)) || strings.Contains(strings.ToLower(card.link), strings.ToLower(query)) {
				url := fulcrumsecOnion + strings.TrimPrefix(card.link, "/")
				data.Source = "fulcrumsec"
				data.Key = query
				data.Url = url
				data.Desc = card.desc
				chanDataForDb <- data
				fmt.Println("[Fulcrumsec] Results found:", data.Key, data.Url)
			}
		}
	}
}
