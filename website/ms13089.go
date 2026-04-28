package website

import (
	"compress/gzip"
	"crypto/tls"
	"darkwebscraper/utils"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/proxy"
)

const ms13089Onion = "http://msleakjir7pxbe6onlqe5uwgvdmy6nq4mnwfy7ojswbhnleenm77vgad.onion/index.html"

var ms13089Client *http.Client
var bodyBytesMs13089 []byte

func initMs13089Client() error {
	if ms13089Client != nil {
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

	ms13089Client = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Ms13089(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}

	if err := initMs13089Client(); err != nil {
		fmt.Println("[Ms13089] init failed:", err)
	}

	req, _ := http.NewRequest("GET", ms13089Onion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := ms13089Client.Do(req)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesMs13089, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesMs13089, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytesMs13089)))
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

	// Extracts path from onclick="location.href='/cl/clicks.php?uri=URO/index.html'"
	onclickRe := regexp.MustCompile(`location\.href='([^']+)'`)

	var f func(*html.Node)
	f = func(n *html.Node) {
		// Each entry is a <div class="post ...">
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "post") {
			entry := cardEntry{}

			var walk func(*html.Node)
			walk = func(c *html.Node) {
				if c.Type == html.ElementNode {
					// Company name is the first plain <div> inside post-title-block
					if c.Data == "div" && hasClass(c, "post-title-block") {
						for child := c.FirstChild; child != nil; child = child.NextSibling {
							if child.Type == html.ElementNode && child.Data == "div" && !hasClass(child, "post-timer-end") {
								entry.company = strings.TrimSpace(innerText(child))
								break
							}
						}
					}
					// Description is in <div class="post-text">
					if c.Data == "div" && hasClass(c, "post-text") {
						entry.desc = strings.TrimSpace(innerText(c))
					}
					// Link is on <a class="post-more-link"> via onclick
					if c.Data == "a" && hasClass(c, "post-more-link") {
						for _, attr := range c.Attr {
							if attr.Key == "onclick" {
								m := onclickRe.FindStringSubmatch(attr.Val)
								if len(m) > 1 {
									entry.link = strings.TrimSpace(m[1])
								}
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
			return // don't recurse into the post itself again
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	baseURL := "http://msleakjir7pxbe6onlqe5uwgvdmy6nq4mnwfy7ojswbhnleenm77vgad.onion"

	for query := range channel {
		query = strings.TrimSpace(query)
		for _, card := range cards {
			fmt.Println(card.company)
			if strings.Contains(card.company, query) {
				url := baseURL + card.link
				data.Source = "ms13089"
				data.Key = query
				data.Url = url
				data.Desc = card.desc
				chanDataForDb <- data
				fmt.Println("[Ms13089] Results found: ", data.Key, data.Url)
			}
		}
	}
}
