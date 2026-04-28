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

const blackwaterOnion = "http://ejzl7cjxmkx7lzhiqwidmrwtfjv45pkczbc4fnyaut3t7gll3yaiq5id.onion/"

var blackwaterClient *http.Client
var bodyBytesBlackwater []byte

func initBlackwaterClient() error {
	if blackwaterClient != nil {
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

	blackwaterClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Blackwater(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}

	if err := initBlackwaterClient(); err != nil {
		fmt.Println("[Blackwater] init failed:", err)
	}

	req, _ := http.NewRequest("GET", blackwaterOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := blackwaterClient.Do(req)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesBlackwater, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesBlackwater, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytesBlackwater)))
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
		// Each card is a <div class="card ...">
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "card") {
			entry := cardEntry{}

			var walk func(*html.Node)
			walk = func(c *html.Node) {
				if c.Type == html.ElementNode {
					// Company name is in <h5 class="card-title">
					if c.Data == "h5" && hasClass(c, "card-title") {
						entry.company = strings.TrimSpace(innerText(c))
					}
					// Description is in <p class="card-text"> — skip the date line, take the content line
					if c.Data == "p" && hasClass(c, "card-text") && !hasClass(c, "text-muted") {
						line := strings.TrimSpace(innerText(c))
						if line != "" {
							entry.desc = line
						}
					}
					// Fallback: grab first non-date card-text as desc if above didn't match
					if c.Data == "p" && hasClass(c, "card-text") && entry.desc == "" {
						line := strings.TrimSpace(innerText(c))
						// skip the "Publicated at ..." line
						if line != "" && !strings.HasPrefix(line, "Publicated") {
							entry.desc = line
						}
					}
					// Link is on the <a class="btn"> — href="/blog?uuid=..."
					if c.Data == "a" && hasClass(c, "btn") {
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
				url := blackwaterOnion + strings.TrimPrefix(card.link, "/")
				data.Source = "blackwater"
				data.Key = query
				data.Url = url
				data.Desc = card.desc
				chanDataForDb <- data
				fmt.Println("[Blackwater] Results found: ", data.Key, data.Url)
			}
		}
	}
}
