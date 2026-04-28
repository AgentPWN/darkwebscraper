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

const braincipherOnion = "http://vkvsgl7lhipjirmz6j5ubp3w3bwvxgcdbpi3fsbqngfynetqtw4w5hyd.onion/"

var braincipherClient *http.Client
var bodyBytesBraincipher []byte

func initBraincipherClient() error {
	if braincipherClient != nil {
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

	braincipherClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Braincipher(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}

	if err := initBraincipherClient(); err != nil {
		fmt.Println("[Braincipher] init failed:", err)
	}

	req, _ := http.NewRequest("GET", braincipherOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := braincipherClient.Do(req)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesBraincipher, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesBraincipher, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytesBraincipher)))
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
		// Each entry is an <article class="news-item">
		if n.Type == html.ElementNode && n.Data == "article" && hasClass(n, "news-item") {
			entry := cardEntry{}

			var walk func(*html.Node)
			walk = func(c *html.Node) {
				if c.Type == html.ElementNode {
					// Company name + link are in <h2 class="headline"><a href="...">
					if c.Data == "h2" && hasClass(c, "headline") {
						for child := c.FirstChild; child != nil; child = child.NextSibling {
							if child.Type == html.ElementNode && child.Data == "a" {
								entry.company = strings.TrimSpace(innerText(child))
								for _, attr := range child.Attr {
									if attr.Key == "href" {
										entry.link = strings.TrimSpace(attr.Val)
									}
								}
							}
						}
					}
					// Description is in <p class="summary">
					if c.Data == "p" && hasClass(c, "summary") {
						entry.desc = strings.TrimSpace(innerText(c))
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
			return // don't recurse into the article itself again
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
				url := braincipherOnion + strings.TrimPrefix(card.link, "/")
				data.Source = "braincipher"
				data.Key = query
				data.Url = url
				data.Desc = card.desc
				chanDataForDb <- data
				fmt.Println("[Braincipher] Results found: ", data.Key, data.Url)
			}
		}
	}
}
