package website

import (
	"compress/gzip"
	"crypto/tls"
	"darkwebscraper/utils"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

type Cry0Item struct {
	ID        string `json:"id"`
	CreatedAt int64  `json:"created_at"`
	Author    []int  `json:"author"`
	Body      struct {
		Content  string   `json:"content"`
		Preview  string   `json:"preview"`
		Attached []string `json:"attached"`
	} `json:"body"`
	Title   string `json:"title"`
	Visible bool   `json:"visible"`
}

const cry0Onion = "http://cryoblogedawivdcknyd4jsjxkrx3xrqqltxla6wwjjnzm3f3jaxjzqd.onion/blog-info"

var cry0Client *http.Client

func initCry0Client() error {
	if cry0Client != nil {
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
	cry0Client = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Cry0(channel chan string, chanDataForDb chan utils.DataForDb) {
	if err := initCry0Client(); err != nil {
		fmt.Println("[cry0] init failed:", err)
	}

	req, _ := http.NewRequest("GET", cry0Onion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := cry0Client.Do(req)
	if err != nil {
		fmt.Println("[cry0] request failed:", err)
		return
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			fmt.Println("[cry0] gzip reader error:", err)
			return
		}
		bodyBytes, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			fmt.Println("[cry0] gzip read error:", err)
			return
		}
	} else {
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("[cry0] body read error:", err)
			return
		}
	}

	var items []Cry0Item
	err = json.Unmarshal(bodyBytes, &items)
	if err != nil {
		fmt.Println("[cry0] JSON parse error:", err)
		return
	}

	for query := range channel {
		query = strings.TrimSpace(query)
		for _, item := range items {
			if strings.Contains(strings.ToLower(item.Title), strings.ToLower(query)) || strings.Contains(strings.ToLower(item.Body.Content), strings.ToLower(query)) {
				desc := item.Body.Content
				if item.Body.Preview != "" {
					desc += "\nPreview: " + item.Body.Preview
				}
				if len(item.Body.Attached) > 0 {
					desc += "\nAttached: " + strings.Join(item.Body.Attached, ", ")
				}
				chanDataForDb <- utils.DataForDb{
					Source: "cry0",
					Key:    query,
					Url:    item.Title,
					Desc:   desc,
				}
				fmt.Println("[cry0] Results found:", item.Title)
			}
		}
	}
}
