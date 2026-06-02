package website

import (
	"crypto/tls"
	"darkwebscraper/utils"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

const atomsiloOnion = "http://npmh5ahrgakbniuntyc7io4adm6ietbdbuejrfonowqtyqn24or556qd.onion/"

var atomsiloClient *http.Client

func initAtomsiloClient() error {
	if atomsiloClient != nil {
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

	atomsiloClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}

func Atomsilo(channel chan string, chanDataForDb chan utils.DataForDb) {
	if err := initAtomsiloClient(); err != nil {
		fmt.Println("[Atomsilo] init failed:", err)
	}

	req, err := http.NewRequest("GET", atomsiloOnion+"javascript/data.js", nil)
	if err != nil {
		fmt.Println("[Atomsilo] request creation failed:", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "application/javascript,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := atomsiloClient.Do(req)
	if err != nil {
		fmt.Println("[Atomsilo] request failed:", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("[Atomsilo] read body failed:", err)
	}

	bodyStr := string(bodyBytes)
	start := strings.Index(bodyStr, "const companies = ")
	if start == -1 {
		fmt.Println("[Atomsilo] companies object not found")
		return
	}
	jsonStart := start + len("const companies = ")
	jsonEnd := strings.LastIndex(bodyStr, "};")
	if jsonEnd == -1 {
		fmt.Println("[Atomsilo] companies object end not found")
		return
	}
	jsObj := bodyStr[jsonStart : jsonEnd+1]

	// Convert JS object to valid JSON
	jsonStr := jsObj
	jsonStr = strings.ReplaceAll(jsonStr, "\r", "")
	jsonStr = strings.TrimSpace(jsonStr)
	// Replace backticks with double quotes for multiline strings
	jsonStr = strings.ReplaceAll(jsonStr, "`", "\"")
	jsonStr = strings.ReplaceAll(jsonStr, "\n", " ")
	// Quote only top-level keys (company names) at the start of a line, not nested keys
	jsonStr = regexp.MustCompile(`(?m)^\s*([a-zA-Z0-9_]+):`).ReplaceAllString(jsonStr, "\"$1\":")
	// Remove trailing commas before closing braces/brackets
	jsonStr = regexp.MustCompile(`,\s*([}\]])`).ReplaceAllString(jsonStr, "$1")
	// Remove any trailing comma before the final closing brace
	jsonStr = regexp.MustCompile(`,\s*}$`).ReplaceAllString(jsonStr, "}")
	jsonStr = strings.TrimSpace(jsonStr)
	// Ensure the object is wrapped in braces for valid JSON
	if !strings.HasPrefix(jsonStr, "{") {
		jsonStr = "{" + jsonStr
	}
	if !strings.HasSuffix(jsonStr, "}") {
		jsonStr = jsonStr + "}"
	}

	var companies map[string]struct {
		Name        string   `json:"name"`
		Domain      string   `json:"domain"`
		Size        string   `json:"size"`
		Logo        string   `json:"logo"`
		Description string   `json:"description"`
		Data        []string `json:"data"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &companies); err != nil {
		fmt.Println("[Atomsilo] JSON unmarshal failed:", err)
		return
	}

	for query := range channel {
		query = strings.TrimSpace(query)
		for _, c := range companies {
			if strings.Contains(strings.ToLower(c.Name), strings.ToLower(query)) || strings.Contains(strings.ToLower(c.Description), strings.ToLower(query)) {
				data := utils.DataForDb{
					Source: "atomsilo",
					Key:    query,
					Url:    "lorem ipsum",
					Desc:   c.Description,
				}
				chanDataForDb <- data
				fmt.Println("[Atomsilo] Match found:", c.Name)
			}
		}
	}
}
