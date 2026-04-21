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

type KillSecResponse struct {
	Posts []utils.CompanyKillSec `json:"posts"`
}

const killSecOnion = "http://ks5424y3wpr5zlug5c7i6svvxweinhbdcqcfnptkfcutrncfazzgz5id.onion/"

var killSecClient *http.Client
var bodyBytesKillSec []byte

func initKillSecClient() error {
	if killSecClient != nil {
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

	killSecClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func KillSec(query string, chanDataForDb chan utils.DataForDb) bool {
	data := utils.DataForDb{}
	var response KillSecResponse

	if err := initKillSecClient(); err != nil {
		fmt.Println("[KillSec] init failed:", err)
		return false
	}

	req, _ := http.NewRequest("GET", killSecOnion+"api/data-x7g9k2.php?force_db=1", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := killSecClient.Do(req)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesKillSec, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesKillSec, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	body := string(bodyBytesKillSec)
	fmt.Println(body)
	err = json.Unmarshal(bodyBytesKillSec, &response)
	if err != nil {
		fmt.Println("[KillSec] JSON parse error:", err)
		return false
	}

	for _, c := range response.Posts {
		if strings.Contains(c.Name, query) {
			url := killSecOnion + "post/" + c.ID
			data.Source = "killSec"
			data.Key = query
			data.Url = url
			data.Desc = "Lorem Ipsum"
			chanDataForDb <- data
			fmt.Println(data.Key, data.Url)
			fmt.Println("[KillSec] Results found")
		}
	}

	if !strings.Contains(body, query) {
		fmt.Println("[KillSec] No results found")
		return false
	}

	return true
}
