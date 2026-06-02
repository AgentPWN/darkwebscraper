package website

import (
	"compress/gzip"
	"crypto/tls"
	"darkwebscraper/utils"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/proxy"
)

const sarcomaOnion = "http://sarcomawmawlhov7o5mdhz4eszxxlkyaoiyiy2b5iwxnds2dmb4jakad.onion/"

var sarcomaClient *http.Client

func initSarcomaClient() error {
	if sarcomaClient != nil {
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

	sarcomaClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}

	return nil
}

func Sarcoma(channel chan string, chanDataForDb chan utils.DataForDb) {
	if err := initSarcomaClient(); err != nil {
		fmt.Println("[Sarcoma] init failed:", err)
		return
	}

	req, err := http.NewRequest("GET", sarcomaOnion, nil)
	if err != nil {
		fmt.Println("[Sarcoma] request build failed:", err)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := sarcomaClient.Do(req)
	if err != nil {
		fmt.Println("[Sarcoma] request failed:", err)
		return
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			fmt.Println("[Sarcoma] gzip reader failed:", err)
			return
		}
		bodyBytes, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			fmt.Println("[Sarcoma] read gzip body failed:", err)
			return
		}
	} else {
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("[Sarcoma] read body failed:", err)
			return
		}
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytes)))
	if err != nil {
		fmt.Println("[Sarcoma] HTML parse failed:", err)
		return
	}

	type cardEntry struct {
		company  string
		site     string
		industry string
		geo      string
		views    string
		logo     string
		moreID   string
	}
	var cards []cardEntry

	var innerText func(*html.Node) string
	innerText = func(n *html.Node) string {
		if n.Type == html.TextNode {
			return n.Data
		}
		var sb strings.Builder
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			sb.WriteString(innerText(c))
		}
		return sb.String()
	}

	normalizeURL := func(v string) string {
		v = strings.TrimSpace(v)
		if v == "" {
			return ""
		}
		if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
			return v
		}
		if strings.HasPrefix(v, "//") {
			return "http:" + v
		}
		if strings.HasPrefix(v, "/") {
			return sarcomaOnion + strings.TrimPrefix(v, "/")
		}
		if strings.HasPrefix(v, "www.") {
			return "https://" + v
		}
		return sarcomaOnion + v
	}

	parseCardText := func(text string, entry *cardEntry) {
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			lower := strings.ToLower(line)
			switch {
			case strings.HasPrefix(lower, "site:"):
				entry.site = normalizeURL(strings.TrimSpace(line[len("Site:"):]))
			case strings.HasPrefix(lower, "industry:"):
				entry.industry = strings.TrimSpace(line[len("Industry:"):])
			case strings.HasPrefix(lower, "geo:"):
				entry.geo = strings.TrimSpace(line[len("GEO:"):])
			}
		}
	}

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "sg-form") && hasClass(n, "card") {
			entry := cardEntry{}

			var walk func(*html.Node)
			walk = func(c *html.Node) {
				if c.Type == html.ElementNode {
					if c.Data == "div" && hasClass(c, "card-title") {
						titleText := strings.TrimSpace(innerText(c))
						lines := strings.Split(titleText, "\n")
						for _, line := range lines {
							line = strings.TrimSpace(line)
							if line == "" {
								continue
							}
							if entry.views == "" {
								onlyDigits := true
								for _, r := range line {
									if r < '0' || r > '9' {
										onlyDigits = false
										break
									}
								}
								if onlyDigits {
									entry.views = line
									continue
								}
							}
							if entry.company == "" {
								entry.company = line
							}
						}
					}

					if c.Data == "div" && hasClass(c, "card-text") {
						parseCardText(innerText(c), &entry)
					}

					if c.Data == "img" {
						for _, attr := range c.Attr {
							if attr.Key == "src" {
								entry.logo = normalizeURL(attr.Val)
							}
						}
					}

					if c.Data == "button" && hasClass(c, "company_button") {
						for _, attr := range c.Attr {
							if attr.Key == "data-company" {
								entry.moreID = strings.TrimSpace(attr.Val)
							}
						}
					}
				}

				for child := c.FirstChild; child != nil; child = child.NextSibling {
					walk(child)
				}
			}
			walk(n)

			if entry.company != "" {
				cards = append(cards, entry)
			}
			return
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	for query := range channel {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}
		needle := strings.ToLower(query)
		for _, card := range cards {
			haystack := strings.ToLower(card.company + " " + card.site + " " + card.industry + " " + card.geo)
			if !strings.Contains(haystack, needle) {
				continue
			}

			url := card.site
			if url == "" {
				url = sarcomaOnion
			}

			descParts := []string{}
			if card.industry != "" {
				descParts = append(descParts, "industry: "+card.industry)
			}
			if card.geo != "" {
				descParts = append(descParts, "geo: "+card.geo)
			}
			if card.views != "" {
				descParts = append(descParts, "views: "+card.views)
			}
			if card.logo != "" {
				descParts = append(descParts, "logo: "+card.logo)
			}
			if card.moreID != "" {
				descParts = append(descParts, "company_id: "+card.moreID)
			}

			chanDataForDb <- utils.DataForDb{
				Source: "sarcoma",
				Key:    query,
				Url:    url,
				Desc:   strings.Join(descParts, " | "),
			}
			fmt.Println("[Sarcoma] Results found:", card.company, url)
		}
	}
}
