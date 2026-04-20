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

const kyberOnion = "http://kyblogtz6k3jtxnjjvluee5ec4g3zcnvyvbgsnq5thumphmqidkt7xid.onion/"

var kyberClient *http.Client
var bodyBytesKyber []byte

func initKyberClient() error {
	if kyberClient != nil {
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

	kyberClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Kyber(query string, chanDataForDb chan utils.DataForDb) bool {
	data := utils.DataForDb{}
	if err := initKyberClient(); err != nil {
		panic(err)
	}

	// targetURL := kyberOnion + "/search/?q=" + url.QueryEscape(query)

	for {
		req, _ := http.NewRequest("GET", kyberOnion, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")
		req.Header.Set("Accept-Encoding", "gzip")

		resp, err := kyberClient.Do(req)
		if err != nil {
			fmt.Println("[Kyber] request failed:", err)
			time.Sleep(10 * time.Second)
			continue
		}

		if resp.Header.Get("Content-Encoding") == "gzip" {
			reader, err := gzip.NewReader(resp.Body)
			if err != nil {
				resp.Body.Close()
				continue
			}
			bodyBytesKyber, err = io.ReadAll(reader)
			reader.Close()
			if err != nil {
				resp.Body.Close()
				continue
			}
		} else {
			bodyBytesKyber, err = io.ReadAll(resp.Body)
			if err != nil {
				resp.Body.Close()
				continue
			}
		}
		resp.Body.Close()

		body := string(bodyBytesKyber)
		fmt.Println(body)
		if strings.Contains(body, query) {
			links := utils.ExtractPostLinks(body, "/post/")
			for _, link := range links {
				fmt.Println("Post link:", kyberOnion+link)
				data.Source = "kyber"
				data.Key = query
				data.Url = kyberOnion + link
				data.Desc = "lorem ipsum"
				chanDataForDb <- data
			}
			return true
		} else {
			fmt.Println("[Kyber] Results not found")
		}
		// 	switch {
		// 	case strings.Contains(body, "No results"):
		// 		fmt.Println("[Kyber] no results")
		// 		return false

		// 	case strings.Contains(body, "Exactly"):
		// 		fmt.Println("[Kyber] Found result")

		// 		links := utils.ExtractPostLinks(body, "/post/")
		// 		for _, link := range links {
		// 			fmt.Println("Post link:", kyberOnion+link)
		// 			data.Source = "kyber"
		// 			data.Key = query
		// 			data.Url = kyberOnion + link
		// 			data.Desc = "lorem ipsum"
		// 			chanDataForDb <- data
		// 		}
		// 		return true
	}
}
