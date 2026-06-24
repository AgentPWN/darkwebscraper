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

const ransomhouseOnion = "http://zohlm7ahjwegcedoz7lrdrti7bvpofymcayotp744qhx6gjmxbuo2yid.onion/"

type ransomhouseResponse struct {
	Data []struct {
		ID      string `json:"id"`
		Header  string `json:"header"`
		URL     string `json:"url"`
		Info    string `json:"info"`
		Content string `json:"content"`
		Views   string `json:"views"`
		Status  string `json:"status"`
		Action  string `json:"action"`
	} `json:"data"`
}

var ransomhouseClient *http.Client

func initRansomhouseClient() error {
	if ransomhouseClient != nil {
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

	ransomhouseClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Ransomhouse(channel chan string, chanDataForDb chan utils.DataForDb) {
	if err := initRansomhouseClient(); err != nil {
		fmt.Println("[Ransomhouse] init failed:", err)
		return
	}

	var response ransomhouseResponse

	req, err := http.NewRequest("GET", ransomhouseOnion+"a", nil)
	if err != nil {
		fmt.Println("[Ransomhouse] request build failed:", err)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "application/json,text/plain,*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := ransomhouseClient.Do(req)
	if err != nil {
		fmt.Println("[Ransomhouse] request failed:", err)
		return
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			fmt.Println("[Ransomhouse] gzip reader error:", err)
			return
		}
		bodyBytes, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			fmt.Println("[Ransomhouse] gzip read error:", err)
			return
		}
	} else {
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("[Ransomhouse] body read error:", err)
			return
		}
	}

	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		fmt.Println("[Ransomhouse] JSON parse error:", err)
		return
	}

	for query := range channel {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}
		needle := strings.ToLower(query)
		for _, entry := range response.Data {
			haystack := strings.ToLower(entry.Header + " " + entry.Info + " " + entry.URL)
			if !strings.Contains(haystack, needle) {
				continue
			}

			url := strings.TrimSpace(entry.URL)
			if url == "" {
				if strings.TrimSpace(entry.Content) != "" {
					url = ransomhouseOnion + strings.TrimPrefix(strings.TrimSpace(entry.Content), "/")
				} else {
					url = ransomhouseOnion
				}
			}

			descParts := []string{}
			if entry.Info != "" {
				descParts = append(descParts, entry.Info)
			}
			if entry.Status != "" {
				descParts = append(descParts, "status: "+entry.Status)
			}
			if entry.Action != "" {
				descParts = append(descParts, "action: "+entry.Action)
			}
			if entry.Views != "" {
				descParts = append(descParts, "views: "+entry.Views)
			}

			chanDataForDb <- utils.DataForDb{
				Source: "ransomhouse",
				Key:    query,
				Url:    url,
				Desc:   strings.Join(descParts, " | "),
			}
			fmt.Println("[Ransomhouse] Results found:", query, url)
		}
	}
}
