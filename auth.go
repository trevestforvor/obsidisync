package main

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

type AuthMiddleware struct {
	token   string
	handler http.Handler
}

func NewAuthMiddleware(token string, handler http.Handler) *AuthMiddleware {
	return &AuthMiddleware{token: token, handler: handler}
}

func (a *AuthMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	provided := strings.TrimPrefix(authHeader, "Bearer ")

	if subtle.ConstantTimeCompare([]byte(provided), []byte(a.token)) != 1 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	a.handler.ServeHTTP(w, r)
}
