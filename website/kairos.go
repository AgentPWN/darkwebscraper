package website

import (
	"compress/gzip"
	"crypto/tls"
	"darkwebscraper/utils"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

const kairosOnion = "http://cvfgzmrcam5efoqx4lkdejv5nb6icocsla2tn2dzw4b7wlyf4luz7fyd.onion/"

var kairosClient *http.Client
var bodyBytesKairos []byte

func initKairosClient() error {
	if kairosClient != nil {
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

	kairosClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Kairos(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}
	var companies []utils.CompanyKairos
	if err := initKairosClient(); err != nil {
		fmt.Println("[Kairos] init failed:", err)
	}
	// go run main.go
	// fmt.Println("[Kairos]")
	req, _ := http.NewRequest("GET", kairosOnion+"/services/search?q=", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := kairosClient.Do(req)
	// fmt.Println(resp)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesKairos, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesKairos, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	// fmt.Println("body found")
	// fmt.Println(body)
	err = json.Unmarshal(bodyBytesKairos, &companies)
	if err != nil {
		fmt.Println("[Kairos] JSON parse error:", err)
		// fmt.Println(string(bodyBytesKairos))
		// debug for truncated responses
	}
	for query := range channel {
		query = strings.TrimSpace(query)
		for _, c := range companies {
			if strings.Contains(c.Name, query) {
				url := kairosOnion + "service/" + strconv.Itoa(c.ID) + "?page=1"
				data.Source = "kairos"
				data.Key = query
				data.Url = url
				data.Desc = c.Desc
				chanDataForDb <- data
				fmt.Println("[Kairos] Results found:", data.Key, data.Url)
			}
		}
	}
}
