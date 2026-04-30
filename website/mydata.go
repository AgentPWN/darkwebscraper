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

const mydataOnion = "http://mydatae2d63il5oaxxangwnid5loq2qmtsol2ozr6vtb7yfm5ypzo6id.onion/blog"

var mydataClient *http.Client
var bodyBytesMydata []byte

func initMydataClient() error {
	if mydataClient != nil {
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

	mydataClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Mydata(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}

	if err := initMydataClient(); err != nil {
		fmt.Println("[Mydata] init failed:", err)
	}

	req, _ := http.NewRequest("GET", mydataOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := mydataClient.Do(req)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesMydata, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesMydata, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytesMydata)))
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
		// Each entry is a <div class="b_block">
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "b_block") {
			entry := cardEntry{}

			var walk func(*html.Node)
			walk = func(c *html.Node) {
				if c.Type == html.ElementNode {
					// Link href is on <a class="a_title"> — the visible text is the URL, not the company name
					if c.Data == "a" && hasClass(c, "a_title") {
						for _, attr := range c.Attr {
							if attr.Key == "href" {
								entry.link = strings.TrimSpace(attr.Val)
							}
						}
					}
					// Text block is a direct child div of news_div with no class.
					// Content: company name <br> description <br> ...
					// Split on <br> to separate company name from description.
					if c.Data == "div" && hasClass(c, "news_div") {
						for child := c.FirstChild; child != nil; child = child.NextSibling {
							if child.Type == html.ElementNode && child.Data == "div" && !hasClass(child, "news_title") {
								var segments []string
								var seg strings.Builder
								var collectText func(*html.Node)
								collectText = func(node *html.Node) {
									if node.Type == html.TextNode {
										seg.WriteString(node.Data)
									} else if node.Type == html.ElementNode && node.Data == "br" {
										if s := strings.TrimSpace(seg.String()); s != "" {
											segments = append(segments, s)
										}
										seg.Reset()
									} else {
										for gc := node.FirstChild; gc != nil; gc = gc.NextSibling {
											collectText(gc)
										}
									}
								}
								for gc := child.FirstChild; gc != nil; gc = gc.NextSibling {
									collectText(gc)
								}
								if s := strings.TrimSpace(seg.String()); s != "" {
									segments = append(segments, s)
								}
								if len(segments) > 0 {
									entry.company = segments[0]
								}
								if len(segments) > 1 {
									entry.desc = strings.Join(segments[1:], " ")
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
			return // don't recurse into the block itself again
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	// Base URL for resolving relative hrefs (e.g. "blog_1-31" -> full onion URL)
	// baseURL := "http://mydatae2d63il5oaxxangwnid5loq2qmtsol2ozr6vtb7yfm5ypzo6id.onion/"

	for query := range channel {
		query = strings.TrimSpace(query)
		for _, card := range cards {
			if strings.Contains(card.company, query) {
				link := mydataOnion + card.link
				// if !strings.HasPrefix(link, "http") {
				// 	link = baseURL + strings.TrimPrefix(link, "/")
				// }
				data.Source = "mydata"
				data.Key = query
				data.Url = link
				data.Desc = card.desc
				chanDataForDb <- data
				fmt.Println("[Mydata] Results found: ", data.Key, data.Url)
			}
		}
	}
}
