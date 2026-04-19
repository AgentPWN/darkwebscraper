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

const ransomexxOnion = "http://rnsm777cdsjrsdlbs4v5qoeppu3px6sb2igmh53jzrx7ipcrbjz5b2ad.onion/"

var ransomexxClient *http.Client
var bodyBytesRansomexx []byte

func initRansomexxClient() error {
	if ransomexxClient != nil {
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

	ransomexxClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func scanRansomexx(url string, query string) bool {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	// req.Header.Set("Accept-Encoding", "gzip")

	resp, err := ransomexxClient.Do(req)

	if err != nil {
		fmt.Println("[Ransomexx] request failed:", err)
	}
	bodyBytesRansomexx, err = io.ReadAll(resp.Body)
	body := string(bodyBytesRansomexx)
	if err != nil {
		panic(err)
	}
	var result bool = false
	// fmt.Println(body)
	if strings.Contains(body, query) {
		result = true
		links := utils.ExtractPostLinks(body, "")
		for _, link := range links {
			// fmt.Println(link)
			if strings.Contains(link, strings.ToLower(query)) {
				fmt.Println(ransomexxOnion + link)
				result = true
			}
		}
	}
	return result
}

func Ransomexx(query string, chanDataForDb chan utils.DataForDb) bool {
	if err := initRansomexxClient(); err != nil {
		fmt.Println("[Ransomexx] init failed:", err)
		return false
	}
	var result bool
	result = scanRansomexx(ransomexxOnion, query)
	for i := range 7 {
		ransomexxOnionCurrent := fmt.Sprintf("%sindex%d.html", ransomexxOnion, i+1)
		if result == false {
			result = scanRansomexx(ransomexxOnionCurrent, query)
			// this does not work fully because it doesn't keep checking for all results
			// it exits the moment it finds a result
		}

	}
	if result == true {
		fmt.Println("[Ransomexx] Results found")
		return result
	} else {
		fmt.Println("[Ransomexx] Results not found")
		return result
	}

}
