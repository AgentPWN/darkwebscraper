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

	"golang.org/x/net/proxy"
)

const darknetOnion = "http://darknet4zvovn77zgkppdrgzuf7i3kvn5aepmjp6g6djyyzxwyjyfcyd.onion"

var darknetClient *http.Client
var bodyBytesDarknet []byte

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

func Darknet(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}
	if err := initDarknetClient(); err != nil {
		fmt.Println("[Darknet] init failed:", err)
	}
	// fmt.Println("[Darknet]")

	body := string(bodyBytesDarknet)
	for query := range channel {
		query = strings.TrimSpace(query)
		// fmt.Println(body)
		darknetOnionQueryURL := darknetOnion + "/search/17557447/?q=" + query + "&o=date"
		req, _ := http.NewRequest("GET", darknetOnionQueryURL, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")
		req.Header.Set("Accept-Encoding", "gzip")

		resp, err := darknetClient.Do(req)
		// fmt.Println(resp)
		if resp.Header.Get("Content-Encoding") == "gzip" {
			reader, err := gzip.NewReader(resp.Body)
			if err != nil {
				resp.Body.Close()
			}
			bodyBytesDarknet, err = io.ReadAll(reader)
			reader.Close()
			if err != nil {
				resp.Body.Close()
			}
		} else {
			bodyBytesDarknet, err = io.ReadAll(resp.Body)
			if err != nil {
				resp.Body.Close()
			}
		}
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
	}

}
