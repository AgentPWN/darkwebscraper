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

// type beastResponse struct {
// 	Posts []utils.Companybeast `json:"posts"`
// }

const beastOnion = "http://beast6azu4f7fxjakiayhnssybibsgjnmy77a6duufqw5afjzfjhzuqd.onion/"

var beastClient *http.Client
var bodyBytesBeast []byte

func initBeastClient() error {
	if beastClient != nil {
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

	beastClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func hasClassBeast(n *html.Node, class string) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" && strings.Contains(attr.Val, class) {
			return true
		}
	}
	return false
}

func Beast(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}

	if err := initBeastClient(); err != nil {
		fmt.Println("[Beast] init failed:", err)
	}

	req, _ := http.NewRequest("GET", beastOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := beastClient.Do(req)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesBeast, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesBeast, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	// err = json.Unmarshal(bodyBytesbeast, &response)
	// if err != nil {
	// 	fmt.Println("[beast] JSON parse error:", err)
	// }

	doc, err := html.Parse(strings.NewReader(string(bodyBytesBeast)))
	if err != nil {
		panic(err)
	}

	type cardEntry struct {
		company string
		link    string
		desc    string
	}
	var cards []cardEntry

	var f func(*html.Node)
	f = func(n *html.Node) {
		// Each card is an <a> tag with class "card"
		if n.Type == html.ElementNode && n.Data == "a" && hasClassBeast(n, "card") {
			entry := cardEntry{}

			// Extract href (relative link)
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					entry.link = strings.TrimSpace(attr.Val)
				}
			}

			// Walk children to find <h3> (company name) and first <p> (description)
			var walk func(*html.Node)
			walk = func(c *html.Node) {
				if c.Type == html.ElementNode {
					if c.Data == "h3" && c.FirstChild != nil {
						entry.company = strings.TrimSpace(c.FirstChild.Data)
					}
					// First <p> inside .card-text holds the description
					if c.Data == "p" && entry.desc == "" && c.FirstChild != nil {
						entry.desc = strings.TrimSpace(c.FirstChild.Data)
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
			return // don't recurse into the card itself again
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	for query := range channel {
		query = strings.TrimSpace(query)
		for _, card := range cards {
			if strings.Contains(card.company, query) {
				url := beastOnion + card.link
				data.Source = "beast"
				data.Key = query
				data.Url = url
				data.Desc = card.desc
				chanDataForDb <- data
				fmt.Println("[Beast] Results found: ", data.Key, data.Url)
			}
		}
	}
}
