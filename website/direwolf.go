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

const direwolfOnion = "http://direwolfcdkv5whaz2spehizdg22jsuf5aeje4asmetpbt6ri4jnd4qd.onion/"

var direwolfClient *http.Client
var bodyBytesDirewolf []byte

func initDirewolfClient() error {
	if direwolfClient != nil {
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

	direwolfClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Direwolf(channel chan string, chanDataForDb chan utils.DataForDb) {
	// data := utils.DataForDb{}

	err := initDirewolfClient()
	if err != nil {
		fmt.Println("[Direwolf] init failed:", err)
	}
	req, _ := http.NewRequest("GET", direwolfOnion+"api/public/articles", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Content-Type", "application/json")

	resp, err := direwolfClient.Do(req)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesDirewolf, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesDirewolf, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}
	var companies struct {
		Data []utils.CompanyDirewolf `json:"articles"`
	}
	json.Unmarshal(bodyBytesDirewolf, &companies)
	for query := range channel {
		query = strings.TrimSpace(query)
		for _, company := range companies.Data {
			if strings.Contains(strings.ToLower(company.Name), strings.ToLower(query)) {
				chanDataForDb <- utils.DataForDb{
					Source: "direwolf",
					Key:    query,
					Url:    direwolfOnion,
				}
				fmt.Println("[Direwolf] Results found:", direwolfOnion)
			}
		}
	}
}
