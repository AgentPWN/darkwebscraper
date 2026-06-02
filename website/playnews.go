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

const playNewsOnion = "http://j75o7xvvsm4lpsjhkjvb4wl2q6ajegvabe6oswthuaubbykk4xkzgpid.onion/"

var playNewsClient *http.Client

func initPlayNewsClient() error {
	if playNewsClient != nil {
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

	playNewsClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func PlayNews(channel chan string, chanDataForDb chan utils.DataForDb) {
	if err := initPlayNewsClient(); err != nil {
		fmt.Println("[PlayNews] init failed:", err)
		return
	}

	req, _ := http.NewRequest("GET", playNewsOnion+"index.php", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := playNewsClient.Do(req)
	if err != nil {
		fmt.Println("[PlayNews] request failed:", err)
		return
	}
	defer resp.Body.Close()

	var bodyBytes []byte
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			fmt.Println("[PlayNews] gzip reader failed:", err)
			return
		}
		bodyBytes, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			fmt.Println("[PlayNews] gzip read failed:", err)
			return
		}
	} else {
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("[PlayNews] read body failed:", err)
			return
		}
	}

	doc, err := html.Parse(strings.NewReader(string(bodyBytes)))
	if err != nil {
		fmt.Println("[PlayNews] HTML parse failed:", err)
		return
	}

	type cardEntry struct {
		company string
		topicID string
		desc    string
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

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "th" && hasClassPlayNews(n, "News") {
			entry := cardEntry{}
			for _, attr := range n.Attr {
				if attr.Key == "onclick" {
					entry.topicID = extractPlayNewsTopicID(attr.Val)
				}
			}

			for child := n.FirstChild; child != nil; child = child.NextSibling {
				if child.Type == html.TextNode {
					text := strings.TrimSpace(child.Data)
					if text != "" && entry.company == "" {
						entry.company = text
					}
					continue
				}
				if child.Type == html.ElementNode && child.Data == "div" {
					text := strings.TrimSpace(innerText(child))
					if text != "" {
						if entry.desc != "" {
							entry.desc += "\n"
						}
						entry.desc += text
					}
				}
			}

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

		for _, card := range cards {
			if strings.Contains(strings.ToLower(card.company), strings.ToLower(query)) {
				topicID := strings.TrimSpace(card.topicID)
				if topicID == "" {
					continue
				}

				chanDataForDb <- utils.DataForDb{
					Source: "playNews",
					Key:    query,
					Url:    playNewsOnion + "topic.php?id=" + topicID,
					Desc:   card.desc,
				}
				fmt.Println("[PlayNews] Results found:", query, playNewsOnion+"topic.php?id="+topicID)
			}
		}
	}
}

func extractPlayNewsTopicID(onclick string) string {
	start := strings.Index(onclick, "viewtopic(")
	if start == -1 {
		return ""
	}

	segment := onclick[start+len("viewtopic("):]
	segment = strings.TrimLeft(segment, " \t\n\r\"")
	if segment == "" {
		return ""
	}

	end := strings.IndexAny(segment, "'\" )")
	if end == -1 {
		return strings.TrimSpace(segment)
	}

	return strings.TrimSpace(segment[:end])
}

func hasClassPlayNews(n *html.Node, class string) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" {
			for _, c := range strings.Fields(attr.Val) {
				if c == class {
					return true
				}
			}
		}
	}
	return false
}