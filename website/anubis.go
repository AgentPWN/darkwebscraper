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

const anubisOnion = "http://i62huw7ve22rpyw6lnq3kmfump2dmsg4xpveec3ere73njwatrz74gad.onion"

var anubisClient *http.Client
var bodyBytesAnubis []byte

// func initanubisClient() error {

// }

func initAnubisClient() error {
	if anubisClient != nil {
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

	anubisClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Anubis(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}

	if err := initAnubisClient(); err != nil {
		fmt.Println("[Anubis] init failed:", err)
	}

	req, _ := http.NewRequest("GET", anubisOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := anubisClient.Do(req)
	if err != nil {
		panic(err)
	}
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesAnubis, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesAnubis, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytesAnubis)))
	if err != nil {
		panic(err)
	}

	type cardEntry struct {
		company string
		link    string
		desc    string
	}
	var cards []cardEntry
	// fmt.Println(string(bodyBytesAnubis))
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && utils.HasClass(n, "col-sm-4 p-2") {
			entry := cardEntry{}

			var walk func(*html.Node)
			walk = func(c *html.Node) {
				if c.Type == html.ElementNode {
					fmt.Println(c)
					// Company name
					if c.Data == "h5" && utils.HasClass(c, "fw-bold") {
						hasStyle := false
						for _, attr := range c.Attr {
							if attr.Key == "style" {
								hasStyle = true
								break
							}
						}

						if !hasStyle {
							for child := c.FirstChild; child != nil; child = child.NextSibling {
								if child.Type == html.TextNode {
									entry.company += child.Data
								}
							}
							entry.company = strings.TrimSpace(entry.company)
						} else {
							// Description
							for child := c.FirstChild; child != nil; child = child.NextSibling {
								if child.Type == html.TextNode {
									entry.desc += child.Data
								}
							}
							entry.desc = strings.TrimSpace(entry.desc)
						}
					}

					// Link
					if c.Data == "a" && utils.HasClass(c, "btn") {
						for _, attr := range c.Attr {
							if attr.Key == "href" {
								entry.link = strings.TrimSpace(attr.Val)
								break
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
	fmt.Println(cards)
	for query := range channel {
		query = strings.TrimSpace(query)
		for _, card := range cards {
			if strings.Contains(strings.ToLower(card.company), strings.ToLower(query)) || strings.Contains(strings.ToLower(card.desc), strings.ToLower(query)) {
				link := anubisOnion + card.link
				// if !strings.HasPrefix(link, "http") {
				// 	link = baseURL + strings.TrimPrefix(link, "/")
				// }
				data.Source = "anubis"
				data.Key = query
				data.Url = anubisOnion + link
				data.Desc = card.desc
				chanDataForDb <- data
				fmt.Println("[Anubis] Results found: ", data.Key, data.Url)
			}
		}
	}
}
