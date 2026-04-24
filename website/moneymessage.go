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

// type MoneyMessageResponse struct {
// 	Posts []utils.CompanyMoneyMessage `json:"posts"`
// }

const moneyMessageOnion = "http://blogvl7tjyjvsfthobttze52w36wwiz34hrfcmorgvdzb6hikucb7aqd.onion/"

var moneyMessageClient *http.Client
var bodyBytesMoneyMessage []byte

func initMoneyMessageClient() error {
	if moneyMessageClient != nil {
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

	moneyMessageClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func MoneyMessage(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}
	var companies []utils.CompanyMoneyMessage

	if err := initMoneyMessageClient(); err != nil {
		fmt.Println("[MoneyMessage] init failed:", err)
	}

	req, _ := http.NewRequest("GET", moneyMessageOnion+"news.php?allNewsPage=1", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := moneyMessageClient.Do(req)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesMoneyMessage, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesMoneyMessage, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	err = json.Unmarshal(bodyBytesMoneyMessage, &companies)
	if err != nil {
		fmt.Println("[MoneyMessage] JSON parse error:", err)
	}
	for query := range channel {
		query = strings.TrimSpace(query)
		for _, c := range companies {
			if strings.Contains(c.Name, query) {
				url := moneyMessageOnion + "news.php/?id=" + strconv.Itoa(c.ID)
				data.Source = "moneyMessage"
				data.Key = query
				data.Url = url
				data.Desc = "Lorem Ipsum"
				chanDataForDb <- data
				fmt.Println(data.Key, data.Url)
				fmt.Println("[MoneyMessage] Results found:", data.Key, data.Url)
			}
		}
	}
}
