package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	vaultRoot := os.Getenv("VAULT_ROOT")
	if vaultRoot == "" {
		vaultRoot = "/srv/vault"
	}

	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8080"
	}

	mux := http.NewServeMux()
	mux.Handle("/api/health", NewHealthHandler(vaultRoot))

	log.Printf("obsidisync starting on %s (vault_root=%s)", listenAddr, vaultRoot)
	if err := http.ListenAndServe(listenAddr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
