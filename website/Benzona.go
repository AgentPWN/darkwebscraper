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

const benzonaOnion = "http://benzona6x5ggng3hx52h4mak5sgx5vukrdlrrd3of54g2uppqog2joyd.onion/"

var benzonaClient *http.Client
var bodyBytesBenzona []byte

func initBenzonaClient() error {
	if benzonaClient != nil {
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

	benzonaClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Benzona(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}

	if err := initBenzonaClient(); err != nil {
		fmt.Println("[Benzona] init failed:", err)
	}

	req, _ := http.NewRequest("GET", benzonaOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := benzonaClient.Do(req)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesBenzona, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesBenzona, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytesBenzona)))
	if err != nil {
		panic(err)
	}

	type cardEntry struct {
		company string
		link    string
		desc    string
	}
	var cards []cardEntry

	// innerText collects all text content under a node, stripping tags.
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
		// Each card is a <div> with class "card"
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "card") {
			entry := cardEntry{}

			var walk func(*html.Node)
			walk = func(c *html.Node) {
				if c.Type == html.ElementNode {
					// Company name is the text inside <h3>
					if c.Data == "h3" {
						entry.company = strings.TrimSpace(innerText(c))
						// Use the company name as the link path (e.g. ccbrt.org -> /ccbrt.org)
						entry.link = entry.company
					}
					// Collect all <p> text for the description, skipping label prefixes
					if c.Data == "p" {
						line := strings.TrimSpace(innerText(c))
						if line != "" {
							if entry.desc == "" {
								entry.desc = line
							} else {
								entry.desc += " | " + line
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
				url := benzonaOnion + card.link
				data.Source = "benzona"
				data.Key = query
				data.Url = url
				data.Desc = card.desc
				chanDataForDb <- data
				fmt.Println("[Benzona] Results found: ", data.Key, data.Url)
			}
		}
	}
}
