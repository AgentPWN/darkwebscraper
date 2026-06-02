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

const netrunnerOnion = "http://netrunrsb3bivj5gnwajzxlig5qkteb6edgthxj7fmsvhkzxtwfxwaad.onion/"

var netrunnerClient *http.Client
var bodyBytesNetrunner []byte

func initNetrunnerClient() error {
	if netrunnerClient != nil {
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

	netrunnerClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

// Netrunner scrapes the netrunner onion site for company blog items and emits results in standard format.
func Netrunner(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}

	if err := initNetrunnerClient(); err != nil {
		fmt.Println("[Netrunner] init failed:", err)
	}

	req, _ := http.NewRequest("GET", netrunnerOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := netrunnerClient.Do(req)
	if err != nil {
		fmt.Println("[Netrunner] request failed:", err)
		return
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return
		}
		bodyBytesNetrunner, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			return
		}
	} else {
		bodyBytesNetrunner, err = io.ReadAll(resp.Body)
		if err != nil {
			return
		}
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytesNetrunner)))
	if err != nil {
		fmt.Println("[Netrunner] HTML parse failed:", err)
		return
	}

	type blogEntry struct {
		company  string
		logo     string
		websites []string
		location string
		stock    string
		revenue  string
		employee string
		status   string
		desc     string
		dataLink string
	}
	var blogs []blogEntry

	// Helper to get inner text
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

	// Helper to get attribute value
	getAttr := func(n *html.Node, key string) string {
		for _, attr := range n.Attr {
			if attr.Key == key {
				return attr.Val
			}
		}
		return ""
	}

	// Helper to check class
	hasClass := func(n *html.Node, class string) bool {
		for _, attr := range n.Attr {
			if attr.Key == "class" && strings.Contains(attr.Val, class) {
				return true
			}
		}
		return false
	}

	// Traverse DOM to find blog-item divs
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "blog-item") {
			entry := blogEntry{}

			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode {
					switch {
					case hasClass(c, "blog-item-title"):
						entry.company = strings.TrimSpace(innerText(c))
					case hasClass(c, "blog-item-logo") && c.Data == "img":
						entry.logo = getAttr(c, "src")
					case hasClass(c, "blog-item-info"):
						for info := c.FirstChild; info != nil; info = info.NextSibling {
							if info.Type == html.ElementNode {
								switch {
								case hasClass(info, "blog-item-info-website"):
									for w := info.FirstChild; w != nil; w = w.NextSibling {
										if w.Type == html.ElementNode && w.Data == "a" {
											entry.websites = append(entry.websites, strings.TrimSpace(innerText(w)))
										}
									}
								case hasClass(info, "blog-item-info-location"):
									entry.location = strings.TrimSpace(innerText(info))
								case hasClass(info, "blog-item-info-stock"):
									entry.stock = strings.TrimSpace(innerText(info))
								case hasClass(info, "blog-item-info-revenue"):
									entry.revenue = strings.TrimSpace(innerText(info))
								case hasClass(info, "blog-item-info-employee"):
									entry.employee = strings.TrimSpace(innerText(info))
								case hasClass(info, "blog-item-info-status"):
									entry.status = strings.TrimSpace(innerText(info))
								}
							}
						}
					case hasClass(c, "blog-item-description"):
						entry.desc = strings.TrimSpace(innerText(c))
					case hasClass(c, "blog-item-button"):
						for btn := c.FirstChild; btn != nil; btn = btn.NextSibling {
							if btn.Type == html.ElementNode && btn.Data == "a" {
								entry.dataLink = getAttr(btn, "href")
							}
						}
					}
				}
			}
			blogs = append(blogs, entry)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	for query := range channel {
		query = strings.TrimSpace(query)
		for _, blog := range blogs {
			if strings.Contains(strings.ToLower(blog.company), strings.ToLower(query)) {
				data.Source = "netrunner"
				data.Key = query
				// Compose a description with all fields
				desc := blog.desc
				if blog.location != "" {
					desc += " | Location: " + blog.location
				}
				if blog.stock != "" {
					desc += " | Stock: " + blog.stock
				}
				if blog.revenue != "" {
					desc += " | Revenue: " + blog.revenue
				}
				if blog.employee != "" {
					desc += " | Employees: " + blog.employee
				}
				if blog.status != "" {
					desc += " | Status: " + blog.status
				}
				if len(blog.websites) > 0 {
					desc += " | Websites: " + strings.Join(blog.websites, ", ")
				}
				if blog.logo != "" {
					desc += " | Logo: " + blog.logo
				}
				data.Desc = desc
				if blog.dataLink != "" {
					if strings.HasPrefix(blog.dataLink, "/") {
						data.Url = netrunnerOnion + strings.TrimPrefix(blog.dataLink, "/")
					} else {
						data.Url = blog.dataLink
					}
				} else {
					data.Url = netrunnerOnion
				}
				chanDataForDb <- data
				fmt.Println("[Netrunner] Results found: ", data.Key, data.Url)
			}
		}
	}
}
