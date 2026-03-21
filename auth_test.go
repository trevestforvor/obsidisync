package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddleware_ValidToken(t *testing.T) {
	token := "test-token-1234567890abcdef1234567890abcdef"
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	handler := NewAuthMiddleware(token, inner)
	req := httptest.NewRequest(http.MethodGet, "/dav/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "ok" {
		t.Errorf("expected body 'ok', got %q", w.Body.String())
	}
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	token := "test-token-1234567890abcdef1234567890abcdef"
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inner handler should not be called")
	})

	handler := NewAuthMiddleware(token, inner)
	req := httptest.NewRequest(http.MethodGet, "/dav/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	token := "test-token-1234567890abcdef1234567890abcdef"
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inner handler should not be called")
	})

	handler := NewAuthMiddleware(token, inner)
	req := httptest.NewRequest(http.MethodGet, "/dav/", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_MalformedHeader(t *testing.T) {
	token := "test-token-1234567890abcdef1234567890abcdef"
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inner handler should not be called")
	})

	handler := NewAuthMiddleware(token, inner)
	req := httptest.NewRequest(http.MethodGet, "/dav/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz") // Wrong scheme
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
