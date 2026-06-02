package website

import (
	"crypto/tls"
	"darkwebscraper/utils"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

const kazuOnion = "http://6czlbd2jfiy6765fbnbnzuwuqocg57ebvp3tbm35kib425k4qnmiiiqd.onion/databases.html"

var kazuClient *http.Client

func initKazuClient() error {
	if kazuClient != nil {
		return nil
	}

	torDialer, err := proxy.SOCKS5("tcp", "localhost:9050", nil, nil)
	if err != nil {
		return fmt.Errorf("proxy.SOCKS5: %w", err)
	}

	transport := &http.Transport{
		DialContext:     torDialer.(proxy.ContextDialer).DialContext,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	kazuClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

type KazuPlatform struct {
	Title       string `json:"title"`
	Logo        string `json:"logo"`
	Link        string `json:"link"`
	Description string `json:"description"`
	Price       string `json:"price"`
	DumpDate    string `json:"dumpDate"`
	Size        string `json:"size"`
	Record      string `json:"record"`
	SampleUrl   string `json:"sampleUrl"`
	SampleName  string `json:"sampleName"`
}

func Kazu(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}

	if err := initKazuClient(); err != nil {
		fmt.Println("[Kazu] init failed:", err)
	}

	req, _ := http.NewRequest("GET", kazuOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := kazuClient.Do(req)
	if err != nil {
		fmt.Println("[Kazu] request failed:", err)
		return
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("[Kazu] body read error:", err)
		return
	}
	htmlStr := string(bodyBytes)

	// Extract the platforms array from the script tag
	platformsRe := regexp.MustCompile(`(?s)const platforms = \[(.*?)]\s*;`)
	match := platformsRe.FindStringSubmatch(htmlStr)
	if len(match) < 2 {
		fmt.Println("[Kazu] platforms array not found")
		return
	}
	jsArray := "[" + match[1] + "]"

	// Convert JS object keys to quoted JSON keys
	// Replace id: 1, -> "id": 1,
	// Replace title: "...", -> "title": "...",
	jsonLike := regexp.MustCompile(`(\w+)\s*:`).ReplaceAllString(jsArray, `"$1":`)
	// Replace single quotes with double quotes (if any)
	jsonLike = strings.ReplaceAll(jsonLike, "'", "\"")

	var platforms []KazuPlatform
	err = json.Unmarshal([]byte(jsonLike), &platforms)
	if err != nil {
		fmt.Println("[Kazu] JSON parse error:", err)
		return
	}

	for query := range channel {
		query = strings.TrimSpace(query)
		for _, p := range platforms {
			if strings.Contains(strings.ToLower(p.Title), strings.ToLower(query)) {
				data.Source = "kazu"
				data.Key = query
				data.Url = p.Link
				desc := p.Description + " | Price: " + p.Price + ", DumpDate: " + p.DumpDate + ", Size: " + p.Size + ", Record: " + p.Record + ", Sample: " + p.SampleUrl
				data.Desc = desc
				chanDataForDb <- data
				fmt.Println("[Kazu] Results found:", data.Key, data.Url)
			}
		}
	}
}
