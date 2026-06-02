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

const secpoOnion = "http://secponewsxgrlnirowclps2kllzaotaf5w2bsvktdnz4qhjr2jnwvvyd.onion/"

var secpoClient *http.Client

func initSecpoClient() error {
	if secpoClient != nil {
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

	secpoClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Secpo(channel chan string, chanDataForDb chan utils.DataForDb) {
	if err := initSecpoClient(); err != nil {
		fmt.Println("[Secpo] init failed:", err)
		return
	}

	req, err := http.NewRequest("GET", secpoOnion, nil)
	if err != nil {
		fmt.Println("[Secpo] request build failed:", err)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := secpoClient.Do(req)
	if err != nil {
		fmt.Println("[Secpo] request failed:", err)
		return
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			fmt.Println("[Secpo] gzip reader failed:", err)
			return
		}
		bodyBytes, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			fmt.Println("[Secpo] read gzip body failed:", err)
			return
		}
	} else {
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("[Secpo] read body failed:", err)
			return
		}
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytes)))
	if err != nil {
		fmt.Println("[Secpo] HTML parse failed:", err)
		return
	}

	type postEntry struct {
		title string
		date  string
		body  string
		link  string
	}
	var posts []postEntry

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

	normalizeURL := func(v string) string {
		v = strings.TrimSpace(v)
		if v == "" {
			return ""
		}
		if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
			return v
		}
		if strings.HasPrefix(v, "//") {
			return "http:" + v
		}
		if strings.HasPrefix(v, "/") {
			return secpoOnion + strings.TrimPrefix(v, "/")
		}
		return secpoOnion + v
	}

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "post") {
			entry := postEntry{}

			var walk func(*html.Node)
			walk = func(c *html.Node) {
				if c.Type == html.ElementNode {
					if c.Data == "a" && hasClass(c, "header-a") {
						for _, attr := range c.Attr {
							if attr.Key == "href" {
								entry.link = normalizeURL(attr.Val)
							}
						}
						for cc := c.FirstChild; cc != nil; cc = cc.NextSibling {
							if cc.Type == html.ElementNode && cc.Data == "h2" {
								entry.title = strings.TrimSpace(innerText(cc))
							}
						}
					}
					if c.Data == "div" && hasClass(c, "metadata") {
						entry.date = strings.TrimSpace(innerText(c))
					}
					if c.Data == "div" && hasClass(c, "text") {
						bodyText := strings.TrimSpace(innerText(c))
						bodyText = strings.Join(strings.Fields(bodyText), " ")
						entry.body = bodyText
					}
				}

				for child := c.FirstChild; child != nil; child = child.NextSibling {
					walk(child)
				}
			}
			walk(n)

			if entry.title != "" {
				posts = append(posts, entry)
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
		for _, post := range posts {
			haystack := strings.ToLower(post.title + " " + post.body)
			if !strings.Contains(haystack, needle) {
				continue
			}

			url := post.link
			if url == "" {
				url = secpoOnion
			}

			chanDataForDb <- utils.DataForDb{
				Source: "secpo",
				Key:    query,
				Url:    url,
				Desc:   strings.TrimSpace(post.date + " | " + post.body),
			}
			fmt.Println("[Secpo] Results found:", post.title, url)
		}
	}
}
