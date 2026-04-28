package website

import (
	"compress/gzip"
	"crypto/tls"
	"darkwebscraper/utils"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/proxy"
)

// type dataExposureLogsResponse struct {
// 	Posts []utils.CompanydataExposureLogs `json:"posts"`
// }

const dataExposureLogsOnion = "http://6tdqqaxftvradka5d2frzgwixis7fmro7rfh4ettzcx7jfapkebe6jad.onion/"

var dataExposureLogsClient *http.Client
var bodyBytesDataExposureLogs []byte

func initDataExposureLogsClient() error {
	if dataExposureLogsClient != nil {
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

	dataExposureLogsClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func hasClass(n *html.Node, class string) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" && strings.Contains(attr.Val, class) {
			return true
		}
	}
	return false
}

func DataExposureLogs(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}

	if err := initDataExposureLogsClient(); err != nil {
		fmt.Println("[dataExposureLogs] init failed:", err)
	}

	req, _ := http.NewRequest("GET", dataExposureLogsOnion, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := dataExposureLogsClient.Do(req)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
		bodyBytesDataExposureLogs, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			resp.Body.Close()
		}
	} else {
		bodyBytesDataExposureLogs, err = io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
		}
	}

	// err = json.Unmarshal(bodyBytesdataExposureLogs, &response)
	// if err != nil {
	// 	fmt.Println("[dataExposureLogs] JSON parse error:", err)
	// }

	doc, err := html.Parse(strings.NewReader(string(bodyBytesDataExposureLogs)))
	if err != nil {
		panic(err)
	}

	var company string
	var link string
	var companies []string
	var links []string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// extract company name
			if n.Data == "div" && hasClass(n, "title") && n.FirstChild != nil {
				company = strings.TrimSpace(n.FirstChild.Data)
				// fmt.Println(company)
				companies = append(companies, company)

			}

			// extract link from onclick
			if n.Data == "div" && hasClass(n, "card") {
				for _, attr := range n.Attr {
					if attr.Key == "onclick" {
						re := regexp.MustCompile(`window\.open\('([^']+)'`)
						m := re.FindStringSubmatch(attr.Val)
						if len(m) > 1 {
							link = m[1]
							// fmt.Println(link)
							links = append(links, link)
						}
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
			// fmt.Println(company)
			// fmt.Println("Link:", link)
			// fmt.Println(c)
		}
	}
	f(doc)

	// fmt.Println("Company:", company)
	// fmt.Println("Link:", link)

	for query := range channel {
		query = strings.TrimSpace(query)
		for i, c := range companies {
			// fmt.Println(c)
			if strings.Contains(c, query) {
				url := dataExposureLogsOnion + links[i]
				data.Source = "dataExposureLogs"
				data.Key = query
				data.Url = url
				data.Desc = "Lorem Ipsum"
				chanDataForDb <- data
				// fmt.Println(data.Key, data.Url)
				fmt.Println("[dataExposureLogs] Results found: ", data.Key, data.Url)
			}
		}
	}
}
