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

const interlockOnion = "http://ebhmkoohccl45qesdbvrjqtyro2hmhkmh6vkyfyjjzfllm3ix72aqaid.onion/leaks.php"

var interlockClient *http.Client
var bodyBytesInterlock []byte

func initInterlockClient() error {
	if interlockClient != nil {
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

	interlockClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Interlock(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}

	if err := initInterlockClient(); err != nil {
		fmt.Println("[Interlock] init failed:", err)
	}

	req, _ := http.NewRequest("GET", interlockOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := interlockClient.Do(req)
	if err != nil {
		fmt.Println("[Interlock] request failed:", err)
		return
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesInterlock, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesInterlock, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytesInterlock)))
	if err != nil {
		panic(err)
	}

	type advertEntry struct {
		company string
		desc    string
		link    string
		size    string
		files   string
		folders string
	}
	var adverts []advertEntry

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
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "advert_item") {
			entry := advertEntry{}

			// Extract company name
			var infoTitle *html.Node
			var infoDesc *html.Node
			var infoCode *html.Node
			var actionLink string

			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && hasClass(c, "advert_info") {
					for cc := c.FirstChild; cc != nil; cc = cc.NextSibling {
						if hasClass(cc, "advert_info_title") {
							infoTitle = cc
						}
						if hasClass(cc, "advert_info_p") {
							infoDesc = cc
						}
						if hasClass(cc, "advert_info_code") {
							infoCode = cc
						}
					}
				}
				if c.Type == html.ElementNode && hasClass(c, "advert_action") {
					for cc := c.FirstChild; cc != nil; cc = cc.NextSibling {
						if cc.Type == html.ElementNode && cc.Data == "a" {
							for _, attr := range cc.Attr {
								if attr.Key == "href" {
									actionLink = strings.TrimSpace(attr.Val)
								}
							}
						}
					}
				}
			}

			if infoTitle != nil {
				entry.company = innerText(infoTitle)
			}
			if infoDesc != nil {
				entry.desc = innerText(infoDesc)
			}
			if infoCode != nil {
				// Extract size, files, folders from spans
				for c := infoCode.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.ElementNode && c.Data == "span" {
						text := innerText(c)
						if strings.Contains(text, "Size:") {
							entry.size = strings.TrimSpace(strings.TrimPrefix(text, "Size:"))
						}
						if strings.Contains(text, "Files:") {
							entry.files = strings.TrimSpace(strings.TrimPrefix(text, "Files:"))
						}
						if strings.Contains(text, "Folders:") {
							entry.folders = strings.TrimSpace(strings.TrimPrefix(text, "Folders:"))
						}
					}
				}
			}
			if actionLink != "" {
				entry.link = actionLink
			}

			if entry.company != "" {
				adverts = append(adverts, entry)
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
		for _, advert := range adverts {
			if strings.Contains(strings.ToLower(advert.company), strings.ToLower(query)) {
				data.Source = "interlock"
				data.Key = query
				data.Url = advert.link
				desc := advert.desc + " | Size: " + advert.size + ", Files: " + advert.files + ", Folders: " + advert.folders
				data.Desc = desc
				chanDataForDb <- data
				fmt.Println("[Interlock] Results found:", data.Key, data.Url)
			}
		}
	}
}
