package main

import (
	"crypto/tls"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

// Checks for common captcha indicators in the response body
func containsCaptcha(body string) bool {
	captchaIndicators := []string{
		"captcha", "are you human", "verify you are", "not a robot", "recaptcha", "hcaptcha",
	}
	bodyLower := strings.ToLower(body)
	for _, indicator := range captchaIndicators {
		if strings.Contains(bodyLower, indicator) {
			return true
		}
	}
	return false
}

// Creates an HTTP client using Tor SOCKS5 proxy
func newTorClient() (*http.Client, error) {
	torDialer, err := proxy.SOCKS5("tcp", "localhost:9050", nil, nil)
	if err != nil {
		return nil, err
	}
	transport := &http.Transport{
		DialContext:     torDialer.(proxy.ContextDialer).DialContext,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}, nil
}

func main() {
	// Load darkweb.csv URLs into a set
	darkwebUrls := make(map[string]struct{})
	darkwebFile, err := os.Open("darkweb.csv")
	if err == nil {
		darkwebReader := csv.NewReader(darkwebFile)
		darkwebHeaders, err := darkwebReader.Read()
		urlCol := -1
		for i, h := range darkwebHeaders {
			if strings.EqualFold(h, "url") || strings.EqualFold(h, "urls") {
				urlCol = i
				break
			}
		}
		if err == nil && urlCol != -1 {
			for {
				row, err := darkwebReader.Read()
				if err == io.EOF {
					break
				}
				if err != nil || len(row) <= urlCol {
					continue
				}
				u := strings.TrimSpace(row[urlCol])
				if u != "" {
					darkwebUrls[u] = struct{}{}
				}
			}
		}
		darkwebFile.Close()
	}

	// Load already-checked URLs from urls_checked.csv into a set
	checkedUrls := make(map[string]struct{})
	checkedExists := false
	if f, err := os.Open("urls_checked.csv"); err == nil {
		checkedExists = true
		r := csv.NewReader(f)
		r.FieldsPerRecord = -1
		hdrs, err := r.Read()
		urlCol := -1
		if err == nil {
			for i, h := range hdrs {
				if strings.EqualFold(h, "url") || strings.EqualFold(h, "urls") {
					urlCol = i
					break
				}
			}
		}
		if err == nil && urlCol != -1 {
			for {
				row, err := r.Read()
				if err == io.EOF {
					break
				}
				if err != nil || len(row) <= urlCol {
					continue
				}
				u := strings.TrimSpace(row[urlCol])
				if u != "" {
					checkedUrls[u] = struct{}{}
				}
			}
		}
		f.Close()
	}

	inFile, err := os.Open("urls.csv")
	if err != nil {
		panic(err)
	}
	defer inFile.Close()

	reader := csv.NewReader(inFile)
	reader.FieldsPerRecord = -1
	headers, err := reader.Read()
	if err != nil {
		panic(err)
	}

	// Find column indices
	typeIdx, urlIdx := -1, -1
	for i, h := range headers {
		if strings.EqualFold(h, "Type") {
			typeIdx = i
		}
		if strings.EqualFold(h, "URL") {
			urlIdx = i
		}
	}
	if typeIdx == -1 || urlIdx == -1 {
		panic("Type or URL column not found")
	}

	// Open urls_checked.csv for append without truncating if it already exists.
	var outFile *os.File
	if checkedExists {
		if err := os.Chmod("urls_checked.csv", 0644); err != nil {
			fmt.Println("Warning: could not chmod urls_checked.csv:", err)
		}
		outFile, err = os.OpenFile("urls_checked.csv", os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
	} else {
		outFile, err = os.Create("urls_checked.csv")
		if err != nil {
			panic(err)
		}
		// Write new headers only when creating the file
		writerTmp := csv.NewWriter(outFile)
		newHeaders := append(headers, "HTTPStatus", "CaptchaDetected")
		writerTmp.Write(newHeaders)
		writerTmp.Flush()
	}
	defer outFile.Close()
	writer := csv.NewWriter(outFile)
	defer writer.Flush()

	torClient, err := newTorClient()
	if err != nil {
		panic(err)
	}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println("Skipping row due to error:", err)
			continue
		}
		if len(record) <= urlIdx || len(record) <= typeIdx {
			continue
		}
		if strings.ToLower(record[typeIdx]) != "group" {
			continue
		}
		url := record[urlIdx]
		urlLower := strings.ToLower(strings.TrimSpace(url))
		skip := false

		// Check against darkweb.csv URLs
		for dbUrl := range darkwebUrls {
			dbUrlLower := strings.ToLower(strings.TrimSpace(dbUrl))
			if dbUrlLower != "" && (strings.Contains(urlLower, dbUrlLower) || strings.Contains(dbUrlLower, urlLower)) {
				skip = true
				break
			}
			// Check http/https variant
			if strings.HasPrefix(urlLower, "http://") && strings.HasPrefix(dbUrlLower, "https://") {
				if strings.Contains("https://"+urlLower[7:], dbUrlLower) || strings.Contains(dbUrlLower, "https://"+urlLower[7:]) {
					skip = true
					break
				}
			}
			if strings.HasPrefix(urlLower, "https://") && strings.HasPrefix(dbUrlLower, "http://") {
				if strings.Contains("http://"+urlLower[8:], dbUrlLower) || strings.Contains(dbUrlLower, "http://"+urlLower[8:]) {
					skip = true
					break
				}
			}
		}

		// Check against already-checked URLs
		if !skip {
			for chUrl := range checkedUrls {
				chUrlLower := strings.ToLower(strings.TrimSpace(chUrl))
				if chUrlLower != "" && (strings.Contains(urlLower, chUrlLower) || strings.Contains(chUrlLower, urlLower)) {
					skip = true
					break
				}
			}
		}

		if skip {
			continue
		}

		status := ""
		captcha := ""
		var bodyContent string
		var htmlStr string

		resp, err := torClient.Get(url)
		if err != nil {
			status = "error"
			captcha = ""
			bodyContent = "[Request error: " + err.Error() + "]"
		} else {
			status = fmt.Sprintf("%d", resp.StatusCode)
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			htmlStr = string(bodyBytes)
			// Extract <body>...</body> content
			bodyStart := strings.Index(strings.ToLower(htmlStr), "<body")
			if bodyStart != -1 {
				gtIdx := strings.Index(htmlStr[bodyStart:], ">")
				if gtIdx != -1 {
					bodyStart += gtIdx + 1
					bodyEnd := strings.Index(strings.ToLower(htmlStr[bodyStart:]), "</body>")
					if bodyEnd != -1 {
						bodyContent = htmlStr[bodyStart : bodyStart+bodyEnd]
					} else {
						bodyContent = htmlStr[bodyStart:]
					}
				}
			}
			if bodyContent == "" {
				bodyContent = htmlStr
			}
			// Check for login page keywords
			loginIndicators := []string{"login", "sign in", "log in", "user: ", "password", "auth", "authentication"}
			bodyContentLower := strings.ToLower(bodyContent)
			isLogin := false
			for _, kw := range loginIndicators {
				if strings.Contains(bodyContentLower, kw) {
					isLogin = true
					break
				}
			}
			if isLogin {
				continue
			}
			if resp.StatusCode == 200 {
				if containsCaptcha(htmlStr) {
					captcha = "yes"
				} else {
					captcha = "no"
				}
			} else {
				captcha = ""
			}
		}

		// Print URL and first 10000 chars of body content, prompt for Enter
		fmt.Println("URL:", url)
		if len(bodyContent) > 10000 {
			fmt.Println(bodyContent[:10000])
		} else {
			fmt.Println(bodyContent)
		}
		fmt.Print("Press Enter to continue...")
		fmt.Scanln()

		newRecord := append(record, status, captcha)
		writer.Write(newRecord)
		writer.Flush()
		// Add to checkedUrls to avoid duplicates within same run
		checkedUrls[url] = struct{}{}

		fmt.Println("Checked:", url, "Status:", status, "Captcha:", captcha)
	}
}
