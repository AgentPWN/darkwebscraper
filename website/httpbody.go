package website

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func readResponseBody(resp *http.Response) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, fmt.Errorf("empty response body")
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Encoding")), "gzip") {
		return rawBody, nil
	}

	reader, err := gzip.NewReader(bytes.NewReader(rawBody))
	if err != nil {
		return rawBody, nil
	}
	defer reader.Close()

	decodedBody, err := io.ReadAll(reader)
	if err != nil {
		return rawBody, nil
	}

	return decodedBody, nil
}
