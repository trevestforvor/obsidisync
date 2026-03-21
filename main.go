package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"golang.org/x/net/webdav"
)

func buildMux(vaultRoot, token string) *http.ServeMux {
	davHandler := &webdav.Handler{
		FileSystem: webdav.Dir(vaultRoot),
		LockSystem: webdav.NewMemLS(),
		Prefix:     "/dav",
	}

	mux := http.NewServeMux()
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

	log.Printf("obsidisync starting on %s (vault_root=%s)", listenAddr, vaultRoot)
	if err := http.ListenAndServe(listenAddr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
