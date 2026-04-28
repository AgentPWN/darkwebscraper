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

const dragonforceOnion = "http://dragonforxxbp3awc7mzs5dkswrua3znqyx5roefmi4smjrsdi22xwqd.onion/"

var dragonforceClient *http.Client
var bodyBytesDragonforce []byte

func initDragonforceClient() error {
	if dragonforceClient != nil {
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

	dragonforceClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Dragonforce(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}

	if err := initDragonforceClient(); err != nil {
		fmt.Println("[Dragonforce] init failed:", err)
	}

	req, _ := http.NewRequest("GET", dragonforceOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := dragonforceClient.Do(req)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesDragonforce, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesDragonforce, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytesDragonforce)))
	if err != nil {
		panic(err)
	}

	type cardEntry struct {
		company string
		link    string
		desc    string
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
		// Each entry is a <div class="text"> containing the company link
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "text") {
			entry := cardEntry{}

			// Company name and link are inside <a class="... link-going">
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.Data == "a" && hasClass(c, "link-going") {
					entry.company = strings.TrimSpace(innerText(c))
					for _, attr := range c.Attr {
						if attr.Key == "href" {
							entry.link = strings.TrimSpace(attr.Val)
						}
					}
				}
			}

			// Collect desc from the two sibling divs that follow:
			// <div class="timer ..."> (date) and <div class="number"> (data size)
			var timer, size string
			for sib := n.NextSibling; sib != nil; sib = sib.NextSibling {
				if sib.Type != html.ElementNode {
					continue
				}
				if sib.Data == "div" && hasClass(sib, "timer") && timer == "" {
					timer = strings.TrimSpace(innerText(sib))
				}
				if sib.Data == "div" && hasClass(sib, "number") && size == "" {
					size = strings.TrimSpace(innerText(sib))
				}
				if timer != "" && size != "" {
					break
				}
			}
			if timer != "" || size != "" {
				entry.desc = strings.TrimSpace(timer + " | " + size)
			}

			if entry.company != "" {
				cards = append(cards, entry)
			}
			// don't return here — siblings are not children, no recursion risk
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	for query := range channel {
		query = strings.TrimSpace(query)
		for _, card := range cards {
			if strings.Contains(card.company, query) {
				url := dragonforceOnion + strings.TrimPrefix(card.link, "/")
				data.Source = "dragonforce"
				data.Key = query
				data.Url = url
				data.Desc = card.desc
				chanDataForDb <- data
				fmt.Println("[Dragonforce] Results found: ", data.Key, data.Url)
			}
		}
	}
}
