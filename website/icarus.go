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

const icarusOnion = "http://e6ujsppajgb756x7x5ykdryvlcjynltb52eiwi6pd4bfwo6hddd6neid.onion/"

var icarusClient *http.Client

func initIcarusClient() error {
	if icarusClient != nil {
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

	icarusClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Icarus(channel chan string, chanDataForDb chan utils.DataForDb) {
	if err := initIcarusClient(); err != nil {
		fmt.Println("[Icarus] init failed:", err)
		return
	}

	req, err := http.NewRequest("GET", icarusOnion, nil)
	if err != nil {
		fmt.Println("[Icarus] request build failed:", err)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := icarusClient.Do(req)
	if err != nil {
		fmt.Println("[Icarus] request failed:", err)
		return
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			fmt.Println("[Icarus] gzip reader failed:", err)
			return
		}
		bodyBytes, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			fmt.Println("[Icarus] read gzip body failed:", err)
			return
		}
	} else {
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("[Icarus] read body failed:", err)
			return
		}
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytes)))
	if err != nil {
		fmt.Println("[Icarus] HTML parse failed:", err)
		return
	}

	type victimEntry struct {
		name     string
		desc     string
		views    string
		size     string
		link     string
		victimID string
	}

	var victims []victimEntry

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
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "victim-item") {
			entry := victimEntry{}
			for _, attr := range n.Attr {
				switch attr.Key {
				case "data-name":
					entry.name = strings.TrimSpace(attr.Val)
				case "data-desc":
					entry.desc = strings.TrimSpace(attr.Val)
				case "data-victim-id":
					entry.victimID = strings.TrimSpace(attr.Val)
				case "data-url", "data-link":
					entry.link = strings.TrimSpace(attr.Val)
				case "onclick":
					if strings.Contains(attr.Val, "http") {
						entry.link = strings.TrimSpace(attr.Val)
					}
				}
			}

			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type != html.ElementNode {
					continue
				}
				if c.Data == "div" && hasClass(c, "victim-info") {
					for cc := c.FirstChild; cc != nil; cc = cc.NextSibling {
						if cc.Type == html.ElementNode && cc.Data == "h4" && entry.name == "" {
							entry.name = strings.TrimSpace(innerText(cc))
						}
						if cc.Type == html.ElementNode && cc.Data == "p" && hasClass(cc, "victim-desc") && entry.desc == "" {
							entry.desc = strings.TrimSpace(innerText(cc))
						}
					}
				}
				if c.Data == "div" && hasClass(c, "victim-meta") {
					for cc := c.FirstChild; cc != nil; cc = cc.NextSibling {
						if cc.Type == html.ElementNode && cc.Data == "span" {
							text := strings.TrimSpace(innerText(cc))
							if hasClass(cc, "victim-views") {
								entry.views = text
							}
							if hasClass(cc, "victim-size") {
								entry.size = text
							}
						}
					}
				}
			}

			if entry.name != "" {
				if entry.link == "" {
					if entry.victimID != "" {
						entry.link = icarusOnion + "victim/" + entry.victimID
					} else {
						entry.link = icarusOnion
					}
				}
				victims = append(victims, entry)
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
		lowerQuery := strings.ToLower(query)
		for _, victim := range victims {
			combined := strings.ToLower(victim.name + " " + victim.desc)
			if strings.Contains(combined, lowerQuery) {
				parts := []string{}
				if victim.desc != "" {
					parts = append(parts, victim.desc)
				}
				if victim.views != "" {
					parts = append(parts, victim.views)
				}
				if victim.size != "" {
					parts = append(parts, victim.size)
				}
				chanDataForDb <- utils.DataForDb{
					Source: "icarus",
					Key:    query,
					Url:    victim.link,
					Desc:   strings.Join(parts, " | "),
				}
				fmt.Println("[Icarus] Results found:", victim.name, victim.link)
			}
		}
	}
}
