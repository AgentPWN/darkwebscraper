package website

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"darkwebscraper/utils"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"golang.org/x/net/proxy"
)

const dreadOnion = "http://dreadytofatroptsdj6io7l3xptbet6onoyno2yv7jicoxknyazubrad.onion"

var dreadClient *http.Client
var bodyBytesDread []byte

func initDreadClient() error {
	if dreadClient != nil {
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

	fmt.Println("[initDreadClient] navigating to Dread, waiting for queue to resolve…")

	var html string
	err := chromedp.Run(ctx,
		chromedp.Navigate(dreadOnion),
		chromedp.Sleep(30*time.Second),
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

	dreadClient = &http.Client{
		Transport: transport,
		Jar:       jar,
		Timeout:   60 * time.Second,
	}

	baseURL, _ := url.Parse(dreadOnion)
	jar.SetCookies(baseURL, cookies)

	return nil
}

func Dread(query string) bool {
	if err := initDreadClient(); err != nil {
		panic(err)
	}

	targetURL := dreadOnion + "/search/?q=" + url.QueryEscape(query)

	for {
		req, _ := http.NewRequest("GET", targetURL, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")
		req.Header.Set("Accept-Encoding", "gzip")

		resp, err := dreadClient.Do(req)
		if err != nil {
			fmt.Println("[Dread] request failed:", err)
			time.Sleep(10 * time.Second)
			continue
		}

		if resp.Header.Get("Content-Encoding") == "gzip" {
			reader, err := gzip.NewReader(resp.Body)
			if err != nil {
				resp.Body.Close()
				continue
			}
			bodyBytesDread, err = io.ReadAll(reader)
			reader.Close()
			if err != nil {
				resp.Body.Close()
				continue
			}
		} else {
			bodyBytesDread, err = io.ReadAll(resp.Body)
			if err != nil {
				resp.Body.Close()
				continue
			}
		}
		resp.Body.Close()

		body := string(bodyBytesDread)

		switch {
		case strings.Contains(body, "No results"):
			fmt.Println("[Dread] no results")
			return false

		case strings.Contains(body, "Exactly"):
			fmt.Println("[Dread] Found result")

			links := utils.ExtractPostLinks(body, "/post/")
			for _, link := range links {
				fmt.Println("Post link:", dreadOnion+link)
			}
			return true

		case strings.Contains(body, "queue"):
			fmt.Println("[Dread] still in queue, retrying…")
			time.Sleep(time.Duration(8+rand.Intn(5)) * time.Second)
			continue
		}
	}
}
