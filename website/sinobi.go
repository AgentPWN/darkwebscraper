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

type SinobiResponse struct {
	Payload struct {
		Posts []utils.CompanySinobi `json:"announcements"`
	} `json:"payload"`
}

const sinobiOnion = "http://sinobi23i75c3znmqqxxyuzqvhxnjsar7actgvc4nqeuhgcn5yvz3zqd.onion/"

var sinobiClient *http.Client
var bodyBytesSinobi []byte

func initSinobiClient() error {
	if sinobiClient != nil {
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

	sinobiClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Sinobi(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}
	var response SinobiResponse

	if err := initSinobiClient(); err != nil {
		fmt.Println("[Sinobi] init failed:", err)
	}

	req, _ := http.NewRequest("GET", sinobiOnion+"api/v1/blog/get/announcements?page=1&perPage=300", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := sinobiClient.Do(req)
	if err != nil {
		fmt.Println("[Sinobi] request failed:", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			fmt.Println("[Sinobi] gzip reader error:", err)
		}
		bodyBytesSinobi, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			fmt.Println("[Sinobi] gzip read error:", err)
		}
	} else {
		bodyBytesSinobi, err = io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("[Sinobi] body read error:", err)
		}
	}

	err = json.Unmarshal(bodyBytesSinobi, &response)
	if err != nil {
		fmt.Println("[Sinobi] JSON parse error:", err)
	}

	found := false
	for query := range channel {
		query = strings.TrimSpace(query)
		// fmt.Println(bodyBytesSinobi)
		for _, c := range response.Payload.Posts {
			if strings.Contains(strings.ToLower(c.Company.Name), strings.ToLower(query)) {
				data.Source = "sinobi"
				data.Key = query
				data.Url = sinobiOnion
				desc, _ := utils.URLDecode(strings.Join(c.Desc, " "))
				data.Desc = desc
				chanDataForDb <- data
				fmt.Println(data.Key, data.Url)
				fmt.Println("[Sinobi] Results found")
				found = true
			}
		}

	}

	if !found {
		fmt.Println("[Sinobi] No results found")
	}

}
