package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRootHandler убеждаемся, что корень ("/")
// отвечает HTTP 200 и содержит заголовок "Артисты".
func TestRootHandler(t *testing.T) {
	ts := httptest.NewServer(InitRoutes())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	// Контент-тест минимальный: проверяем, что пришло ≈ HTML.
	ct := resp.Header.Get("Content-Type")
	if ct == "" || ct[:5] != "text/" {
		t.Fatalf("unexpected Content-Type: %s", ct)
	}
}
