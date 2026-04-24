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

// type TermiteResponse struct {
// 	Posts []utils.CompanyTermite `json:"posts"`
// }

const termiteOnion = "http://termitelfvhutinrgpe55siktisskbqntkuq7ojidg42zh26avekq6qd.onion/"

var termiteClient *http.Client
var bodyBytesTermite []byte

func initTermiteClient() error {
	if termiteClient != nil {
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

	termiteClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Termite(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}
	var companies []utils.CompanyTermite

	if err := initTermiteClient(); err != nil {
		fmt.Println("[Termite] init failed:", err)
	}

	req, _ := http.NewRequest("GET", termiteOnion+"api/blog/blogs", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := termiteClient.Do(req)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesTermite, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesTermite, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	// body := string(bodyBytesTermite)

	err = json.Unmarshal(bodyBytesTermite, &companies)
	if err != nil {
		fmt.Println("[Termite] JSON parse error:", err)
	}
	for query := range channel {
		// fmt.Println(query)
		query = strings.TrimSpace(query)

		for _, c := range companies {
			if strings.Contains(c.Name, query) || strings.Contains(c.Desc, query) {
				url := termiteOnion + "post/" + c.ID
				data.Source = "termite"
				data.Key = query
				data.Url = url
				data.Desc = c.Desc
				chanDataForDb <- data
				fmt.Println(data.Key, data.Url)
				fmt.Println("[Termite] Results found:", data.Key, data.Url)
			}
		}
	}
}
