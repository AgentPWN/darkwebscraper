package website

import (
	"compress/gzip"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

const darknetOnion = "http://darknet4zvovn77zgkppdrgzuf7i3kvn5aepmjp6g6djyyzxwyjyfcyd.onion"

var darknetClient *http.Client
var bodyBytesDarknet []byte

func initDarknetClient() error {
	if darknetClient != nil {
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

	darknetClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Darknet(query string) bool {
	if err := initDarknetClient(); err != nil {
		fmt.Println("[Darknet] init failed:", err)
		return false
	}
	// fmt.Println("[Darknet]")
	darknetOnionQueryURL := darknetOnion + "/search/17557447/?q=" + query + "&o=date"
	req, _ := http.NewRequest("GET", darknetOnionQueryURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := darknetClient.Do(req)
	// fmt.Println(resp)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesDarknet, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesDarknet, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}
	body := string(bodyBytesDarknet)

	if strings.Contains(body, "No results found.") {
		fmt.Println("[Darknet] No results found")
		return false
	} else {
		// fmt.Println(body)
		fmt.Println("[Darknet] Results found")
		return true
	}
}
