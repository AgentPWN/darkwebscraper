package website

import (
	"compress/gzip"
	"crypto/tls"
	"darkwebscraper/utils"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/proxy"
)

const basheOnion = "http://basheqtvzqwz4vp6ks5lm2ocq7i6tozqgf6vjcasj4ezmsy4bkpshhyd.onion/"

var basheClient *http.Client
var bodyBytesBashe []byte

func initBasheClient() error {
	if basheClient != nil {
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

	basheClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Bashe(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}

	if err := initBasheClient(); err != nil {
		fmt.Println("[Bashe] init failed:", err)
	}

	req, _ := http.NewRequest("GET", basheOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := basheClient.Do(req)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesBashe, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesBashe, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytesBashe)))
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

	// Extracts the path from onclick="window.location.href='/page_company.php?id=161'"
	onclickRe := regexp.MustCompile(`window\.location\.href='([^']+)'`)

	var f func(*html.Node)
	f = func(n *html.Node) {
		// Each entry is a <div class="segment ..."> with an onclick
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "segment") {
			entry := cardEntry{}

			// Extract link from onclick attribute
			for _, attr := range n.Attr {
				if attr.Key == "onclick" {
					m := onclickRe.FindStringSubmatch(attr.Val)
					if len(m) > 1 {
						entry.link = strings.TrimSpace(m[1])
					}
				}
			}

			// Walk children to find company name and description
			var walk func(*html.Node)
			walk = func(c *html.Node) {
				if c.Type == html.ElementNode {
					// Company name is in <div class="segment__text__off">
					if c.Data == "div" && hasClass(c, "segment__text__off") {
						entry.company = strings.TrimSpace(innerText(c))
					}
					// Description is in <div class="segment__text__dsc">
					if c.Data == "div" && hasClass(c, "segment__text__dsc") {
						entry.desc = strings.TrimSpace(innerText(c))
					}
				}
				for child := c.FirstChild; child != nil; child = child.NextSibling {
					walk(child)
				}
			}
			walk(n)

			if entry.company != "" {
				cards = append(cards, entry)
			}
			return // don't recurse into the segment itself again
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
				url := basheOnion + strings.TrimPrefix(card.link, "/")
				data.Source = "bashe"
				data.Key = query
				data.Url = url
				data.Desc = card.desc
				chanDataForDb <- data
				fmt.Println("[Bashe] Results found: ", data.Key, data.Url)
			}
		}
	}
}
