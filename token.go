package main

import (
	"encoding/json"
	"net/http"
)

type TokenHandler struct {
	token string
}

func NewTokenHandler(token string) *TokenHandler {
	return &TokenHandler{token: token}
}

func (h *TokenHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": h.token})
}
