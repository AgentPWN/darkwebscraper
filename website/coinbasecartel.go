package website

import (
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

const coinbasecartelOnion = "http://fjg4zi4opkxkvdz7mvwp7h6goe4tcby3hhkrz43pht4j3vakhy75znyd.onion/"

var coinbasecartelClient *http.Client

func initCoinbasecartelClient() error {
	if coinbasecartelClient != nil {
		return nil
	}
	torDialer, err := proxy.SOCKS5("tcp", "localhost:9050", nil, nil)
	if err != nil {
		return fmt.Errorf("proxy.SOCKS5: %w", err)
	}
	transport := &http.Transport{
		DialContext:     torDialer.(proxy.ContextDialer).DialContext,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	coinbasecartelClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Coinbasecartel(channel chan string, chanDataForDb chan utils.DataForDb) {
	if err := initCoinbasecartelClient(); err != nil {
		fmt.Println("[coinbasecartel] init failed:", err)
	}

	req, _ := http.NewRequest("GET", coinbasecartelOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := coinbasecartelClient.Do(req)
	if err != nil {
		fmt.Println("[coinbasecartel] request failed:", err)
		return
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("[coinbasecartel] read body failed:", err)
		return
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytes)))
	if err != nil {
		fmt.Println("[coinbasecartel] HTML parse failed:", err)
		return
	}

	type targetEntry struct {
		name     string
		industry string
		revenue  string
		viewUrl  string
	}
	var targets []targetEntry

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
		if n.Type == html.ElementNode && n.Data == "div" && hasClassCoinbasecartel(n, "target-row") {
			entry := targetEntry{}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && hasClassCoinbasecartel(c, "target-name") {
					entry.name = strings.TrimSpace(innerText(c))
				}
				if c.Type == html.ElementNode && hasClassCoinbasecartel(c, "target-industry") {
					entry.industry = strings.TrimSpace(innerText(c))
				}
				if c.Type == html.ElementNode && hasClassCoinbasecartel(c, "target-rev") {
					entry.revenue = strings.TrimSpace(innerText(c))
				}
				if c.Type == html.ElementNode && hasClassCoinbasecartel(c, "target-actions") {
					for a := c.FirstChild; a != nil; a = a.NextSibling {
						if a.Type == html.ElementNode && a.Data == "a" {
							for _, attr := range a.Attr {
								if attr.Key == "href" {
									entry.viewUrl = strings.TrimSpace(attr.Val)
								}
							}
						}
					}
				}
			}
			if entry.name != "" {
				targets = append(targets, entry)
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
		for _, t := range targets {
			if strings.Contains(strings.ToLower(t.name), strings.ToLower(query)) {
				url := t.viewUrl
				if url != "" && !strings.HasPrefix(url, "http") {
					url = coinbasecartelOnion + strings.TrimPrefix(url, "/")
				}
				desc := t.industry
				if t.revenue != "" {
					desc += " | Revenue: " + t.revenue
				}
				chanDataForDb <- utils.DataForDb{
					Source: "coinbasecartel",
					Key:    query,
					Url:    url,
					Desc:   desc,
				}
				fmt.Println("[coinbasecartel] Results found:", t.name, url)
			}
		}
	}
}

func hasClassCoinbasecartel(n *html.Node, class string) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" {
			for _, c := range strings.Fields(attr.Val) {
				if c == class {
					return true
				}
			}
		}
	}
	return false
}
