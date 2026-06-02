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

type radarResponse []radarItem

type radarItem struct {
	ID          int    `json:"id"`
	Status      int    `json:"status"`
	Blog        string `json:"blog"`
	CompanyName string `json:"company_name"`
	Description string `json:"description"`
	Website     string `json:"website"`
	User        struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Login    string `json:"login"`
		RoleID   int    `json:"role_id"`
		StatusID int    `json:"status_id"`
		Password string `json:"password"`
	} `json:"user"`
	LogoURL string `json:"logo_url"`
	Screenshots []struct {
		ID       int    `json:"id"`
		ImageURL string `json:"image_url"`
	} `json:"screenshots"`
	Videos     []any `json:"videos"`
	URLs       []struct {
		ID  int    `json:"id"`
		URL string `json:"url"`
	} `json:"urls"`
	Payout     int    `json:"payout"`
	PayoutUnit int    `json:"payout_unit"`
	Builder    int    `json:"builder"`
	Publish    int    `json:"publish"`
	IsAccept   int    `json:"is_accept"`
	CreatedAt  string `json:"created_at"`
	Expires    string `json:"expires"`
}

const radarOnion = "http://3bnusfu2lgk5at43ceu7cdok5yv4gfbono2jv57ho74ucjvc7czirfid.onion/"

var radarClient *http.Client
var bodyBytesRadar []byte

func initRadarClient() error {
	if radarClient != nil {
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

	radarClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Radar(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}
	var response radarResponse

	if err := initRadarClient(); err != nil {
		fmt.Println("[Radar] init failed:", err)
		return
	}

	req, _ := http.NewRequest("GET", radarOnion+"api/leakeds_a", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "application/json,text/plain,*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := radarClient.Do(req)
	if err != nil {
		fmt.Println("[Radar] request failed:", err)
		return
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			fmt.Println("[Radar] gzip reader error:", err)
			return
		}
		bodyBytesRadar, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			fmt.Println("[Radar] gzip read error:", err)
			return
		}
	} else {
		bodyBytesRadar, err = io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("[Radar] body read error:", err)
			return
		}
	}

	err = json.Unmarshal(bodyBytesRadar, &response)
	if err != nil {
		fmt.Println("[Radar] JSON parse error:", err)
		return
	}

	for query := range channel {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}

		for _, item := range response {
			if strings.Contains(strings.ToLower(item.CompanyName), strings.ToLower(query)) ||
				strings.Contains(strings.ToLower(item.Description), strings.ToLower(query)) ||
				strings.Contains(strings.ToLower(item.Website), strings.ToLower(query)) {

				url := radarOnion
				if len(item.URLs) > 0 && strings.TrimSpace(item.URLs[0].URL) != "" {
					url = strings.TrimSpace(item.URLs[0].URL)
				} else if strings.TrimSpace(item.Website) != "" {
					url = strings.TrimSpace(item.Website)
				}

				desc := strings.TrimSpace(item.Description)
				if desc == "" {
					desc = strings.TrimSpace(item.Blog)
				}

				if decoded, decodeErr := utils.URLDecode(desc); decodeErr == nil && decoded != "" {
					desc = decoded
				}

				data.Source = "radar"
				data.Key = query
				data.Url = url
				data.Desc = desc
				chanDataForDb <- data
				fmt.Println("[Radar] Results found:", data.Key, data.Url)
			}
		}
	}
}