package website

import (
	"compress/gzip"
	"crypto/tls"
	"darkwebscraper/utils"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

const noirthOnion = "https://noirth.com/"

var noirthClient *http.Client
var bodyBytesNoirth []byte

func initNoirthClient() error {
	if noirthClient != nil {
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

	noirthClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func getNoirthCookies() ([]*http.Cookie, string, error) {
	username := os.Getenv("NOIRTHUSERNAME")
	password := os.Getenv("NOIRTHPASSWORD")

	if username == "" || password == "" {
		return nil, "", fmt.Errorf("NOIRTHUSERNAME or NOIRTHPASSWORD not set")
	}

	jar, _ := cookiejar.New(nil)

	client := &http.Client{
		Jar: jar,
	}
	// Fetch login page
	req, err := http.NewRequest("GET", "https://noirth.com/login/", nil)
	if err != nil {
		return nil, "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	// Extract _xfToken
	re := regexp.MustCompile(`name="_xfToken"\s+value="([^"]+)"`)
	m := re.FindStringSubmatch(string(body))
	if len(m) != 2 {
		return nil, "", fmt.Errorf("could not find _xfToken")
	}
	token := m[1]
	// fmt.Println(token)
	// Prepare login request
	data := url.Values{}
	data.Set("_xfToken", token)
	data.Set("login", username)
	data.Set("password", password)
	data.Set("remember", "1")
	data.Set("_xfRedirect", "https://noirth.com/")

	req, err = http.NewRequest(
		"POST",
		"https://noirth.com/login/login",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return nil, "", err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err = client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	u, _ := url.Parse("https://noirth.com")
	cookies := jar.Cookies(u)
	return cookies, token, nil
}

func Noirth(channel chan string, chanDataForDb chan utils.DataForDb) {
	data := utils.DataForDb{}
	if err := initNoirthClient(); err != nil {
		panic(err)
	}
	cookies, token, _ := getNoirthCookies()
	// for _, c := range cookies {
	// 	fmt.Printf("%s=%s\n", c.Name, c.Value)
	// }
	for query := range channel {
		query = strings.TrimSpace(query)
		targetURL := noirthOnion + "/search/search"
		reqData := url.Values{}
		reqData.Set("keywords", query)
		reqData.Set("c[users]", "")
		reqData.Set("_xfToken", token)
		req, _ := http.NewRequest("POST", targetURL, strings.NewReader(reqData.Encode()))
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept-Encoding", "gzip")
		for _, c := range cookies {
			req.AddCookie(c)
		}
		resp, err := noirthClient.Do(req)
		if err != nil {
			fmt.Println("[Noirth] request failed:", err)
			time.Sleep(10 * time.Second)
			continue
		}

		if resp.Header.Get("Content-Encoding") == "gzip" {
			reader, err := gzip.NewReader(resp.Body)
			if err != nil {
				resp.Body.Close()
				continue
			}
			bodyBytesNoirth, err = io.ReadAll(reader)
			reader.Close()
			if err != nil {
				resp.Body.Close()
				continue
			}
		} else {
			bodyBytesNoirth, err = io.ReadAll(resp.Body)
			if err != nil {
				resp.Body.Close()
				continue
			}
		}
		resp.Body.Close()

		body := string(bodyBytesNoirth)
		// fmt.Println(body)
		switch {
		case strings.Contains(body, "No results"):
			fmt.Println("[Noirth] no results")

		case strings.Contains(body, "Search results for query"):
			fmt.Println("[Noirth] Found result")

			links := utils.ExtractPostLinks(body, "/threads/")
			for _, link := range links {
				fmt.Println("Post link:", noirthOnion+link)
				data.Source = "noirth"
				data.Key = query
				data.Url = noirthOnion + link
				data.Desc = "lorem ipsum"
				chanDataForDb <- data
			}
		default:
			fmt.Println("[Noirth] some error occured")
			// case strings.Contains(body, "queue"):
			// 	fmt.Println("[Noirth] still in queue, retrying…")
			// 	continue
			// }

		}

		time.Sleep(time.Duration(8+rand.Intn(5)) * time.Second)
	}
}
