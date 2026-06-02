package website

import (
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

type MerxCard struct {
	Logo   string `json:"logo"`
	Title  string `json:"title"`
	Text   string `json:"text"`
	Stolen string `json:"stolen"`
	List   string `json:"list"`
}

type MerxResponse []MerxCard

const merxOnion = "http://4k6plf4h2cm2nco6ae3inrsxnmqgl6lllmwefydhnlcq4tuhwbj4qpad.onion/cards.json"

var merxClient *http.Client

func initMerxClient() error {
	if merxClient != nil {
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

	merxClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Merx(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}
	var response MerxResponse

	if err := initMerxClient(); err != nil {
		fmt.Println("[Merx] init failed:", err)
	}

	req, _ := http.NewRequest("GET", merxOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := merxClient.Do(req)
	if err != nil {
		fmt.Println("[Merx] request failed:", err)
		return
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("[Merx] body read error:", err)
		return
	}

	err = json.Unmarshal(bodyBytes, &response)
	if err != nil {
		fmt.Println("[Merx] JSON parse error:", err)
		return
	}

	for query := range channel {
		query = strings.TrimSpace(query)
		for _, card := range response {
			if strings.Contains(strings.ToLower(card.Title), strings.ToLower(query)) {
				data.Source = "M3RX"
				data.Key = query
				data.Url = card.List
				desc := card.Text + " | Stolen: " + card.Stolen + ", Logo: " + card.Logo
				data.Desc = desc
				chanDataForDb <- data
				fmt.Println("[M3RX] Results found:", data.Key, data.Url)
			}
		}
	}
}
