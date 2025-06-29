package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// CSS раздаётся как статика
func TestCSSServed(t *testing.T) {
	ts := httptest.NewServer(InitRoutes())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/static/style.css")
	if err != nil {
		t.Fatal(err)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Fatalf("unexpected Content-Type %s", ct)
	}
}

// Проверяет, что для несуществующего маршрута сервер возвращает статус 404 и фирменную страницу ошибки.
func Test404Template(t *testing.T) {
	ts := httptest.NewServer(InitRoutes())
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/no/such/page")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Ошибка 404") {
		t.Fatalf("expected custom error page, got: %s", string(body))
	}
}
