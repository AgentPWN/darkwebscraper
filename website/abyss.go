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

const abyssOnion = "http://3ev4metjirohtdpshsqlkrqcmxq6zu3d7obrdhglpy5jpbr7whmlfgqd.onion"

var abyssClient *http.Client

func initAbyssClient() error {
	if abyssClient != nil {
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

	abyssClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	return nil
}
func collapseJSStrings(raw string) string {
	// Collapse JS string concatenation: '"..." + "..."' -> '"..."'
	re := regexp.MustCompile(`"\s*\+\s*"`)
	raw = re.ReplaceAllString(raw, "")

	// Remove HTML tags like <br>, <b>, etc.
	htmlTags := regexp.MustCompile(`<[^>]+>`)
	raw = htmlTags.ReplaceAllString(raw, " ")

	// Replace literal newlines and tabs inside what will become JSON string values
	// These are invalid in JSON strings and must be escaped or removed
	raw = strings.ReplaceAll(raw, "\r\n", " ")
	raw = strings.ReplaceAll(raw, "\n", " ")
	raw = strings.ReplaceAll(raw, "\r", " ")
	raw = strings.ReplaceAll(raw, "\t", " ")

	// Collapse multiple spaces into one
	spaces := regexp.MustCompile(`\s{2,}`)
	raw = spaces.ReplaceAllString(raw, " ")

	return raw
}
func parseAbyssJS(raw string) ([]utils.CompanyAbyss, error) {
	raw = strings.TrimSpace(raw)

	// Strip the JS variable assignment
	raw = strings.TrimPrefix(raw, "let data =")
	raw = strings.TrimSpace(raw)
	raw = strings.TrimSuffix(raw, ";")

	// Replace single quotes with double quotes ONLY outside of already double-quoted strings
	// Simple approach: replace all single quotes first
	raw = strings.ReplaceAll(raw, "\\'", "__ESCAPED_QUOTE__")
	raw = strings.ReplaceAll(raw, "'", "\"")
	raw = strings.ReplaceAll(raw, "__ESCAPED_QUOTE__", "\\'")

	// Collapse JS string concatenation and strip HTML
	raw = collapseJSStrings(raw)

	// Remove trailing commas before ] or } which are valid JS but not JSON
	trailingComma := regexp.MustCompile(`,\s*([}\]])`)
	raw = trailingComma.ReplaceAllString(raw, "$1")

	// Remove any JS comments
	jsComment := regexp.MustCompile(`//[^\n]*`)
	raw = jsComment.ReplaceAllString(raw, "")

	var entries []utils.CompanyAbyss
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		// Dump a snippet around the error for easier debugging
		if jsonErr, ok := err.(*json.SyntaxError); ok {
			offset := jsonErr.Offset
			start := max(0, int(offset)-50)
			end := min(len(raw), int(offset)+50)
			fmt.Printf("[Abyss] JSON error near: ...%s...\n", raw[start:end])
		}
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}
	return entries, nil
}

// splitFull splits the full field into:
//   - name: the part before the first " <br>" (the entity name line)
//   - desc: everything from the description onwards, including the password line
func splitFull(full string) (name, desc string) {
	// After HTML stripping, lines are space-separated
	// The name is the first sentence/segment, desc is the rest
	parts := strings.SplitN(full, " ", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return strings.TrimSpace(full), ""
}

func Abyss(channel chan string, chanDataForDb chan utils.DataForDb) {
	if err := initAbyssClient(); err != nil {
		fmt.Println("[Abyss] init failed:", err)
	}

	req, err := http.NewRequest("GET", abyssOnion+"/static/data.js", nil)
	if err != nil {
		fmt.Println("[Abyss] request creation failed:", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:102.0) Gecko/20100101 Firefox/102.0")
	req.Header.Set("Accept", "application/javascript,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := abyssClient.Do(req)
	if err != nil {
		fmt.Println("[Abyss] request failed:", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("[Abyss] read body failed:", err)
	}

	entries, err := parseAbyssJS(string(bodyBytes))
	if err != nil {
		fmt.Println("[Abyss] parse failed:", err)
	}

	// Filter entries that match the query
	for query := range channel {
		query = strings.ToLower(query)

		for _, entry := range entries {
			// Check if this entry is relevant to the client query
			if !strings.Contains(strings.ToLower(entry.Title), query) &&
				!strings.Contains(strings.ToLower(entry.Short), query) &&
				!strings.Contains(strings.ToLower(entry.Full), query) {
				continue
			}

			_, desc := splitFull(entry.Full)

			// Each link gets its own DB record
			for _, link := range entry.Links {
				data := utils.DataForDb{
					Source: "abyss",
					Key:    query,
					Url:    link,
					Desc:   desc,
				}
				fmt.Println("[Abyss] Match found:", entry.Title, "->", link)
				chanDataForDb <- data
			}
		}

	}
}
