// this website doesn't have a login page but the login logic from pwnForums has been refactored for ease
package website

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"darkwebscraper/utils"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"golang.org/x/net/proxy"
)

const bravoxOnion = "http://bravoxxtrmqeeevhl7gdh2yzvlrjxajr66d33c7ozosrccx4cz7cepad.onion"

var bravoxClient *http.Client
var bodyBytesBravox []byte

// func initbravoxClient() error {

// }

func initBravoxClient() ([]*http.Cookie, error) {
	if bravoxClient != nil {
		return nil, nil
	}
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("proxy-server", "socks5://127.0.0.1:9050"),
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("ignore-certificate-errors", true),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAlloc()

	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	defer cancelCtx()

	if err := chromedp.Run(ctx, network.Enable()); err != nil {
		return nil, err
	}

	if err := chromedp.Run(ctx,
		chromedp.Navigate(bravoxOnion),
	); err != nil {
		return nil, err
	}

	fmt.Println("Waiting for login...")

	var chromeCookies []*network.Cookie
	for {
		err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			chromeCookies, err = network.GetCookies().Do(ctx)
			return err
		}))
		if err != nil {
			return nil, err
		}

		loggedIn := false
		for _, c := range chromeCookies {
			if c.Name == "pow-id" {
				loggedIn = true
				break
			}
		}

		if loggedIn {
			fmt.Println("Login detected")
			break
		}

		time.Sleep(1 * time.Second)
	}

	cookies := make([]*http.Cookie, 0, len(chromeCookies))
	for _, c := range chromeCookies {
		cookies = append(cookies, &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Secure:   c.Secure,
			HttpOnly: c.HTTPOnly,
		})
	}
	torDialer, err := proxy.SOCKS5("tcp", "localhost:9050", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("proxy.SOCKS5: %w", err)
	}
	jar, _ := cookiejar.New(nil)

	transport := &http.Transport{
		DialContext: torDialer.(proxy.ContextDialer).DialContext,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	bravoxClient = &http.Client{
		Transport: transport,
		Jar:       jar,
		Timeout:   60 * time.Second,
	}
	baseURL, _ := url.Parse(bravoxOnion)
	jar.SetCookies(baseURL, cookies)

	return cookies, nil
}

func Bravox(channel chan string, chanDataForDb chan utils.DataForDb) {
	// data := utils.DataForDb{}

	cookies, err := initBravoxClient()
	fmt.Println(cookies)
	if err != nil {
		fmt.Println("[Bravox] init failed:", err)
	}
	jsonBody := []byte(`{"page":1,"limit":100}`)
	req, _ := http.NewRequest("POST", bravoxOnion+"/api/posts/filter", bytes.NewBuffer(jsonBody))
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	// req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Content-Type", "application/json")
	// req.Header.Set("Accept-Encoding", "gzip")

	resp, err := bravoxClient.Do(req)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesBravox, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesBravox, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}
	var companies struct {
		Data []utils.CompanyBravox `json:"data"`
	}
	json.Unmarshal(bodyBytesBravox, &companies)
	for query := range channel {
		query = strings.TrimSpace(query)
		for _, company := range companies.Data {
			if strings.Contains(strings.ToLower(company.Campaign.Name), strings.ToLower(query)) || strings.Contains(strings.ToLower(company.Desc), strings.ToLower(query)) {
				desc := company.Desc
				chanDataForDb <- utils.DataForDb{
					Source: "bravox",
					Key:    query,
					Url:    bravoxOnion + "/blog/" + company.ID,
					Desc:   desc,
				}
				fmt.Println("[Bravox] Results found:", bravoxOnion+"/blog/"+company.ID)
			}
		}
	}
}
