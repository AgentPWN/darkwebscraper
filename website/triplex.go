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

const triplexOnion = "http://ojcmpbdncjo5dhaxxll44bq6to3kwqtoeraevgsjquhdtt4uv5l4igid.onion/"

var triplexClient *http.Client

func initTriplexClient() error {
	if triplexClient != nil {
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

	triplexClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Triplex(channel chan string, chanDataForDb chan utils.DataForDb) {
	if err := initTriplexClient(); err != nil {
		fmt.Println("[Triplex] init failed:", err)
		return
	}

	req, err := http.NewRequest("GET", triplexOnion, nil)
	if err != nil {
		fmt.Println("[Triplex] request build failed:", err)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := triplexClient.Do(req)
	if err != nil {
		fmt.Println("[Triplex] request failed:", err)
		return
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			fmt.Println("[Triplex] gzip reader failed:", err)
			return
		}
		bodyBytes, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			fmt.Println("[Triplex] read gzip body failed:", err)
			return
		}
	} else {
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("[Triplex] read body failed:", err)
			return
		}
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytes)))
	if err != nil {
		fmt.Println("[Triplex] HTML parse failed:", err)
		return
	}

	type postEntry struct {
		title string
		date  string
		body  string
		links []string
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
		v = strings.TrimPrefix(v, "view-source:")
		if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
			return v
		}
		if strings.HasPrefix(v, "//") {
			return "http:" + v
		}
		if strings.HasPrefix(v, "/") {
			return triplexOnion + strings.TrimPrefix(v, "/")
		}
		if strings.HasPrefix(v, "www.") {
			return "https://" + v
		}
		return triplexOnion + v
	}

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "post") {
			entry := postEntry{}

			var walk func(*html.Node)
			walk = func(c *html.Node) {
				if c.Type == html.ElementNode {
					if c.Data == "h2" && hasClass(c, "post-title") {
						entry.title = strings.TrimSpace(innerText(c))
					}
					if c.Data == "div" && hasClass(c, "post-date") {
						entry.date = strings.TrimSpace(innerText(c))
					}
					if c.Data == "div" && hasClass(c, "post-content") {
						contentText := strings.TrimSpace(innerText(c))
						contentText = strings.Join(strings.Fields(contentText), " ")
						if contentText != "" {
							entry.body = contentText
						}
					}
					if c.Data == "a" {
						for _, attr := range c.Attr {
							if attr.Key == "href" {
								url := normalizeURL(attr.Val)
								if url != "" {
									entry.links = append(entry.links, url)
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

			url := triplexOnion
			for _, l := range post.links {
				if strings.Contains(strings.ToLower(l), ".onion") {
					url = l
					break
				}
			}

			descParts := []string{}
			if post.date != "" {
				descParts = append(descParts, "date: "+post.date)
			}
			if post.body != "" {
				descParts = append(descParts, post.body)
			}
			for i, l := range post.links {
				descParts = append(descParts, fmt.Sprintf("link%d: %s", i+1, l))
			}

			chanDataForDb <- utils.DataForDb{
				Source: "triple x",
				Key:    query,
				Url:    url,
				Desc:   strings.Join(descParts, " | "),
			}
			fmt.Println("[Triplex] Results found:", post.title, url)
		}
	}
}
