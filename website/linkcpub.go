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

type LinkcpubResponse struct {
	Posts []utils.CompanyLinkcpub `json:"items"`
}

const linkcpubOnion = "http://iywqjjaf2zioehzzauys3sktbcdmuzm2fsjkqsblnm7dt6axjfpoxwid.onion/"

var linkcpubClient *http.Client
var bodyBytesLinkcpub []byte

func initLinkcpubClient() error {
	if linkcpubClient != nil {
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

	linkcpubClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Linkcpub(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}
	var response LinkcpubResponse

	if err := initLinkcpubClient(); err != nil {
		fmt.Println("[Linkcpub] init failed:", err)
	}

	req, _ := http.NewRequest("GET", linkcpubOnion+"api/article?skip=0&size=20", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := linkcpubClient.Do(req)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesLinkcpub, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesLinkcpub, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	err = json.Unmarshal(bodyBytesLinkcpub, &response)
	if err != nil {
		fmt.Println("[Linkcpub] JSON parse error:", err)
	}
	for query := range channel {
		query = strings.TrimSpace(query)

		for _, c := range response.Posts {
			if strings.Contains(c.Name, query) {
				url := linkcpubOnion + "article/" + c.ID
				data.Source = "linkcpub"
				data.Key = query
				data.Url = url
				data.Desc = c.Desc
				chanDataForDb <- data
				fmt.Println("[Linkcpub] Results found:", data.Key, data.Url)
			}

		}
	}
}
