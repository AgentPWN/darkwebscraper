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

type LamashtuResponse struct {
	Posts []utils.CompanyLamashtu `json:"posts"`
}

const lamashtuOnion = "http://lamashtux5j74mcm7lwwgn5yrvuwtrpxjoyendif3v3hrztjesfoyayd.onion/"

var lamashtuClient *http.Client
var bodyBytesLamashtu []byte

func initLamashtuClient() error {
	if lamashtuClient != nil {
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

	lamashtuClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Lamashtu(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}
	var response LamashtuResponse

	if err := initLamashtuClient(); err != nil {
		fmt.Println("[Lamashtu] init failed:", err)
	}

	req, _ := http.NewRequest("GET", lamashtuOnion+"api/posts?page=1&limit=6&filter=all", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := lamashtuClient.Do(req)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesLamashtu, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesLamashtu, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	err = json.Unmarshal(bodyBytesLamashtu, &response)
	if err != nil {
		fmt.Println("[Lamashtu] JSON parse error:", err)
	}
	for query := range channel {
		query = strings.TrimSpace(query)
		for _, c := range response.Posts {
			if strings.Contains(c.Name, query) || strings.Contains(c.Desc, query) {
				url := lamashtuOnion + "post/" + c.ID
				data.Source = "lamashtu"
				data.Key = query
				data.Url = url
				data.Desc = c.Desc
				chanDataForDb <- data
				fmt.Println(data.Key, data.Url)
				fmt.Println("[Lamashtu] Results found: ", data.Key, data.Url)
			}
		}
	}
}
