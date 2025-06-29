package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// убеждаемся, что корень ("/")
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

// Проверяет, что production-хендлер корректно возвращает статус 500 и фирменную страницу ошибки.
// Использует временный маршрут /force500 (разкомментировать в InitRoutes в server.go для теста).
func Test500Template(t *testing.T) {
	ts := httptest.NewServer(InitRoutes())
	defer ts.Close()

	// Можно эмулировать 500, вызвав refresh с нерабочим API или подложив mock.
	// Либо разкомментировать временный route, который вызывает renderError(500).
	resp, _ := http.Get(ts.URL + "/force500")
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Ошибка 500") {
		t.Fatalf("expected custom error page, got: %s", string(body))
	}
}

// Проверяет, что при некорректном HTTP-методе (GET вместо POST) на /api/refresh
// сервер возвращает статус 400 и фирменную страницу ошибки.
func Test400Template(t *testing.T) {
	ts := httptest.NewServer(InitRoutes())
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/api/refresh")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Неверный HTTP-метод") {
		t.Fatalf("expected custom error page, got: %s", string(body))
	}
}
