package website

import (
	"crypto/tls"
	"darkwebscraper/utils"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/proxy"
)

const darknetOnion = "http://darknet4zvovn77zgkppdrgzuf7i3kvn5aepmjp6g6djyyzxwyjyfcyd.onion"

var darknetClient *http.Client
var bodyBytesDarknet []byte

func extractPostLinks(body string) []string {
	var links []string

	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return links
	}

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" && strings.HasPrefix(attr.Val, "/post/") {
					links = append(links, attr.Val)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}

	f(doc)
	return links
}
func initDarknetClient() error {
	if darknetClient != nil {
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

	darknetClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Darknet(query string, chanDataForDb chan utils.DataForDb) bool {
	data := utils.DataForDb{}
	if err := initDarknetClient(); err != nil {
		fmt.Println("[Darknet] init failed:", err)
		return false
	}
	// fmt.Println("[Darknet]")
	darknetOnionQueryURL := darknetOnion + "/search/17557447/?q=" + query + "&o=date"
	req, _ := http.NewRequest("GET", darknetOnionQueryURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := darknetClient.Do(req)
	if err != nil {
		fmt.Println("[Darknet] request failed:", err)
		return false
	}
	bodyBytesDarknet, err = readResponseBody(resp)
	if err != nil {
		fmt.Println("[Darknet] read body failed:", err)
		return false
	}
	body := string(bodyBytesDarknet)

	if strings.Contains(body, "No results found.") {
		fmt.Println("[Darknet] No results found")
		return false
	} else {
		// fmt.Println(body)
		links := utils.ExtractPostLinks(body, "/threads/")
		for _, link := range links {
			if link != "/threads/contact-us.701/" {
				fmt.Println("Post link:", darknetOnion+link)
				data.Source = "darknetarmy"
				data.Key = query
				data.Url = darknetOnion + link
				data.Desc = "lorem ipsum"
				chanDataForDb <- data
			}
		}
		fmt.Println("[Darknet] Results found")
		return true
	}
}
