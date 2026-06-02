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

const nightspireOnion = "http://nspirep7orjq73k2x2fwh2mxgh74vm2now6cdbnnxjk2f5wn34bmdxad.onion/database"

var nightspireClient *http.Client
var bodyBytesNightspire []byte

func initNightspireClient() error {
	if nightspireClient != nil {
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

	nightspireClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

// Nightspire scrapes the nightspire onion site for team cards and emits results in standard format.
func Nightspire(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}

	if err := initNightspireClient(); err != nil {
		fmt.Println("[Nightspire] init failed:", err)
	}

	req, _ := http.NewRequest("GET", nightspireOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := nightspireClient.Do(req)
	if err != nil {
		fmt.Println("[Nightspire] request failed:", err)
		return
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return
		}
		bodyBytesNightspire, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			return
		}
	} else {
		bodyBytesNightspire, err = io.ReadAll(resp.Body)
		if err != nil {
			return
		}
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytesNightspire)))
	if err != nil {
		fmt.Println("[Nightspire] HTML parse failed:", err)
		return
	}

	type teamEntry struct {
		company string
		logo    string
		biztype string
		bio     string
		files   string
		date    string
		count   string
	}
	var teams []teamEntry

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
	hasClassNightspire := func(n *html.Node, class string) bool {
		for _, attr := range n.Attr {
			if attr.Key == "class" && strings.Contains(attr.Val, class) {
				return true
			}
		}
		return false
	}

	// Traverse DOM to find team-card divs
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && hasClassNightspire(n, "team-card") {
			entry := teamEntry{}

			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode {
					// First child: company, logo, biztype
					if c.FirstChild != nil && c.Data == "div" {
						for cc := c.FirstChild; cc != nil; cc = cc.NextSibling {
							if cc.Type == html.ElementNode && cc.Data == "div" && getAttr(cc.FirstChild, "class") == "team-img" {
								// logo
								img := cc.FirstChild
								if img != nil && img.Data == "img" {
									entry.logo = getAttr(img, "src")
								}
							}
							if cc.Type == html.ElementNode && cc.Data == "a" && hasClassNightspire(cc, "team-name") {
								entry.company = strings.TrimSpace(innerText(cc))
							}
							if cc.Type == html.ElementNode && cc.Data == "div" && hasClassNightspire(cc, "tooltip") {
								// business type from tooltiptext span
								for t := cc.FirstChild; t != nil; t = t.NextSibling {
									if t.Type == html.ElementNode && t.Data == "span" && hasClassNightspire(t, "tooltiptext") {
										entry.biztype = strings.TrimSpace(innerText(t))
									}
								}
							}
						}
					}
					// Second child: bio and files
					if c.Data == "div" && c.FirstChild != nil && hasClassNightspire(c.FirstChild, "team-bio") {
						for cc := c.FirstChild; cc != nil; cc = cc.NextSibling {
							if cc.Type == html.ElementNode && hasClassNightspire(cc, "team-bio") {
								if entry.bio == "" {
									entry.files = strings.TrimSpace(innerText(cc))
								} else {
									entry.bio = strings.TrimSpace(innerText(cc))
								}
							}
						}
					}
				}
			}
			// Find date and count
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && hasClassNightspire(c, "team-bio") && strings.Contains(innerText(c), "🧭") {
					entry.date = strings.TrimSpace(strings.ReplaceAll(innerText(c), "🧭", ""))
				}
				if c.Type == html.ElementNode && hasClassNightspire(c, "team-bio") && c.NextSibling != nil && c.NextSibling.Type == html.ElementNode {
					// try to get count from next sibling
					for cc := c.NextSibling.FirstChild; cc != nil; cc = cc.NextSibling {
						if cc.Type == html.ElementNode && cc.Data == "p" && hasClassNightspire(cc, "team-bio") {
							entry.count = strings.TrimSpace(innerText(cc))
						}
					}
				}
			}
			teams = append(teams, entry)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	for query := range channel {
		query = strings.TrimSpace(query)
		for _, team := range teams {
			if strings.Contains(strings.ToLower(team.company), strings.ToLower(query)) {
				data.Source = "nightspire"
				data.Key = query
				desc := team.files
				if team.biztype != "" {
					desc += " | Type: " + team.biztype
				}
				if team.bio != "" {
					desc += " | Bio: " + team.bio
				}
				if team.date != "" {
					desc += " | Date: " + team.date
				}
				if team.count != "" {
					desc += " | Count: " + team.count
				}
				if team.logo != "" {
					desc += " | Logo: " + team.logo
				}
				data.Desc = desc
				data.Url = nightspireOnion
				chanDataForDb <- data
				fmt.Println("[Nightspire] Results found: ", data.Key, data.Url)
			}
		}
	}
}
