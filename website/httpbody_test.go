package website

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"testing"
)

func TestReadResponseBodyPlain(t *testing.T) {
	body := []byte("<html>plain</html>")
	resp := &http.Response{
		Header: http.Header{},
		Body:   io.NopCloser(bytes.NewReader(body)),
	}

	got, err := readResponseBody(resp)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("expected %q, got %q", string(body), string(got))
	}
}

func TestReadResponseBodyGzip(t *testing.T) {
	plainBody := []byte("<html>gzip</html>")
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(plainBody); err != nil {
		t.Fatalf("gzip write failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("gzip close failed: %v", err)
	}

	resp := &http.Response{
		Header: http.Header{"Content-Encoding": []string{"gzip"}},
		Body:   io.NopCloser(bytes.NewReader(compressed.Bytes())),
	}

	got, err := readResponseBody(resp)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !bytes.Equal(got, plainBody) {
		t.Fatalf("expected %q, got %q", string(plainBody), string(got))
	}
}

func TestReadResponseBodyInvalidGzipFallsBackToRawBody(t *testing.T) {
	rawBody := []byte("<html>not-actually-gzip</html>")
	resp := &http.Response{
		Header: http.Header{"Content-Encoding": []string{"gzip"}},
		Body:   io.NopCloser(bytes.NewReader(rawBody)),
	}

	got, err := readResponseBody(resp)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !bytes.Equal(got, rawBody) {
		t.Fatalf("expected %q, got %q", string(rawBody), string(got))
	}
}
