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

type LynxPayload struct {
	Length        int                 `json:"length"`
	Announcements []utils.CompanyLynx `json:"announcements"`
}

type LynxResponse struct {
	Type    bool        `json:"type"`
	Message string      `json:"message"`
	Payload LynxPayload `json:"payload"`
}

const lynxOnion = "http://lynxblogco7r37jt7p5wrmfxzqze7ghxw6rihzkqc455qluacwotciyd.onion/"

var lynxClient *http.Client
var bodyBytesLynx []byte

func initLynxClient() error {
	if lynxClient != nil {
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

	lynxClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Lynx(query string, chanDataForDb chan utils.DataForDb) bool {
	data := utils.DataForDb{}
	var response LynxResponse

	if err := initLynxClient(); err != nil {
		fmt.Println("[Lynx] init failed:", err)
		return false
	}

	req, _ := http.NewRequest("GET", lynxOnion+"api/v1/blog/get/announcements?page=1&perPage=400", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := lynxClient.Do(req)
	if err != nil {
		fmt.Println("[Lynx] request failed:", err)
		return false
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			fmt.Println("[Lynx] gzip reader error:", err)
			return false
		}
		bodyBytesLynx, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			fmt.Println("[Lynx] gzip read error:", err)
			return false
		}
	} else {
		bodyBytesLynx, err = io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("[Lynx] body read error:", err)
			return false
		}
	}

	err = json.Unmarshal(bodyBytesLynx, &response)
	if err != nil {
		fmt.Println("[Lynx] JSON parse error:", err)
		return false
	}

	for _, c := range response.Payload.Announcements {
		if strings.Contains(strings.ToLower(c.Company.Name), strings.ToLower(query)) {
			url := lynxOnion + "leaks/" + c.ID
			data.Source = "lynx"
			data.Key = query
			data.Url = url
			data.Desc = strings.Join(c.Description, " ")
			chanDataForDb <- data
			fmt.Println(data.Key, data.Url)
			fmt.Println("[Lynx] Results found")
		} else {
			fmt.Println("[Lynx] No results found")
			return false
		}
	}

	return true
}
