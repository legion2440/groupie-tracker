package server

import (
	"context"
	"log"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"groupie-tracker/internal/core"
)

// Run запускает HTTP-сервер с кеш-воркером и graceful-shutdown.
func Run(addr string, ttl time.Duration) error {
	mux := InitRoutes()

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// кеш
	cacheCtx, cancelCache := context.WithCancel(context.Background())
	core.StartCache(cacheCtx, ttl)

	// сервер в горутине
	go func() {
		log.Printf("server started at http://localhost%s\n", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http error: %v", err)
		}
	}()

	// ловим SIGINT/SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down…")

	cancelCache() // останавливаем кеш-воркер

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

func init() {
	mime.AddExtensionType(".css", "text/css")
}
