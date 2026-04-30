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

const metaencryptorOnion = "https://metacrptmytukkj7ajwjovdpjqzd7esg5v3sg344uzhigagpezcqlpyd.onion/"

var metaencryptorClient *http.Client
var bodyBytesMetaencryptor []byte

func initMetaencryptorClient() error {
	if metaencryptorClient != nil {
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

	metaencryptorClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Metaencryptor(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}

	if err := initMetaencryptorClient(); err != nil {
		fmt.Println("[Metaencryptor] init failed:", err)
	}

	req, _ := http.NewRequest("GET", metaencryptorOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := metaencryptorClient.Do(req)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesMetaencryptor, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesMetaencryptor, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytesMetaencryptor)))
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
		// Each entry is a <div class="lt-card">
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "lt-card") {
			entry := cardEntry{}

			var walk func(*html.Node)
			walk = func(c *html.Node) {
				if c.Type == html.ElementNode {
					// Company name is in <span class="lt-card-name">
					if c.Data == "span" && hasClass(c, "lt-card-name") {
						entry.company = strings.TrimSpace(innerText(c))
					}
					// Description is in <p class="lt-card-memo">
					if c.Data == "p" && hasClass(c, "lt-card-memo") {
						entry.desc = strings.TrimSpace(innerText(c))
					}
					// Link is on <a class="lt-btn-site"> — the external website href
					if c.Data == "a" && hasClass(c, "lt-btn-site") {
						for _, attr := range c.Attr {
							if attr.Key == "href" {
								entry.link = strings.TrimSpace(attr.Val)
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
			return // don't recurse into the card itself again
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	for query := range channel {
		query = strings.TrimSpace(query)
		for _, card := range cards {
			if strings.Contains(card.company, query) {
				data.Source = "metaencryptor"
				data.Key = query
				data.Url = card.link
				data.Desc = card.desc
				chanDataForDb <- data
				// fmt.Printf("[Metaencryptor] ch addr: %p\n", chanDataForDb)
				fmt.Println("[Metaencryptor] Results found: ", data)
			}
		}
	}
}
