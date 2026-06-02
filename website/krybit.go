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

const krybitOnion = "http://krybieodq754vlwufrsuxaswxb5zpxyibaawmed2jaduoz2e5m56hmid.onion/"

var krybitClient *http.Client
var bodyBytesKrybit []byte

func initKrybitClient() error {
	if krybitClient != nil {
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

	krybitClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Krybit(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}

	if err := initKrybitClient(); err != nil {
		fmt.Println("[Krybit] init failed:", err)
	}

	req, _ := http.NewRequest("GET", krybitOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := krybitClient.Do(req)
	if err != nil {
		fmt.Println("[Krybit] request failed:", err)
		return
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesKrybit, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesKrybit, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytesKrybit)))
	if err != nil {
		panic(err)
	}

	type postEntry struct {
		title  string
		desc   string
		views  string
		status string
		link   string
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

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "post-card") {
			entry := postEntry{}
			// Extract link from onclick
			for _, attr := range n.Attr {
				if attr.Key == "onclick" && strings.Contains(attr.Val, "window.location") {
					link := strings.TrimPrefix(attr.Val, "window.location=")
					link = strings.Trim(link, "'/\"")
					entry.link = krybitOnion + strings.TrimPrefix(link, "/")
				}
			}
			var header, content *html.Node
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if hasClass(c, "post-header") {
					header = c
				}
				if hasClass(c, "post-content") {
					content = c
				}
			}
			if header != nil {
				for c := header.FirstChild; c != nil; c = c.NextSibling {
					if hasClass(c, "post-status") {
						entry.status = innerText(c)
					}
					if hasClass(c, "post-views") {
						entry.views = innerText(c)
					}
				}
			}
			if content != nil {
				for c := content.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.ElementNode && c.Data == "div" {
						for gc := c.FirstChild; gc != nil; gc = gc.NextSibling {
							if gc.Type == html.ElementNode && gc.Data == "h3" && hasClass(gc, "post-title") {
								entry.title = innerText(gc)
							}
							if gc.Type == html.ElementNode && hasClass(gc, "post-excerpt") {
								entry.desc = innerText(gc)
							}
						}
					}
				}
			}
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
		for _, post := range posts {
			if strings.Contains(strings.ToLower(post.title), strings.ToLower(query)) {
				data.Source = "krybit"
				data.Key = query
				data.Url = post.link
				desc := post.desc + " | Status: " + post.status + ", Views: " + post.views
				data.Desc = desc
				chanDataForDb <- data
				fmt.Println("[Krybit] Results found:", data.Key, data.Url)
			}
		}
	}
}
