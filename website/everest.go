package website

import (
	"context"
	"crypto/tls"
	"darkwebscraper/utils"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"golang.org/x/net/proxy"
)

// This script changes the pattern a bit
// Instead of using the client only for connecting to the website
// It uses the client to actually fetch the html
// Because the html has to be brought from the browser window
// The website makes you wait every time you visit
// This felt like the easiest way to achieve fetching the html

const everestOnion = "http://ransomocmou6mnbquqz44ewosbkjk3o5qjsl3orawojexfook2j7esad.onion/news/"

var everestClient *http.Client

// var bodyBytesEverest []bytes
var htmlBody string

func initEverestClient() error {
	if everestClient != nil {
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

	ctx, cancelTimeout := context.WithTimeout(ctx, 120*time.Second)
	defer cancelTimeout()

	err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			err := chromedp.EvaluateAsDevTools(
				fmt.Sprintf(`window.location.href = "%s"`, everestOnion),
				nil,
			).Do(ctx)
			return err
		}),

		// wait for initial page load
		// chromedp.Poll(`document.readyState === "complete"`, nil),

		// allow JS rendering / animations to finish
		chromedp.Sleep(20*time.Second),

		// now capture final DOM
		chromedp.EvaluateAsDevTools(
			`document.documentElement.outerHTML`,
			&htmlBody,
		),
	)
	// fmt.Println(html)
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

	everestClient = &http.Client{
		Transport: transport,
		Jar:       jar,
		Timeout:   120 * time.Second,
	}

	baseURL, _ := url.Parse(everestOnion)
	jar.SetCookies(baseURL, cookies)

	return nil
}

func Everest(query string, chanDataForDb chan utils.DataForDb) bool {
	if err := initEverestClient(); err != nil {
		fmt.Println("[Everest] init failed:", err)
		return false
	}

	body := htmlBody
	var result bool = false
	if strings.Contains(body, query) {
		// links := utils.ExtractPostLinks(body, "")
		// for _, link := range links {
		// 	fmt.Println(everestOnion + link)
		// 	result = true
		// }
		result = true
	}

	if result == true {
		fmt.Println("[Everest] Results found")
		return result
	} else {
		fmt.Println("[Everest] Results not found")
		return result
	}

}
