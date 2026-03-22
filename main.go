package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/net/webdav"
)

func buildMux(vaultRoot, token string) *http.ServeMux {
	davHandler := &webdav.Handler{
		FileSystem: webdav.Dir(vaultRoot),
		LockSystem: webdav.NewMemLS(),
		Prefix:     "/dav",
	}

	mux := http.NewServeMux()
	mux.Handle("/", NewIndexHandler())
	mux.Handle("/api/health", NewHealthHandler(vaultRoot))
	mux.Handle("/dav/", NewAuthMiddleware(token, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "..") {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		davHandler.ServeHTTP(w, r)
	})))

	return mux
}

func main() {
	vaultRoot := os.Getenv("VAULT_ROOT")
	if vaultRoot == "" {
		vaultRoot = "/srv/vault"
	}

	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8080"
	}

	token := os.Getenv("VAULT_TOKEN")
	if token == "" {
		log.Fatal("VAULT_TOKEN environment variable is required")
	}

	mux := buildMux(vaultRoot, token)

	srv := &http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}

	// Graceful shutdown on SIGTERM/SIGINT
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("obsidisync starting on %s (vault_root=%s)", listenAddr, vaultRoot)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-done
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}
	log.Println("stopped")
}
