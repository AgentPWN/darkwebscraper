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

// the links provided in this website to access data do not work

const warlockOnion = "http://warlock4fagqhnfuxtcmncfepe3jc33e33dmj2jsk64svxaerm5zhaqd.onion/"

var warlockClient *http.Client
var bodyBytesWarlock []byte

func initWarlockClient() error {
	if warlockClient != nil {
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

	warlockClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Warlock(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}
	var companies []utils.CompanyWarlock

	if err := initWarlockClient(); err != nil {
		fmt.Println("[Warlock] init failed:", err)
	}

	req, _ := http.NewRequest("GET", warlockOnion+"api?action=get_public_clients", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := warlockClient.Do(req)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesWarlock, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesWarlock, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	// body := string(bodyBytesWarlock)

	err = json.Unmarshal(bodyBytesWarlock, &companies)
	if err != nil {
		fmt.Println("[Warlock] JSON parse error:", err)
	}
	for query := range channel {
		query = strings.TrimSpace(query)

		for _, c := range companies {
			if strings.Contains(c.Name, query) || strings.Contains(c.Desc, query) {
				url := warlockOnion + "files.html?clientId=" + strconv.Itoa(c.ID)
				data.Source = "warlock"
				data.Key = query
				data.Url = url
				data.Desc = c.Desc
				chanDataForDb <- data
				fmt.Println(data.Key, data.Url)
				fmt.Println("[Warlock] Results found")
			}
		}
		// if !strings.Contains(body, query) {
		// 	fmt.Println("[Warlock] No results found")
		// }
	}

}
