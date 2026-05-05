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

const abyssOnion = "http://3ev4metjirohtdpshsqlkrqcmxq6zu3d7obrdhglpy5jpbr7whmlfgqd.onion"

var abyssClient *http.Client

func initAbyssClient() error {
	if abyssClient != nil {
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

	abyssClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}
func makeValidJson(s string) string {
	s = strings.ReplaceAll(s, "+", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\t", "")
	s = strings.ReplaceAll(s, "''", " ")
	s = strings.ReplaceAll(s, "<br>", "")
	s = strings.ReplaceAll(s, "'", "\"")
	s = strings.ReplaceAll(s, "'", "\"")
	s = strings.ReplaceAll(s, "\",]", "\"]")
	return s
}

func splitFull(full string) (name, desc string) {
	parts := strings.SplitN(full, " ", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return strings.TrimSpace(full), ""
}

func Abyss(channel chan string, chanDataForDb chan utils.DataForDb) {
	var companies []utils.CompanyAbyss
	if err := initAbyssClient(); err != nil {
		fmt.Println("[Abyss] init failed:", err)
	}

	req, err := http.NewRequest("GET", abyssOnion+"/static/data.js", nil)
	if err != nil {
		fmt.Println("[Abyss] request creation failed:", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "application/javascript,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := abyssClient.Do(req)
	if err != nil {
		fmt.Println("[Abyss] request failed:", err)
	}
	defer resp.Body.Close()

	bodyBytesAbyss, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("[Abyss] read body failed:", err)
	}

	entries := bodyBytesAbyss[11 : len(bodyBytesAbyss)-1]
	err = json.Unmarshal([]byte(makeValidJson(string(entries))), &companies)
	fmt.Println(companies)
	if err != nil {
		if se, ok := err.(*json.SyntaxError); ok {
			offset := se.Offset
			start := max(0, int(offset)-50)
			end := min(len([]byte(makeValidJson(string(entries)))), int(offset)+50)

			fmt.Printf("Syntax error at byte offset %d:\n", offset)
			fmt.Println(string([]byte(makeValidJson(string(entries)))[start:end]))
		} else {
			fmt.Println(err)
		}
	}

	for query := range channel {
		query = strings.TrimSpace(query)
		for _, c := range companies {
			if strings.Contains(c.Title, query) {
				data := utils.DataForDb{
					Source: "abyss",
					Key:    query,
					Url:    "lorem ipsum",
					Desc:   c.Full,
				}
				chanDataForDb <- data
				fmt.Println("[Abyss] Match found:", c.Title)
			}
		}
	}
}
