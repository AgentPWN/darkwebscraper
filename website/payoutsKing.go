// this website doesn't have a login page but the login logic from pwnForums has been refactored for ease

package website

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"darkwebscraper/utils"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"golang.org/x/net/html"
	"golang.org/x/net/proxy"
)

const payoutsKingOnion = "http://payoutsgn7cy6uliwevdqspncjpfxpmzgirwl2au65la7rfs5x3qnbqd.onion/"

var payoutsKingClient *http.Client
var bodyBytesPayoutsKing []byte

// func initpayoutsKingClient() error {

// }

func initPayoutsKingClient() ([]*http.Cookie, error) {
	if payoutsKingClient != nil {
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
		chromedp.Navigate("http://payoutsgn7cy6uliwevdqspncjpfxpmzgirwl2au65la7rfs5x3qnbqd.onion/"),
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
			if c.Name == "ctp" {
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

	payoutsKingClient = &http.Client{
		Transport: transport,
		Jar:       jar,
		Timeout:   60 * time.Second,
	}
	baseURL, _ := url.Parse(payoutsKingOnion)
	jar.SetCookies(baseURL, cookies)

	return cookies, nil
}

func PayoutsKing(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}

	if _, err := initPayoutsKingClient(); err != nil {
		fmt.Println("[PayoutsKing] init failed:", err)
	}

	req, _ := http.NewRequest("GET", payoutsKingOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := payoutsKingClient.Do(req)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesPayoutsKing, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesPayoutsKing, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytesPayoutsKing)))
	if err != nil {
		panic(err)
	}

	type cardEntry struct {
		company string
		link    string
		desc    string
	}

	var cards []cardEntry

	var innerText func(*html.Node) string
	innerText = func(n *html.Node) string {
		if n == nil {
			return ""
		}
		if n.Type == html.TextNode {
			return n.Data
		}

		var b strings.Builder
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			b.WriteString(innerText(c))
		}
		return strings.TrimSpace(b.String())
	}

	var find func(*html.Node)
	find = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			var id string
			for _, a := range n.Attr {
				if a.Key == "id" {
					id = a.Val
					break
				}
			}

			if id != "" {
				entry := cardEntry{}

				// company name
				var walk func(*html.Node)
				walk = func(c *html.Node) {
					if c.Type == html.ElementNode && c.Data == "p" && hasClass(c, "_title_1lkb3_93") {
						entry.company = strings.TrimSpace(innerText(c))
					}

					for cc := c.FirstChild; cc != nil; cc = cc.NextSibling {
						walk(cc)
					}
				}
				walk(n)

				// details are in the following infoRow
				info := n.NextSibling
				for info != nil &&
					!(info.Type == html.ElementNode &&
						info.Data == "tr" &&
						hasClass(info, "infoRow")) {
					info = info.NextSibling
				}

				if info != nil {
					var walkInfo func(*html.Node)
					walkInfo = func(c *html.Node) {
						if c.Type == html.ElementNode && c.Data == "span" &&
							hasClass(c, "_value_1lkb3_196") &&
							entry.desc == "" {
							entry.desc = strings.TrimSpace(innerText(c))
						}

						if c.Type == html.ElementNode && c.Data == "a" {
							for _, a := range c.Attr {
								if a.Key == "href" &&
									strings.HasPrefix(a.Val, "/") &&
									strings.Contains(innerText(c), "FULL INFO") {
									entry.link = payoutsKingOnion + strings.TrimPrefix(a.Val, "/")
								}
							}
						}

						for cc := c.FirstChild; cc != nil; cc = cc.NextSibling {
							walkInfo(cc)
						}
					}
					walkInfo(info)
				}

				if entry.company != "" {
					cards = append(cards, entry)
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}

	find(doc)

	for query := range channel {
		query = strings.TrimSpace(query)
		for _, card := range cards {
			if strings.Contains(strings.ToLower(card.company), query) {
				link := payoutsKingOnion + card.link
				// if !strings.HasPrefix(link, "http") {
				// 	link = baseURL + strings.TrimPrefix(link, "/")
				// }
				data.Source = "payoutsKing"
				data.Key = query
				data.Url = link
				data.Desc = card.desc
				chanDataForDb <- data
				fmt.Println("[PayoutsKing] Results found: ", data.Key, data.Url)
			}
		}
	}
}
