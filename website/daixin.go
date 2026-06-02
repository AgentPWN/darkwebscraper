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

const daixinOnion = "http://7ukmkdtyxdkdivtjad57klqnd3kdsmq6tp45rrsxqnu76zzv3jvitlqd.onion/"

var daixinClient *http.Client

func initDaixinClient() error {
	if daixinClient != nil {
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
	daixinClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Daixin(channel chan string, chanDataForDb chan utils.DataForDb) {
	if err := initDaixinClient(); err != nil {
		fmt.Println("[daixin] init failed:", err)
	}

	req, _ := http.NewRequest("GET", daixinOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := daixinClient.Do(req)
	if err != nil {
		fmt.Println("[daixin] request failed:", err)
		return
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("[daixin] read body failed:", err)
		return
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytes)))
	if err != nil {
		fmt.Println("[daixin] HTML parse failed:", err)
		return
	}

	type cardEntry struct {
		company   string
		website   string
		desc      string
		leakLink  string
		leakLabel string
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
		if n.Type == html.ElementNode && n.Data == "div" && hasClassDaixin(n, "card-body") {
			entry := cardEntry{}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.Data == "h4" && hasClassDaixin(c, "card-title") {
					entry.company = strings.TrimSpace(innerText(c))
				}
				if c.Type == html.ElementNode && c.Data == "h6" && hasClassDaixin(c, "card-subtitle") && strings.Contains(innerText(c), "Web Site:") {
					for a := c.FirstChild; a != nil; a = a.NextSibling {
						if a.Type == html.ElementNode && a.Data == "a" {
							entry.website = strings.TrimSpace(innerText(a))
						}
					}
				}
				if c.Type == html.ElementNode && c.Data == "p" && hasClassDaixin(c, "card-text") {
					// Description is in the first p.card-text
					if entry.desc == "" {
						entry.desc = strings.TrimSpace(innerText(c))
					}
				}
				if c.Type == html.ElementNode && c.Data == "h6" && hasClassDaixin(c, "card-subtitle") && strings.Contains(innerText(c), "FULL LEAK") {
					for a := c.FirstChild; a != nil; a = a.NextSibling {
						if a.Type == html.ElementNode && a.Data == "a" {
							for _, attr := range a.Attr {
								if attr.Key == "href" {
									entry.leakLink = strings.TrimSpace(attr.Val)
								}
							}
							entry.leakLabel = strings.TrimSpace(innerText(a))
						}
					}
				}
			}
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
			if strings.Contains(strings.ToLower(card.company), strings.ToLower(query)) {
				url := card.leakLink
				if url != "" && !strings.HasPrefix(url, "http") {
					url = daixinOnion + strings.TrimPrefix(url, "/")
				}
				desc := card.desc
				if card.website != "" {
					desc += " | Website: " + card.website
				}
				if card.leakLabel != "" {
					desc += " | Leak: " + card.leakLabel
				}
				chanDataForDb <- utils.DataForDb{
					Source: "daixin",
					Key:    query,
					Url:    url,
					Desc:   desc,
				}
				fmt.Println("[daixin] Results found:", card.company, url)
			}
		}
	}
}

func hasClassDaixin(n *html.Node, class string) bool {
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
