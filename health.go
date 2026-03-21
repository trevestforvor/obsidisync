package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

type HealthHandler struct {
	vaultRoot string
}

func NewHealthHandler(vaultRoot string) *HealthHandler {
	return &HealthHandler{vaultRoot: vaultRoot}
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	writable := h.checkWritable()

	resp := map[string]interface{}{
		"vault_root": h.vaultRoot,
		"writable":   writable,
	}

	if writable {
		resp["status"] = "ok"
		w.WriteHeader(http.StatusOK)
	} else {
		resp["status"] = "error"
		resp["error"] = "mount not accessible"
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(resp)
}

func (h *HealthHandler) checkWritable() bool {
	info, err := os.Stat(h.vaultRoot)
	if err != nil || !info.IsDir() {
		return false
	}

	probe := filepath.Join(h.vaultRoot, fmt.Sprintf(".health-probe-%d", os.Getpid()))
	f, err := os.Create(probe)
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(probe)
	return true
}
