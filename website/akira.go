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

type AkiraResponse struct {
	Posts []utils.CompanyAkira `json:"objects"`
}

const akiraOnion = "https://akiral2iz6a7qgd3ayp3l6yub7xx2uep76idk3u2kollpj5z3z636bad.onion/"

var akiraClient *http.Client
var bodyBytesAkira []byte

func initAkiraClient() error {
	if akiraClient != nil {
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

	akiraClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Akira(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}
	var response AkiraResponse

	if err := initAkiraClient(); err != nil {
		fmt.Println("[Akira] init failed:", err)
	}

	req, _ := http.NewRequest("GET", akiraOnion+"l?page=1&sort=date:desc", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := akiraClient.Do(req)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesAkira, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesAkira, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	err = json.Unmarshal(bodyBytesAkira, &response)
	if err != nil {
		fmt.Println("[Akira] JSON parse error:", err)
	}
	for query := range channel {
		query = strings.TrimSpace(query)
		for _, c := range response.Posts {
			if strings.Contains(c.Name, query) || strings.Contains(c.Desc, query) {
				url := akiraOnion
				data.Source = "akira"
				data.Key = query
				data.Url = url
				data.Desc = c.Desc
				chanDataForDb <- data
				fmt.Println(data.Key, data.Url)
				fmt.Println("[Akira] Results found: ", data.Key, data.Url)
			}
		}
	}
}
