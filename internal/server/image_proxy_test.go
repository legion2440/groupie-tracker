package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestServeImgProxySuccess(t *testing.T) {
	const imageBody = "fake-jpeg-data"

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/queen.jpeg" {
			t.Errorf("unexpected upstream path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, imageBody)
	}))
	defer upstream.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/img/queen.jpeg", nil)

	serveImgProxy(rec, req, upstream.Client(), upstream.URL+"/")

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if contentType := res.Header.Get("Content-Type"); contentType != "image/jpeg" {
		t.Fatalf("expected image/jpeg Content-Type, got %q", contentType)
	}
	if cacheControl := res.Header.Get("Cache-Control"); cacheControl != "public, max-age=604800" {
		t.Fatalf("unexpected Cache-Control: %q", cacheControl)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read proxy response: %v", err)
	}
	if string(body) != imageBody {
		t.Fatalf("expected image body %q, got %q", imageBody, body)
	}
}

func TestServeImgProxyUpstreamError(t *testing.T) {
	for _, upstreamStatus := range []int{http.StatusNotFound, http.StatusInternalServerError} {
		t.Run(http.StatusText(upstreamStatus), func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, http.StatusText(upstreamStatus), upstreamStatus)
			}))
			defer upstream.Close()

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/img/missing.jpeg", nil)

			serveImgProxy(rec, req, upstream.Client(), upstream.URL+"/")

			res := rec.Result()
			defer res.Body.Close()

			if res.StatusCode != http.StatusNotFound {
				t.Fatalf("expected branded 404, got %d", res.StatusCode)
			}
			if contentType := res.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "text/html") {
				t.Fatalf("expected HTML error page, got Content-Type %q", contentType)
			}
			body, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatalf("read error response: %v", err)
			}
			if !strings.Contains(string(body), "groupie_tracker_404_neon.gif") {
				t.Fatalf("expected branded 404 page, got %q", body)
			}
		})
	}
}

func TestServeImgProxyTimeout(t *testing.T) {
	requestStarted := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-r.Context().Done()
	}))
	defer upstream.Close()

	client := upstream.Client()
	client.Timeout = 50 * time.Millisecond

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/img/slow.jpeg", nil)
	done := make(chan struct{})
	go func() {
		serveImgProxy(rec, req, client, upstream.URL+"/")
		close(done)
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("upstream request did not start")
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("image proxy did not return after client timeout")
	}

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected branded 404 after timeout, got %d", rec.Code)
	}
}

func TestServeImgProxyRequestCancellation(t *testing.T) {
	requestStarted := make(chan struct{})
	upstreamCanceled := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-r.Context().Done()
		close(upstreamCanceled)
	}))
	defer upstream.Close()

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/img/canceled.jpeg", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		serveImgProxy(rec, req, upstream.Client(), upstream.URL+"/")
		close(done)
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("upstream request did not start")
	}
	cancel()

	select {
	case <-upstreamCanceled:
	case <-time.After(time.Second):
		t.Fatal("input request cancellation did not reach upstream")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("image proxy did not return after request cancellation")
	}

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected branded 404 after cancellation, got %d", rec.Code)
	}
}

func TestServeImgProxyClosesUpstreamBody(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusNotFound} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			body := &trackingReadCloser{Reader: strings.NewReader("upstream body")}
			client := &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: status,
						Header:     make(http.Header),
						Body:       body,
						Request:    req,
					}, nil
				}),
			}

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/img/body.jpeg", nil)

			serveImgProxy(rec, req, client, "http://image-upstream.test/")

			if !body.closed {
				t.Fatalf("upstream body was not closed for status %d", status)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type trackingReadCloser struct {
	io.Reader
	closed bool
}

func (body *trackingReadCloser) Close() error {
	body.closed = true
	return nil
}
