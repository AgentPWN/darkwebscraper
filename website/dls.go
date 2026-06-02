package website

import (
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

// DLSCompany represents a single company entry in the DLS API response
// Structure based on data.json
// Add or adjust fields as needed for your use case

type DLSCompany struct {
	ID            string   `json:"id"`
	Company       string   `json:"company"`
	Website       string   `json:"website"`
	Revenue       string   `json:"revenue"`
	Description   string   `json:"description"`
	Status        string   `json:"status"`
	LogoUrl       string   `json:"logoUrl"`
	BackgroundUrl string   `json:"backgroundUrl"`
	PreviewUrls   []string `json:"previewUrls"`
}

type DLSResponse []DLSCompany

const dlsOnion = "http://ctyfftrjgtwdjzlgqh4avbd35sqrs6tde4oyam2ufbjch6oqpqtkdtid.onion/api/publication"

var dlsClient *http.Client

func initDlsClient() error {
	if dlsClient != nil {
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
	dlsClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Dls(channel chan string, chanDataForDb chan utils.DataForDb) {
	if err := initDlsClient(); err != nil {
		fmt.Println("[DLS] init failed:", err)
	}

	req, _ := http.NewRequest("POST", dlsOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Referer", "http://ctyfftrjgtwdjzlgqh4avbd35sqrs6tde4oyam2ufbjch6oqpqtkdtid.onion/")

	resp, err := dlsClient.Do(req)
	if err != nil {
		fmt.Println("[DLS] request failed:", err)
		return
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("[DLS] read body failed:", err)
		return
	}
	var companies DLSResponse
	if err := json.Unmarshal(bodyBytes, &companies); err != nil {
		fmt.Println("[DLS] JSON parse error:", err)
		return
	}

	for query := range channel {
		query = strings.TrimSpace(query)
		for _, c := range companies {
			if strings.Contains(strings.ToLower(c.Company), strings.ToLower(query)) {
				desc := c.Description
				if len(desc) > 300 {
					desc = desc[:300] + "..."
				}
				data := utils.DataForDb{
					Source: "dls",
					Key:    query,
					Url:    c.Website,
					Desc:   desc,
				}
				chanDataForDb <- data
				fmt.Println("[DLS] Match found:", c.Company)
			}
		}
	}
}
