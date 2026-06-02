package website

import (
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

type AkiraResponse struct {
	Posts []utils.CompanyAkira `json:"objects"`
}

const akiraOnion = "https://akiral2iz6a7qgd3ayp3l6yub7xx2uep76idk3u2kollpj5z3z636bad.onion/"

var akiraClient *http.Client
var bodyBytesAkira []byte

// cancelAkiraAlloc and cancelAkiraCtx are kept alive for the lifetime of the
// process so the browser session (and its cookies) remain valid after
// initAkiraClient returns.  Call cancelAkiraAlloc() to shut the browser down.
var cancelAkiraAlloc context.CancelFunc
var cancelAkiraCtx context.CancelFunc

func initAkiraClient() error {
	if akiraClient != nil {
		return nil
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
		return fmt.Errorf("enable network domain: %w", err)
	}

	fmt.Println("[initAkiraClient] navigating to Akira, waiting for queue to resolve…")

	var html string
	err := chromedp.Run(ctx,
		chromedp.Navigate(akiraOnion),
		chromedp.Sleep(60*time.Second),
		chromedp.OuterHTML("html", &html),
	)
	if err != nil {
		return fmt.Errorf("chromedp navigate/wait: %w", err)
	}

	var chromeCookies []*network.Cookie
	if err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		var cdpErr error
		chromeCookies, cdpErr = network.GetCookies().Do(ctx)
		return cdpErr
	})); err != nil {
		return fmt.Errorf("get cookies: %w", err)
	}

	var cookies []*http.Cookie
	for _, c := range chromeCookies {
		cookies = append(cookies, &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Path:     c.Path,
			Domain:   c.Domain,
			Secure:   c.Secure,
			HttpOnly: c.HTTPOnly,
		})
	}

	jar, _ := cookiejar.New(nil)

	torDialer, err := proxy.SOCKS5("tcp", "localhost:9050", nil, nil)
	if err != nil {
		return fmt.Errorf("proxy.SOCKS5: %w", err)
	}

	transport := &http.Transport{
		DialContext: torDialer.(proxy.ContextDialer).DialContext,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		DisableKeepAlives: true,
	}

	akiraClient = &http.Client{
		Transport: transport,
		Jar:       jar,
		Timeout:   60 * time.Second,
	}

	baseURL, _ := url.Parse(akiraOnion)
	jar.SetCookies(baseURL, cookies)

	return nil
}

func Akira(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}
	var response AkiraResponse

	if err := initAkiraClient(); err != nil {
		fmt.Println("[Akira] init failed:", err)
	}

	req, _ := http.NewRequest("GET", akiraOnion+"l?page=1&sort=date:desc", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Referrer", akiraOnion)
	fmt.Println(akiraClient.Jar)
	resp, err := akiraClient.Do(req)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesAkira, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesAkira, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}
	fmt.Println(string(bodyBytesAkira))
	err = json.Unmarshal(bodyBytesAkira, &response)
	if err != nil {
		fmt.Println("[Akira] JSON parse error:", err)
	}
	for query := range channel {
		query = strings.TrimSpace(query)
		for _, c := range response.Posts {
			if strings.Contains(c.Name, query) || strings.Contains(c.Desc, query) {
				url := akiraOnion
				data.Source = "akira"
				data.Key = query
				data.Url = url
				data.Desc = c.Desc
				chanDataForDb <- data
				fmt.Println(data.Key, data.Url)
				fmt.Println("[Akira] Results found: ", data.Key, data.Url)
			}
		}
	}
}
