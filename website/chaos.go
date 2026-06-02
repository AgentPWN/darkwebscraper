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

type ChaosItem struct {
	title      string `json:"title"`
	text       string `json:"text"`
	url        string `json:"url"`
	leakedSize int    `json:"leakedSize"`
	views      int    `json:"views"`
	links      struct {
		posted int `json:"posted"`
		total  int `json:"total"`
	} `json:"links"`
	link string `json:"link"`
}

type ChaosResponse struct {
	totalItems int         `json:"totalItems"`
	items      []ChaosItem `json:"items"`
}

const chaosOnion = "http://hptqq2o2qjva7lcaaq67w36jihzivkaitkexorauw7b2yul2z6zozpqd.onion/api/post/list"

var chaosClient *http.Client

func initChaosClient() error {
	if chaosClient != nil {
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
	chaosClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Chaos(channel chan string, chanDataForDb chan utils.DataForDb) {
	if err := initChaosClient(); err != nil {
		fmt.Println("[Chaos] init failed:", err)
	}

	req, _ := http.NewRequest("GET", chaosOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := chaosClient.Do(req)
	if err != nil {
		fmt.Println("[Chaos] request failed:", err)
		return
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			fmt.Println("[Chaos] gzip reader error:", err)
			return
		}
		bodyBytes, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			fmt.Println("[Chaos] gzip read error:", err)
			return
		}
	} else {
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("[Chaos] body read error:", err)
			return
		}
	}

	var response struct {
		TotalItems int `json:"totalItems"`
		Items      []struct {
			Title      string `json:"title"`
			Text       string `json:"text"`
			Url        string `json:"url"`
			LeakedSize int    `json:"leakedSize"`
			Views      int    `json:"views"`
			Links      struct {
				Posted int `json:"posted"`
				Total  int `json:"total"`
			} `json:"links"`
			Link string `json:"link"`
		} `json:"items"`
	}

	err = json.Unmarshal(bodyBytes, &response)
	if err != nil {
		fmt.Println("[Chaos] JSON parse error:", err)
		return
	}

	for query := range channel {
		query = strings.TrimSpace(query)
		for _, item := range response.Items {
			if strings.Contains(strings.ToLower(item.Title), strings.ToLower(query)) {
				url := item.Url
				if url == "" {
					url = chaosOnion
				}
				desc := item.Text
				if item.Links.Total > 0 || item.Links.Posted > 0 {
					desc += "\nLinks: posted=" + fmt.Sprint(item.Links.Posted) + ", total=" + fmt.Sprint(item.Links.Total)
				}
				if item.LeakedSize > 0 {
					desc += "\nLeaked size: " + fmt.Sprint(item.LeakedSize)
				}
				if item.Views > 0 {
					desc += "\nViews: " + fmt.Sprint(item.Views)
				}
				chanDataForDb <- utils.DataForDb{
					Source: "chaos",
					Key:    query,
					Url:    url,
					Desc:   desc,
				}
				fmt.Println("[Chaos] Results found:", item.Title, url)
			}
		}
	}
}
