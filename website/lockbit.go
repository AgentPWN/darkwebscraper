package website

import (
	"crypto/tls"
	"darkwebscraper/utils"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

const lockbitOnion = "http://lockbit7z2jwcskxpbokpemdxmltipntwlkmidcll2qirbu7ykg46eyd.onion/"

var lockbitClient *http.Client
var bodyBytesLockbit []byte

func initLockbitClient() error {
	if lockbitClient != nil {
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

	lockbitClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Lockbit(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}
	if err := initLockbitClient(); err != nil {
		fmt.Println("[Lockbit] init failed:", err)
	}
	// fmt.Println("[Lockbit]")
	req, _ := http.NewRequest("GET", lockbitOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := lockbitClient.Do(req)

	if err != nil {
		fmt.Println("[Lockbit] request failed:", err)
	}
	bodyBytesLockbit, err = io.ReadAll(resp.Body)
	body := string(bodyBytesLockbit)
	if err != nil {
		panic(err)
	}
	links := utils.ExtractPostLinks(body, "")
	// fmt.Println(links)
	for query := range channel {
		query = strings.TrimSpace(query)
		for _, link := range links {
			if strings.Contains(link, strings.ToLower(query)) {
				fmt.Println(lockbitOnion + link)
				data.Source = "lockbit"
				data.Key = query
				data.Url = lockbitOnion + link
				data.Desc = "lorem ipsum"
				chanDataForDb <- data
			}
		}
	}
}
