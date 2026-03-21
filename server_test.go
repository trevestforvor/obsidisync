package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func buildTestServer(t *testing.T, token string) (*http.ServeMux, string) {
	t.Helper()
	dir := t.TempDir()
	return buildMux(dir, token), dir
}

func TestIntegration_HealthNoAuth(t *testing.T) {
	mux, _ := buildTestServer(t, "test-token")

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("health should not require auth, got %d", w.Code)
	}
}

func TestIntegration_DavRequiresAuth(t *testing.T) {
	mux, _ := buildTestServer(t, "test-token")

	req := httptest.NewRequest(http.MethodGet, "/dav/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", w.Code)
	}
}

func TestIntegration_PutAndGetFile(t *testing.T) {
	token := "test-token"
	mux, _ := buildTestServer(t, token)

	body := "# Hello World\nThis is a test note."
	putReq := httptest.NewRequest(http.MethodPut, "/dav/test.md", strings.NewReader(body))
	putReq.Header.Set("Authorization", "Bearer "+token)
	putW := httptest.NewRecorder()
	mux.ServeHTTP(putW, putReq)

	if putW.Code != http.StatusCreated && putW.Code != http.StatusNoContent {
		t.Fatalf("PUT expected 201 or 204, got %d", putW.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/dav/test.md", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getW := httptest.NewRecorder()
	mux.ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("GET expected 200, got %d", getW.Code)
	}
	got, _ := io.ReadAll(getW.Body)
	if string(got) != body {
		t.Errorf("round-trip mismatch:\nwant: %q\ngot:  %q", body, string(got))
	}
}

func TestIntegration_MkcolAndPropfind(t *testing.T) {
	token := "test-token"
	mux, _ := buildTestServer(t, token)

	mkReq := httptest.NewRequest("MKCOL", "/dav/personal/", nil)
	mkReq.Header.Set("Authorization", "Bearer "+token)
	mkW := httptest.NewRecorder()
	mux.ServeHTTP(mkW, mkReq)

	if mkW.Code != http.StatusCreated {
		t.Fatalf("MKCOL expected 201, got %d", mkW.Code)
	}

	pfReq := httptest.NewRequest("PROPFIND", "/dav/", nil)
	pfReq.Header.Set("Authorization", "Bearer "+token)
	pfReq.Header.Set("Depth", "1")
	pfW := httptest.NewRecorder()
	mux.ServeHTTP(pfW, pfReq)

	if pfW.Code != http.StatusMultiStatus {
		t.Fatalf("PROPFIND expected 207, got %d", pfW.Code)
	}
	if !strings.Contains(pfW.Body.String(), "personal") {
		t.Errorf("PROPFIND response should contain 'personal'")
	}
}

func TestIntegration_DeleteFile(t *testing.T) {
	token := "test-token"
	mux, _ := buildTestServer(t, token)

	putReq := httptest.NewRequest(http.MethodPut, "/dav/delete-me.md", strings.NewReader("temp"))
	putReq.Header.Set("Authorization", "Bearer "+token)
	putW := httptest.NewRecorder()
	mux.ServeHTTP(putW, putReq)

	delReq := httptest.NewRequest(http.MethodDelete, "/dav/delete-me.md", nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delW := httptest.NewRecorder()
	mux.ServeHTTP(delW, delReq)

	if delW.Code != http.StatusNoContent {
		t.Fatalf("DELETE expected 204, got %d", delW.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/dav/delete-me.md", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getW := httptest.NewRecorder()
	mux.ServeHTTP(getW, getReq)

	if getW.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", getW.Code)
	}
}

func TestIntegration_PathTraversal(t *testing.T) {
	token := "test-token"
	mux, _ := buildTestServer(t, token)

	req := httptest.NewRequest(http.MethodGet, "/dav/../../../etc/passwd", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Fatal("path traversal should not return 200")
	}
}
