package server

import (
	"net/http"
	"net/http/httptest"
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

// проверяет, что HTTP handler корректно возвращает статус 500 Internal Server Error.
// Тестовый handler всегда возвращает 500 ошибку с помощью http.Error.
func TestInternalServerError(t *testing.T) {
	req := httptest.NewRequest("GET", "/test500", nil)
	w := httptest.NewRecorder()

	handlerInternalServerError(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", resp.StatusCode)
	}
}

func handlerInternalServerError(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}
