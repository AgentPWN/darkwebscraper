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

type Crypto24Response struct {
	Items []utils.CompanyCrypto24 `json:"items"`
}

const crypto24Onion = "http://j5o5y2feotmhvr7cbcp2j2ewayv5mn5zenl3joqwx67gtfchhezjznad.onion/"

var crypto24Client *http.Client
var bodyBytesCrypto24 []byte

func initCrypto24Client() error {
	if crypto24Client != nil {
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

	crypto24Client = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Crypto24(channel chan string, chanDataForDb chan utils.DataForDb) {
	for query := range channel {
		found := false
		query = strings.TrimSpace(query)
		data := utils.DataForDb{}
		var response Crypto24Response

		if err := initCrypto24Client(); err != nil {
			fmt.Println("[Crypto24] init failed:", err)
		}
		for i := range 7 {
			req, _ := http.NewRequest("GET", crypto24Onion+"api/data?page="+strconv.Itoa(i+1), nil)
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
			req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
			req.Header.Set("Accept-Language", "en-US,en;q=0.5")
			req.Header.Set("Accept-Encoding", "gzip")
			req.Header.Set("Referrer", "http://j5o5y2feotmhvr7cbcp2j2ewayv5mn5zenl3joqwx67gtfchhezjznad.onion/")

			resp, err := crypto24Client.Do(req)
			if err != nil {
				fmt.Println("[Crypto24] request failed:", err)
			}
			defer resp.Body.Close()

			if resp.Header.Get("Content-Encoding") == "gzip" {
				reader, err := gzip.NewReader(resp.Body)
				if err != nil {
					fmt.Println("[Crypto24] gzip reader error:", err)
				}
				bodyBytesCrypto24, err = io.ReadAll(reader)
				reader.Close()
				if err != nil {
					fmt.Println("[Crypto24] gzip read error:", err)
				}
			} else {
				bodyBytesCrypto24, err = io.ReadAll(resp.Body)
				if err != nil {
					fmt.Println("[Crypto24] body read error:", err)
				}
			}

			err = json.Unmarshal(bodyBytesCrypto24, &response)
			if err != nil {
				fmt.Println("[Crypto24] JSON parse error:", err)
			}

			// fmt.Println(bodyBytesCrypto24)
			for _, c := range response.Items {
				if strings.Contains(strings.ToLower(c.Name), strings.ToLower(query)) || strings.Contains(strings.ToLower(c.Desc), strings.ToLower(query)) {
					data.Source = "crypto24"
					data.Key = query
					data.Url = crypto24Onion
					data.Desc = c.Desc
					chanDataForDb <- data
					fmt.Println("[Crypto24] Results found:", data.Key, data.Url)
					found = true
					break
				}
			}
			if found {
				found = false
				break
			}
		}
	}
}
