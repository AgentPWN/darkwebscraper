// this website doesn't have a login page but the login logic from pwnForums has been refactored for ease

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

const insomniaOnion = "http://i62huw7ve22rpyw6lnq3kmfump2dmsg4xpveec3ere73njwatrz74gad.onion"

var insomniaClient *http.Client
var bodyBytesInsomnia []byte

// func initinsomniaClient() error {

// }

func initInsomniaClient() error {
	if insomniaClient != nil {
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

	insomniaClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Insomnia(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}

	if err := initInsomniaClient(); err != nil {
		fmt.Println("[Insomnia] init failed:", err)
	}

	req, _ := http.NewRequest("GET", insomniaOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := insomniaClient.Do(req)
	if err != nil {
		panic(err)
	}
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesInsomnia, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesInsomnia, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytesInsomnia)))
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
		if n == nil {
			return ""
		}
		if n.Type == html.TextNode {
			return n.Data
		}

		var b strings.Builder
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			b.WriteString(innerText(c))
		}
		return strings.TrimSpace(b.String())
	}

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "book-card") {
			entry := cardEntry{}

			var walk func(*html.Node)
			walk = func(c *html.Node) {
				if c.Type == html.ElementNode {

					// <h3 class="info-title"><a href="/Company/TVG/">The Vant Group</a></h3>
					if c.Data == "h3" && hasClass(c, "info-title") {
						for child := c.FirstChild; child != nil; child = child.NextSibling {
							if child.Type == html.ElementNode && child.Data == "a" {
								for _, attr := range child.Attr {
									if attr.Key == "href" {
										entry.link = strings.TrimSpace(attr.Val)
									}
								}

								for gc := child.FirstChild; gc != nil; gc = gc.NextSibling {
									if gc.Type == html.TextNode {
										entry.company += gc.Data
									}
								}
								entry.company = strings.TrimSpace(entry.company)
							}
						}
					}

					// <p class="info-desc">...</p>
					if c.Data == "p" && hasClass(c, "info-desc") {
						for child := c.FirstChild; child != nil; child = child.NextSibling {
							if child.Type == html.TextNode {
								entry.desc += child.Data
							}
						}
						entry.desc = strings.TrimSpace(entry.desc)
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
		for _, card := range cards {
			if strings.Contains(strings.ToLower(card.company), strings.ToLower(query)) || strings.Contains(strings.ToLower(card.desc), strings.ToLower(query)) {
				link := insomniaOnion + card.link
				// if !strings.HasPrefix(link, "http") {
				// 	link = baseURL + strings.TrimPrefix(link, "/")
				// }
				data.Source = "insomnia"
				data.Key = query
				data.Url = insomniaOnion + link
				data.Desc = card.desc
				chanDataForDb <- data
				fmt.Println("[Insomnia] Results found: ", data.Key, data.Url)
			}
		}
	}
}
