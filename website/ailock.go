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

const ailockOnion = "http://dhnsppqjaaa22lsqxl2tfhji4ca43743kubltnodvsft3hkvai77p6ad.onion/"

var ailockClient *http.Client

func initAilockClient() error {
	if ailockClient != nil {
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

	ailockClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}
func makeValidJsonAilock(s string) string {
	// s = strings.ReplaceAll(s, "\",", ",")
	// s = strings.ReplaceAll(s, ",\n", "\",\"\n")
	// s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\t", "")
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "<br>", "")
	// s = strings.ReplaceAll(s, ":\"[", ":[")
	// s = strings.ReplaceAll(s, ":\"", ":")
	s = strings.ReplaceAll(s, "{", "{\"")
	s = strings.ReplaceAll(s, "}", "\"}")
	s = strings.ReplaceAll(s, ":", "\":\"")
	s = strings.ReplaceAll(s, ",", "\",\"")

	s = strings.ReplaceAll(s, "}\",\"{", "},{")

	return s
}

func Ailock(channel chan string, chanDataForDb chan utils.DataForDb) {
	var companies []utils.CompanyAilock
	if err := initAilockClient(); err != nil {
		fmt.Println("[Ailock] init failed:", err)
	}

	req, err := http.NewRequest("GET", ailockOnion+"news.js", nil)
	if err != nil {
		fmt.Println("[Ailock] request creation failed:", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "application/javascript,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := ailockClient.Do(req)
	if err != nil {
		fmt.Println("[Ailock] request failed:", err)
	}
	defer resp.Body.Close()

	bodyBytesAilock, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("[Ailock] read body failed:", err)
	}

	entries := bodyBytesAilock[17 : len(bodyBytesAilock)-2]
	err = json.Unmarshal([]byte(makeValidJsonAilock(string(entries))), &companies)
	// fmt.Println(companies)
	fmt.Println(makeValidJsonAilock(string(entries)))
	if err != nil {
		if se, ok := err.(*json.SyntaxError); ok {
			offset := se.Offset
			start := max(0, int(offset)-50)
			end := min(len([]byte(makeValidJsonAilock(string(entries)))), int(offset)+50)

			fmt.Printf("Syntax error at byte offset %d:\n", offset)
			fmt.Println(string([]byte(makeValidJsonAilock(string(entries)))[start:end]))
		} else {
			fmt.Println(err)
		}
	}

	for query := range channel {
		query = strings.TrimSpace(query)
		for _, c := range companies {
			if strings.Contains(c.Title, query) {
				data := utils.DataForDb{
					Source: "ailock",
					Key:    query,
					Url:    "lorem ipsum",
					Desc:   c.Desc,
				}
				chanDataForDb <- data
				fmt.Println("[Ailock] Match found:", c.Title)
			}
		}
	}
}
