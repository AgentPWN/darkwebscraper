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

const gunraOnion = "http://www.lgiil72vkmdtbc3qv4tyq6wedyjxqr2qd4ze7xl2cxgerdnymxj7soqd.onion/"

var gunraClient *http.Client
var bodyBytesGunra []byte

func initGunraClient() error {
	if gunraClient != nil {
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

	gunraClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Gunra(query string, chanDataForDb chan utils.DataForDb) bool {
	data := utils.DataForDb{}
	var companies []utils.CompanyGunra
	if err := initGunraClient(); err != nil {
		fmt.Println("[Gunra] init failed:", err)
		return false
	}
	// go run main.go
	// fmt.Println("[Gunra]")
	req, _ := http.NewRequest("GET", gunraOnion+"api/public/companies", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := gunraClient.Do(req)
	// fmt.Println(resp)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesGunra, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesGunra, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	// fmt.Println("body found")
	body := string(bodyBytesGunra)
	// fmt.Println(body)
	err = json.Unmarshal(bodyBytesGunra, &companies)
	if err != nil {
		fmt.Println("[Gunra] JSON parse error:", err)
		fmt.Println(string(bodyBytesGunra)) // debug for truncated responses
		return false
	}
	for _, c := range companies {
		if strings.Contains(c.Name, query) {
			url := gunraOnion + "/company/" + c.ID
			data.Source = "gunra"
			data.Key = query
			data.Url = url
			data.Desc = "lorem ipsum"
			chanDataForDb <- data
			fmt.Println(data.Key, data.Url)
		}
	}

	if !strings.Contains(body, query) {
		fmt.Println("[Gunra] No results found")
		return false
	} else {

		return true
	}
	//  else {
	// 	fmt.Println("[Gunra] No data found")
	// 	return false
	// }
}
