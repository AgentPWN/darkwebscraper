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

type EmbargoResponse struct {
	Payload struct {
		Posts []struct {
			ID    string `json:"_id"`
			Title string `json:"title"`
			Desc  string `json:"description"`
		} `json:"posts"`
	} `json:"payload"`
}

const embargoOnion = "http://embargobe3n5okxyzqphpmk3moinoap2snz5k6765mvtkk7hhi544jid.onion/"

var embargoClient *http.Client
var bodyBytesEmbargo []byte

func initEmbargoClient() error {
	if embargoClient != nil {
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

	embargoClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Embargo(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}
	var response EmbargoResponse

	if err := initEmbargoClient(); err != nil {
		fmt.Println("[Embargo] init failed:", err)
	}

	req, _ := http.NewRequest("GET", embargoOnion+"api/blog/get", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := embargoClient.Do(req)
	if err != nil {
		fmt.Println("[Embargo] request failed:", err)
		return
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			fmt.Println("[Embargo] gzip reader error:", err)
		}
		bodyBytesEmbargo, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			fmt.Println("[Embargo] gzip read error:", err)
		}
	} else {
		bodyBytesEmbargo, err = io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("[Embargo] body read error:", err)
		}
	}

	err = json.Unmarshal(bodyBytesEmbargo, &response)
	if err != nil {
		fmt.Println("[Embargo] JSON parse error:", err)
	}

	for query := range channel {
		query = strings.TrimSpace(query)
		for _, c := range response.Payload.Posts {
			if strings.Contains(strings.ToLower(c.Title), strings.ToLower(query)) {
				data.Source = "embargo"
				data.Key = query
				data.Url = embargoOnion
				data.Desc = c.Desc
				chanDataForDb <- data
				fmt.Println(data.Key, data.Url)
				fmt.Println("[Embargo] Results found:", data.Key, data.Url)
			}
		}
	}
}
