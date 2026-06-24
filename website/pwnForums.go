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

const pwnForumsOnion = "http://pwnfrm7rbf6kyerigxi677lcz5ifmoagdbqqknwdu2by27wfdst5qmqd.onion"

var pwnForumsClient *http.Client
var bodyBytesPwnForums []byte

// func initPwnForumsClient() error {

// }

func initPwnForumsClient() ([]*http.Cookie, error) {
	if pwnForumsClient != nil {
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
		chromedp.Navigate("http://pwnfrm7rbf6kyerigxi677lcz5ifmoagdbqqknwdu2by27wfdst5qmqd.onion/member.php?action=login"),
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
			if c.Name == "mybbuser" {
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

	pwnForumsClient = &http.Client{
		Transport: transport,
		Jar:       jar,
		Timeout:   60 * time.Second,
	}
	baseURL, _ := url.Parse(pwnForumsOnion)
	jar.SetCookies(baseURL, cookies)

	return cookies, nil
}

func PwnForums(channel chan string, chanDataForDb chan utils.DataForDb) {
	// data := utils.DataForDb{}
	// if err := initPwnForumsClient(); err != nil {
	// 	panic(err)
	// }
	_, err := initPwnForumsClient()
	if err != nil {
		panic(err)
	}
	// fmt.Println(cookies)
	// for _, c := range cookies {
	// 	fmt.Printf("%s=%s\n", c.Name, c.Value)
	// }
	for query := range channel {
		query = strings.TrimSpace(query)
		targetURL := pwnForumsOnion + "/search.php?action=do_search&keywords=" + query + "&postthread=1&author=&matchusername=1&forums[]=all&findthreadst=1&numreplies=&postdate=0&pddir=1&threadprefix[]=any&sortby=lastpost&sortordr=desc&showresults=threads&submit=Search"
		reqData := url.Values{}
		// reqData.Set("keywords", query)
		// reqData.Set("c[users]", "")
		// reqData.Set("_xfToken", token)
		req, _ := http.NewRequest("GET", targetURL, strings.NewReader(reqData.Encode()))
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept-Encoding", "gzip")
		// for _, c := range cookies {
		// 	req.AddCookie(c)
		// }
		resp, err := pwnForumsClient.Do(req)
		if err != nil {
			fmt.Println("[PwnForums] request failed:", err)
			time.Sleep(10 * time.Second)
			continue
		}

		if resp.Header.Get("Content-Encoding") == "gzip" {
			reader, err := gzip.NewReader(resp.Body)
			if err != nil {
				resp.Body.Close()
				continue
			}
			bodyBytesPwnForums, err = io.ReadAll(reader)
			reader.Close()
			if err != nil {
				resp.Body.Close()
				continue
			}
		} else {
			bodyBytesPwnForums, err = io.ReadAll(resp.Body)
			if err != nil {
				resp.Body.Close()
				continue
			}
		}
		resp.Body.Close()

		body := string(bodyBytesPwnForums)

		// fmt.Println(body)
		switch {
		case strings.Contains(body, "no results"):
			fmt.Println("[PwnForums] no results")

		case strings.Contains(body, "Search Results"):
			fmt.Println("[PwnForums] Found result")

			// links := utils.ExtractPostLinks(body, "/threads/")
			// for _, link := range links {
			// 	fmt.Println("Post link:", pwnForumsOnion+link)
			// 	data.Source = "pwnForums"
			// 	data.Key = query
			// 	data.Url = pwnForumsOnion + link
			// 	data.Desc = "lorem ipsum"
			// 	chanDataForDb <- data
			// }
		default:
			fmt.Println("[PwnForums] some error occured")
			// case strings.Contains(body, "queue"):
			// 	fmt.Println("[PwnForums] still in queue, retrying…")
			// 	continue
			// }

		}

		time.Sleep(time.Duration(30+rand.Intn(5)) * time.Second)
	}
}
