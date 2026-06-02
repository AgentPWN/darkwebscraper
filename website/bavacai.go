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

const bavacaiOnion = "http://t33zoj4qwv455fog7qnb2azi5xcdxkixughmmduzbw2rtdgryqfbh6id.onion/"

var bavacaiClient *http.Client

func initBavacaiClient() error {
	if bavacaiClient != nil {
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

	bavacaiClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Bavacai(channel chan string, chanDataForDb chan utils.DataForDb) {
	if err := initBavacaiClient(); err != nil {
		fmt.Println("[Bavacai] init failed:", err)
	}

	req, _ := http.NewRequest("GET", bavacaiOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := bavacaiClient.Do(req)
	if err != nil {
		fmt.Println("[Bavacai] request failed:", err)
		return
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("[Bavacai] read body failed:", err)
		return
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytes)))
	if err != nil {
		fmt.Println("[Bavacai] HTML parse failed:", err)
		return
	}

	type cardEntry struct {
		company string
		address string
		amount  string
		desc    string
		link    string
	}
	var cards []cardEntry

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" && hasClassBavacai(n, "card") {
			entry := cardEntry{}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.Data == "div" && hasClassBavacai(c, "card-name-row") {
					for cc := c.FirstChild; cc != nil; cc = cc.NextSibling {
						if cc.Type == html.ElementNode && cc.Data == "div" && hasClassBavacai(cc, "card-name") {
							entry.company = getInnerText(cc)
						}
					}
				}
				if c.Type == html.ElementNode && c.Data == "div" && hasClassBavacai(c, "card-address") {
					entry.address = getInnerText(c)
				}
				if c.Type == html.ElementNode && c.Data == "div" && hasClassBavacai(c, "card-stub-label") {
					entry.amount = getInnerText(c)
				}
				if c.Type == html.ElementNode && c.Data == "div" && hasClassBavacai(c, "card-desc") {
					entry.desc = getInnerText(c)
				}
			}
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					entry.link = strings.TrimSpace(attr.Val)
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
				data := utils.DataForDb{
					Source: "bavacai",
					Key:    query,
					Url:    bavacaiOnion + strings.TrimPrefix(card.link, "/"),
					Desc:   card.company + " | " + card.address + " | " + card.amount + " | " + card.desc,
				}
				chanDataForDb <- data
				fmt.Println("[Bavacai] Match found:", card.company)
			}
		}
	}
}

func hasClassBavacai(n *html.Node, class string) bool {
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

func getInnerText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(getInnerText(c))
	}
	return sb.String()
}
