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

type MorpheusResponse struct {
	Posts []utils.CompanyMorpheus `json:"items"`
}

const morpheusOnion = "http://izsp6ipui4ctgxfugbgtu65kzefrucltyfpbxplmfybl5swiadpljmyd.onion/"

var morpheusClient *http.Client
var bodyBytesMorpheus []byte

func initMorpheusClient() error {
	if morpheusClient != nil {
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

	morpheusClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Morpheus(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}
	var response MorpheusResponse

	if err := initMorpheusClient(); err != nil {
		fmt.Println("[Morpheus] init failed:", err)
	}

	req, _ := http.NewRequest("GET", morpheusOnion+"intrumpwetrust/api/posts?page=1&perPage=50", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := morpheusClient.Do(req)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesMorpheus, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesMorpheus, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	// body := string(bodyBytesMorpheus)

	err = json.Unmarshal(bodyBytesMorpheus, &response)
	if err != nil {
		fmt.Println("[Morpheus] JSON parse error:", err)
	}
	for query := range channel {
		query = strings.TrimSpace(query)

		for _, c := range response.Posts {
			if strings.Contains(c.Name, query) {
				url := morpheusOnion
				data.Source = "morpheus"
				data.Key = query
				data.Url = url
				data.Desc = "Lorem Ipsum"
				chanDataForDb <- data
				fmt.Println(data.Key, data.Url)
				fmt.Println("[Morpheus] Results found")
			}
		}

	}
}
