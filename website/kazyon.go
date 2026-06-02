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

const kazyonOnion = "http://pdndkkg2hu4z36yhrbgtycxf52iodlh5os4argm2ooia4ypwgnvlzgqd.onion/"

var kazyonClient *http.Client
var bodyBytesKazyon []byte

func initKazyonClient() error {
	if kazyonClient != nil {
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

	kazyonClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Kazyon(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}

	if err := initKazyonClient(); err != nil {
		fmt.Println("[Kazyon] init failed:", err)
	}

	req, _ := http.NewRequest("GET", kazyonOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := kazyonClient.Do(req)
	if err != nil {
		fmt.Println("[Kazyon] request failed:", err)
		return
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesKazyon, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesKazyon, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytesKazyon)))
	if err != nil {
		panic(err)
	}

	type fileEntry struct {
		name string
		size string
		date string
		link string
	}
	var files []fileEntry

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
		if n.Type == html.ElementNode && n.Data == "tr" {
			var entry fileEntry
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.Data == "td" && hasClass(c, "link") {
					for gc := c.FirstChild; gc != nil; gc = gc.NextSibling {
						if gc.Type == html.ElementNode && gc.Data == "a" {
							entry.name = innerText(gc)
							for _, attr := range gc.Attr {
								if attr.Key == "href" {
									entry.link = kazyonOnion + strings.TrimPrefix(attr.Val, "/")
								}
							}
						}
					}
				}
				if c.Type == html.ElementNode && c.Data == "td" && hasClass(c, "size") {
					entry.size = innerText(c)
				}
				if c.Type == html.ElementNode && c.Data == "td" && hasClass(c, "date") {
					entry.date = innerText(c)
				}
			}
			if entry.name != "" {
				files = append(files, entry)
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
		for _, file := range files {
			if strings.Contains(strings.ToLower(file.name), strings.ToLower(query)) {
				data.Source = "Kazyon"
				data.Key = query
				data.Url = file.link
				desc := file.name + " | Size: " + file.size + ", Date: " + file.date
				data.Desc = desc
				chanDataForDb <- data
				fmt.Println("[Kazyon] Results found:", data.Key, data.Url)
			}
		}
	}
}
