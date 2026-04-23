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

const incRansomOnionApi = "http://incbacg6bfwtrlzwdbqc55gsfl763s3twdtwhp27dzuik6s6rwdcityd.onion/"
const incRansomOnionBlog = "http://incblog6qu4y4mm4zvw5nrmue6qbwtgjsxpw6b7ixzssu36tsajldoad.onion/blog/disclosures/"

var incRansomClient *http.Client
var bodyBytesIncRansom []byte

func initIncRansomClient() error {
	if incRansomClient != nil {
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

	incRansomClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func IncRansom(query string, chanDataForDb chan utils.DataForDb) bool {
	data := utils.DataForDb{}
	var companies []utils.CompanyIncRansom // changed from utils.Company
	if err := initIncRansomClient(); err != nil {
		fmt.Println("[IncRansom] init failed:", err)
		return false
	}

	req, _ := http.NewRequest("GET", incRansomOnionApi+"api/v1/blog/get/announcements?page=1&perPage=1000", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := incRansomClient.Do(req)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesIncRansom, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesIncRansom, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	body := string(bodyBytesIncRansom)

	// unmarshal into the wrapper to reach payload.announcements
	var apiResp struct {
		Payload struct {
			Announcements []utils.CompanyIncRansom `json:"announcements"`
		} `json:"payload"`
	}
	err = json.Unmarshal(bodyBytesIncRansom, &apiResp)
	if err != nil {
		fmt.Println("[IncRansom] JSON parse error:", err)
		fmt.Println(string(bodyBytesIncRansom))
		return false
	}
	companies = apiResp.Payload.Announcements // keep using `companies` throughout

	if !strings.Contains(body, query) {
		fmt.Println("[IncRansom] No results found")
		return false
	} else {
		for _, c := range companies {
			if strings.Contains(c.Company.Name, query) { // c.Company.Name for nested field
				url := incRansomOnionBlog + c.ID
				data.Source = "incRansom"
				data.Key = query
				data.Url = url
				desc, _ := utils.URLDecode(strings.Join(c.Desc, " "))
				data.Desc = desc // description is []string
				chanDataForDb <- data
				fmt.Println(data.Key, data.Url)
				fmt.Println("[IncRansom] Result found")
			}
		}
		return true
	}
}
