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

const cmdOnion = "http://cmdnkiqjije2tllr3biee2sjgj3i4robg2cbtilbnytdhh2wy3syrlyd.onion/"

var cmdClient *http.Client

func initCmdClient() error {
	if cmdClient != nil {
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
	cmdClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Cmd(channel chan string, chanDataForDb chan utils.DataForDb) {
	if err := initCmdClient(); err != nil {
		fmt.Println("[cmd] init failed:", err)
	}

	req, _ := http.NewRequest("GET", cmdOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := cmdClient.Do(req)
	if err != nil {
		fmt.Println("[cmd] request failed:", err)
		return
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("[cmd] read body failed:", err)
		return
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytes)))
	if err != nil {
		fmt.Println("[cmd] HTML parse failed:", err)
		return
	}

	type cardEntry struct {
		company     string
		descBefore  string
		descAfter   string
		docLinks    []string
		companyLink string
	}
	var cards []cardEntry

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

	// Find all .item-card divs
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && hasClassCmd(n, "item-card") {
			entry := cardEntry{}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.Data == "div" && hasClassCmd(c, "item-header") {
					for cc := c.FirstChild; cc != nil; cc = cc.NextSibling {
						if cc.Type == html.ElementNode && cc.Data == "h2" {
							for a := cc.FirstChild; a != nil; a = a.NextSibling {
								if a.Type == html.ElementNode && a.Data == "a" {
									entry.company = innerText(a)
									for _, attr := range a.Attr {
										if attr.Key == "href" {
											entry.companyLink = strings.TrimSpace(attr.Val)
										}
									}
								}
							}
						}
					}
				}
				if c.Type == html.ElementNode && c.Data == "div" && hasClassCmd(c, "description-before") {
					entry.descBefore = strings.TrimSpace(innerText(c))
				}
				if c.Type == html.ElementNode && c.Data == "div" && hasClassCmd(c, "description-after") {
					entry.descAfter = strings.TrimSpace(innerText(c))
				}
				if c.Type == html.ElementNode && c.Data == "div" && hasClassCmd(c, "item-links") {
					for l := c.FirstChild; l != nil; l = l.NextSibling {
						if l.Type == html.ElementNode && l.Data == "a" {
							for _, attr := range l.Attr {
								if attr.Key == "href" {
									entry.docLinks = append(entry.docLinks, strings.TrimSpace(attr.Val))
								}
							}
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
				var desc strings.Builder
				desc.WriteString(card.descBefore)
				if card.descAfter != "" {
					desc.WriteString("\n")
					desc.WriteString(card.descAfter)
				}
				for _, l := range card.docLinks {
					desc.WriteString("\nDoc: ")
					if strings.HasPrefix(l, "http") {
						desc.WriteString(l)
					} else {
						desc.WriteString(cmdOnion + strings.TrimPrefix(l, "/"))
					}
				}
				url := card.companyLink
				if url == "" {
					url = cmdOnion
				}
				chanDataForDb <- utils.DataForDb{
					Source: "cmd",
					Key:    query,
					Url:    url,
					Desc:   desc.String(),
				}
				fmt.Println("[cmd] Results found:", card.company, url)
			}
		}
	}
}

func hasClassCmd(n *html.Node, class string) bool {
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
