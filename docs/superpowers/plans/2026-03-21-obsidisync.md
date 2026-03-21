# Obsidisync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go WebDAV server that exposes `Home/Obsidian/` from Olares for vault sync via Remotely Save, packaged as an Olares marketplace app.

**Architecture:** Single Go binary serving WebDAV at `/dav/*` and health at `/api/health`, with Bearer token auth middleware. Packaged as a distroless Docker image and deployed via Helm chart following Olares conventions.

**Tech Stack:** Go, `golang.org/x/net/webdav`, `net/http`, Docker multi-stage build, Helm 3, GitHub Actions

**Spec:** `docs/superpowers/specs/2026-03-21-obsidisync-design.md`

---

## File Structure

```
go.mod
go.sum
main.go                 # Entry point: wires handlers, starts server, graceful shutdown
auth.go                 # Bearer token auth middleware
auth_test.go            # Auth middleware tests
health.go               # Health check handler
health_test.go          # Health handler tests
server_test.go          # Integration tests (full server with WebDAV + auth + health)
Dockerfile              # Multi-stage build → distroless
obsidisync/             # Helm chart directory
  Chart.yaml
  OlaresManifest.yaml
  values.yaml
  owners
  .helmignore
  templates/
    deployment.yaml     # Secret + ConfigMap + Deployment + Service
  i18n/
    en-US/
      OlaresManifest.yaml
    zh-CN/
      OlaresManifest.yaml
.github/
  workflows/
    release.yaml        # Build + push Docker image + package Helm chart on tag
```

---

### Task 1: Project Scaffolding + Health Endpoint

**Files:**
- Create: `go.mod`
- Create: `health.go`
- Create: `health_test.go`
- Create: `main.go`

This task sets up the Go module, implements the health check endpoint, and creates a minimal `main.go` that starts an HTTP server. The health endpoint is the simplest component and validates the project builds and tests run.

- [ ] **Step 1: Initialize Go module**

```bash
cd /c/Users/treve/obsidisync
go mod init github.com/treve/obsidisync
```

- [ ] **Step 2: Write the health handler failing test**

Create `health_test.go`:

```go
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHealthHandler_OK(t *testing.T) {
	// Create a temp directory to act as vault root
	dir := t.TempDir()
	h := NewHealthHandler(dir)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %v", resp["status"])
	}
	if resp["writable"] != true {
		t.Errorf("expected writable true, got %v", resp["writable"])
	}
	if resp["vault_root"] != dir {
		t.Errorf("expected vault_root %s, got %v", dir, resp["vault_root"])
	}
}

func TestHealthHandler_Unavailable(t *testing.T) {
	// Point to a directory that does not exist
	h := NewHealthHandler("/nonexistent/path/that/does/not/exist")

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp["status"] != "error" {
		t.Errorf("expected status error, got %v", resp["status"])
	}
	if resp["writable"] != false {
		t.Errorf("expected writable false, got %v", resp["writable"])
	}
}

func TestHealthHandler_NotWritable(t *testing.T) {
	// Create a read-only directory
	dir := t.TempDir()
	roDir := filepath.Join(dir, "readonly")
	os.MkdirAll(roDir, 0o444)

	h := NewHealthHandler(roDir)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	// On Windows, 0o444 may not prevent writes, so accept 200 or 503
	if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 200 or 503, got %d", w.Code)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
go test -v -run TestHealth ./...
```

Expected: FAIL — `NewHealthHandler` not defined.

- [ ] **Step 4: Implement the health handler**

Create `health.go`:

```go
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

	// Try writing a temp file to verify write access
	probe := filepath.Join(h.vaultRoot, fmt.Sprintf(".health-probe-%d", os.Getpid()))
	f, err := os.Create(probe)
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(probe)
	return true
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test -v -run TestHealth ./...
```

Expected: PASS (all TestHealth* tests).

- [ ] **Step 6: Write minimal main.go**

Create `main.go`:

```go
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
```

- [ ] **Step 7: Verify build**

```bash
go build -o obsidisync.exe .
```

Expected: Builds successfully.

- [ ] **Step 8: Commit**

```bash
git add go.mod health.go health_test.go main.go
git commit -m "feat: project scaffolding with health endpoint"
```

---

### Task 2: Auth Middleware

**Files:**
- Create: `auth.go`
- Create: `auth_test.go`

This task implements the Bearer token authentication middleware. It validates tokens using constant-time comparison and returns 401 for missing/invalid tokens. The `/api/health` endpoint must bypass auth. This middleware wraps handlers that need protection.

- [ ] **Step 1: Write the auth middleware failing tests**

Create `auth_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test -v -run TestAuthMiddleware ./...
```

Expected: FAIL — `NewAuthMiddleware` not defined.

- [ ] **Step 3: Implement the auth middleware**

Create `auth.go`:

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test -v -run TestAuthMiddleware ./...
```

Expected: PASS (all TestAuthMiddleware* tests).

- [ ] **Step 5: Commit**

```bash
git add auth.go auth_test.go
git commit -m "feat: bearer token auth middleware with constant-time comparison"
```

---

### Task 3: WebDAV Handler + Server Wiring

**Files:**
- Create: `server_test.go`
- Modify: `main.go`

This task adds the WebDAV handler using `golang.org/x/net/webdav`, wires it into `main.go` behind the auth middleware, and adds path traversal defense. Integration tests verify the full request flow: auth + WebDAV operations together.

- [ ] **Step 1: Add the webdav dependency**

```bash
go get golang.org/x/net/webdav
```

- [ ] **Step 2: Write integration tests**

Create `server_test.go`:

```go
package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/net/webdav"
)

// buildTestServer creates a full server mux with auth, WebDAV, and health,
// using a temp directory as vault root.
func buildTestServer(t *testing.T, token string) (*http.ServeMux, string) {
	t.Helper()
	dir := t.TempDir()

	davHandler := &webdav.Handler{
		FileSystem: webdav.Dir(dir),
		LockSystem: webdav.NewMemLS(),
		Prefix:     "/dav",
	}

	mux := http.NewServeMux()
	mux.Handle("/api/health", NewHealthHandler(dir))
	mux.Handle("/dav/", NewAuthMiddleware(token, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Path traversal defense in depth
		if strings.Contains(r.URL.Path, "..") {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		davHandler.ServeHTTP(w, r)
	})))

	return mux, dir
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

	// PUT a file
	body := "# Hello World\nThis is a test note."
	putReq := httptest.NewRequest(http.MethodPut, "/dav/test.md", strings.NewReader(body))
	putReq.Header.Set("Authorization", "Bearer "+token)
	putW := httptest.NewRecorder()
	mux.ServeHTTP(putW, putReq)

	if putW.Code != http.StatusCreated && putW.Code != http.StatusNoContent {
		t.Fatalf("PUT expected 201 or 204, got %d", putW.Code)
	}

	// GET the file back
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

	// MKCOL to create a vault directory
	mkReq := httptest.NewRequest("MKCOL", "/dav/personal/", nil)
	mkReq.Header.Set("Authorization", "Bearer "+token)
	mkW := httptest.NewRecorder()
	mux.ServeHTTP(mkW, mkReq)

	if mkW.Code != http.StatusCreated {
		t.Fatalf("MKCOL expected 201, got %d", mkW.Code)
	}

	// PROPFIND to list
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

	// PUT a file first
	putReq := httptest.NewRequest(http.MethodPut, "/dav/delete-me.md", strings.NewReader("temp"))
	putReq.Header.Set("Authorization", "Bearer "+token)
	putW := httptest.NewRecorder()
	mux.ServeHTTP(putW, putReq)

	// DELETE
	delReq := httptest.NewRequest(http.MethodDelete, "/dav/delete-me.md", nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delW := httptest.NewRecorder()
	mux.ServeHTTP(delW, delReq)

	if delW.Code != http.StatusNoContent {
		t.Fatalf("DELETE expected 204, got %d", delW.Code)
	}

	// Verify gone
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

	// Should be blocked — either 403 from our check or webdav.Dir prevents it
	if w.Code == http.StatusOK {
		t.Fatal("path traversal should not return 200")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test -v -run TestIntegration ./...
```

Expected: FAIL — tests compile but `buildTestServer` duplicates wiring we need in main.

- [ ] **Step 4: Update main.go with WebDAV handler and extracted server builder**

Replace `main.go` with:

```go
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
```

- [ ] **Step 5: Update server_test.go to use buildMux**

Replace the `buildTestServer` helper in `server_test.go`:

```go
func buildTestServer(t *testing.T, token string) (*http.ServeMux, string) {
	t.Helper()
	dir := t.TempDir()
	return buildMux(dir, token), dir
}
```

Remove the `"golang.org/x/net/webdav"` import from server_test.go (no longer needed directly in test file).

- [ ] **Step 6: Run all tests**

```bash
go test -v ./...
```

Expected: PASS (all health, auth, and integration tests).

- [ ] **Step 7: Verify build**

```bash
go build -o obsidisync.exe .
```

Expected: Builds successfully.

- [ ] **Step 8: Commit**

```bash
git add main.go server_test.go go.mod go.sum
git commit -m "feat: webdav handler with auth and path traversal defense"
```

---

### Task 4: Graceful Shutdown

**Files:**
- Modify: `main.go`

This task adds SIGTERM/SIGINT signal handling so the server drains in-flight requests before exiting. This prevents corrupted writes during pod restarts. No unit test for this — it's wiring code that relies on OS signals. The integration tests from Task 3 continue to validate the server works.

- [ ] **Step 1: Update main.go with graceful shutdown**

Replace the `main()` function in `main.go`:

```go
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
```

- [ ] **Step 2: Run all tests to verify nothing broke**

```bash
go test -v ./...
```

Expected: PASS (all existing tests still pass).

- [ ] **Step 3: Verify build**

```bash
go build -o obsidisync.exe .
```

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "feat: graceful shutdown on SIGTERM/SIGINT"
```

---

### Task 5: Dockerfile

**Files:**
- Create: `Dockerfile`
- Create: `.dockerignore`

This task creates a multi-stage Docker build that compiles the Go binary and copies it into a distroless image. The final image should be ~20MB.

- [ ] **Step 1: Create .dockerignore**

Create `.dockerignore`:

```
.git/
docs/
obsidisync/
*.tgz
*.exe
.github/
```

- [ ] **Step 2: Create the Dockerfile**

Create `Dockerfile`:

```dockerfile
FROM golang:1.24-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o obsidisync .

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /build/obsidisync /usr/local/bin/obsidisync

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/obsidisync"]
```

- [ ] **Step 3: Verify Dockerfile syntax (dry run)**

```bash
# If Docker is available:
docker build --no-cache -t obsidisync:test .
# If not, just verify the file exists and is well-formed
cat Dockerfile
```

Note: Docker may not be available in the dev environment. If it's not, skip the build and just verify the file content is correct. The CI/CD pipeline will validate it.

- [ ] **Step 4: Commit**

```bash
git add Dockerfile .dockerignore
git commit -m "feat: multi-stage Dockerfile with distroless base"
```

---

### Task 6: Helm Chart (Olares Packaging)

**Files:**
- Create: `obsidisync/Chart.yaml`
- Create: `obsidisync/OlaresManifest.yaml`
- Create: `obsidisync/values.yaml`
- Create: `obsidisync/owners`
- Create: `obsidisync/.helmignore`
- Create: `obsidisync/templates/deployment.yaml`
- Create: `obsidisync/i18n/en-US/OlaresManifest.yaml`
- Create: `obsidisync/i18n/zh-CN/OlaresManifest.yaml`

This is the Olares marketplace packaging. Every name field must be `obsidisync`. All 4 version fields must be `0.1.0`. The entrance auth level must be `public` (Remotely Save cannot do Olares SSO). Follow the patterns in `CLAUDE.md` exactly.

**Critical Olares rules (from CLAUDE.md):**
- `appid` = folder name = `name` in Chart.yaml = deployment name = service name = entrance name = entrance host = `obsidisync`
- Deployment and Service names must be hardcoded (NOT `{{ .Release.Name }}`)
- `metadata.name` must exist and match `appid`
- Title: max 30 chars, `[a-z0-9A-Z- ]` only
- `requiredMemory` >= sum of container memory requests (64Mi >= 64Mi ✓)
- `requiredCpu` >= sum of container CPU requests (100m >= 100m ✓)
- Both i18n directories must exist

- [ ] **Step 1: Create directory structure**

```bash
mkdir -p obsidisync/templates obsidisync/i18n/en-US obsidisync/i18n/zh-CN
```

- [ ] **Step 2: Create Chart.yaml**

Create `obsidisync/Chart.yaml`:

```yaml
apiVersion: v2
appVersion: '0.1.0'
description: 'WebDAV bridge for Obsidian vault sync on Olares'
name: obsidisync
type: application
version: '0.1.0'
```

- [ ] **Step 3: Create OlaresManifest.yaml**

Create `obsidisync/OlaresManifest.yaml`:

```yaml
olaresManifest.version: '0.11.0'
olaresManifest.type: app
metadata:
  name: obsidisync
  appid: obsidisync
  title: Obsidisync
  icon: https://raw.githubusercontent.com/treve/obsidisync/main/icon.png
  description: WebDAV bridge for Obsidian vault sync
  version: 0.1.0
  versionName: '0.1.0'
  categories:
    - Productivity
entrances:
  - name: obsidisync
    host: obsidisync
    port: 8080
    title: Obsidisync
    authLevel: public
spec:
  versionName: '0.1.0'
  fullDescription: |
    Obsidisync exposes your Olares Obsidian vaults over WebDAV so you can sync
    them with Obsidian's Remotely Save plugin across desktop, mobile, and
    automation tools like Claude.

    Features:
    - Serves all vaults under Home/Obsidian/ as WebDAV directories
    - Bearer token authentication for secure access
    - Works with Obsidian Remotely Save plugin
    - Compatible with any WebDAV client or curl
    - Lightweight Go server (~20MB container image)
  developer: treve
  website: https://github.com/treve/obsidisync
  sourceCode: https://github.com/treve/obsidisync
  submitter: treve
  locale:
    - en-US
    - zh-CN
  license:
    - text: MIT
  category: Productivity
  requiredMemory: 64Mi
  limitedMemory: 256Mi
  requiredCpu: 100m
  limitedCpu: 500m
  requiredGpu: 0
  limitedGpu: 0
  requiredDisk: 256Mi
  limitedDisk: 1Gi
  supportArch:
    - amd64
permission:
  appData: true
  appCache: true
  userData:
    - Home
middleware: {}
options:
  apiTimeout: 0
  dependencies:
    - type: system
      name: olares
      version: '>=1.12.3-0'
```

- [ ] **Step 4: Create values.yaml**

Create `obsidisync/values.yaml`:

```yaml
admin: ""
bfl:
  username: ""
userspace:
  appData: ""
  appCache: ""
  userData: ""
image:
  repository: "ghcr.io/treve/obsidisync"
  tag: "0.1.0"
```

- [ ] **Step 5: Create owners**

Create `obsidisync/owners`:

```yaml
owners:
- 'treve'
```

- [ ] **Step 6: Create .helmignore**

Create `obsidisync/.helmignore`:

```
.DS_Store
.git/
docs/
src/
*.tgz
docker/
```

- [ ] **Step 7: Create deployment.yaml**

Create `obsidisync/templates/deployment.yaml`:

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: obsidisync-secret
  namespace: "{{ .Release.Namespace }}"
  annotations:
    "helm.sh/hook": pre-install
    "helm.sh/hook-delete-policy": before-hook-creation
type: Opaque
data:
  {{- if (lookup "v1" "Secret" .Release.Namespace "obsidisync-secret") }}
  vault-token: {{ index (lookup "v1" "Secret" .Release.Namespace "obsidisync-secret").data "vault-token" }}
  {{- else }}
  vault-token: {{ randAlphaNum 44 | b64enc | quote }}
  {{- end }}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: obsidisync-env
  namespace: "{{ .Release.Namespace }}"
data:
  VAULT_ROOT: "/srv/vault"
  LISTEN_ADDR: ":8080"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    io.kompose.service: obsidisync
  name: obsidisync
  namespace: "{{ .Release.Namespace }}"
spec:
  replicas: 1
  selector:
    matchLabels:
      io.kompose.service: obsidisync
  strategy:
    type: Recreate
  template:
    metadata:
      creationTimestamp: null
      labels:
        io.kompose.network/chrome-default: "true"
        io.kompose.service: obsidisync
    spec:
      containers:
        - name: obsidisync
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
          ports:
            - containerPort: 8080
              protocol: TCP
          env:
            - name: VAULT_TOKEN
              valueFrom:
                secretKeyRef:
                  name: obsidisync-secret
                  key: vault-token
            - name: VAULT_ROOT
              valueFrom:
                configMapKeyRef:
                  name: obsidisync-env
                  key: VAULT_ROOT
            - name: LISTEN_ADDR
              valueFrom:
                configMapKeyRef:
                  name: obsidisync-env
                  key: LISTEN_ADDR
          readinessProbe:
            httpGet:
              path: /api/health
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
          livenessProbe:
            httpGet:
              path: /api/health
              port: 8080
            initialDelaySeconds: 15
            periodSeconds: 30
          resources:
            limits:
              cpu: "500m"
              memory: "256Mi"
            requests:
              cpu: "100m"
              memory: "64Mi"
          volumeMounts:
            - mountPath: "/srv/vault"
              name: vaultdata
            - mountPath: "/var/lib/vault-bridge"
              name: appdata
            - mountPath: "/var/cache/vault-bridge"
              name: appcache
      volumes:
        - name: vaultdata
          hostPath:
            path: "{{ .Values.userspace.userData }}/Obsidian"
            type: DirectoryOrCreate
        - name: appdata
          hostPath:
            path: "{{ .Values.userspace.appData }}/obsidisync"
            type: DirectoryOrCreate
        - name: appcache
          hostPath:
            path: "{{ .Values.userspace.appCache }}/obsidisync"
            type: DirectoryOrCreate
      restartPolicy: Always
status: {}
---
apiVersion: v1
kind: Service
metadata:
  creationTimestamp: null
  labels:
    io.kompose.service: obsidisync
  name: obsidisync
  namespace: "{{ .Release.Namespace }}"
spec:
  ports:
    - name: "webdav"
      port: 8080
      targetPort: 8080
  selector:
    io.kompose.service: obsidisync
status:
  loadBalancer: {}
```

- [ ] **Step 8: Create i18n files**

Create `obsidisync/i18n/en-US/OlaresManifest.yaml`:

```yaml
metadata:
  title: Obsidisync
  description: WebDAV bridge for Obsidian vault sync
spec:
  fullDescription: |
    Obsidisync exposes your Olares Obsidian vaults over WebDAV so you can sync
    them with Obsidian's Remotely Save plugin across desktop, mobile, and
    automation tools like Claude.

    Features:
    - Serves all vaults under Home/Obsidian/ as WebDAV directories
    - Bearer token authentication for secure access
    - Works with Obsidian Remotely Save plugin
    - Compatible with any WebDAV client or curl
    - Lightweight Go server (~20MB container image)
```

Create `obsidisync/i18n/zh-CN/OlaresManifest.yaml`:

```yaml
metadata:
  title: Obsidisync
  description: Obsidian 保险库 WebDAV 同步桥接
spec:
  fullDescription: |
    Obsidisync 通过 WebDAV 协议暴露您在 Olares 上的 Obsidian 保险库，
    让您可以使用 Obsidian 的 Remotely Save 插件在桌面端、移动端以及
    Claude 等自动化工具之间同步笔记。

    功能特性：
    - 将 Home/Obsidian/ 下的所有保险库作为 WebDAV 目录提供
    - Bearer token 身份验证确保安全访问
    - 兼容 Obsidian Remotely Save 插件
    - 兼容任何 WebDAV 客户端或 curl
    - 轻量级 Go 服务器（约 20MB 容器镜像）
```

- [ ] **Step 9: Lint the Helm chart**

```bash
helm lint obsidisync/
```

Expected: No errors (warnings about missing values are OK since Olares injects them at install time).

- [ ] **Step 10: Verify all 4 version fields match**

```bash
grep -n "version" obsidisync/Chart.yaml obsidisync/OlaresManifest.yaml | grep "0.1.0"
```

Expected: 4 matches, all showing `0.1.0`.

- [ ] **Step 11: Verify name consistency**

```bash
grep -n "obsidisync" obsidisync/Chart.yaml obsidisync/OlaresManifest.yaml obsidisync/templates/deployment.yaml | head -30
```

Expected: All name references are `obsidisync`, no `{{ .Release.Name }}` in deployment/service names.

- [ ] **Step 12: Commit**

```bash
git add obsidisync/
git commit -m "feat: olares helm chart with secret, configmap, deployment, service"
```

---

### Task 7: GitHub Actions CI/CD

**Files:**
- Create: `.github/workflows/release.yaml`

This task creates a GitHub Actions workflow that triggers on version tag pushes (`v*`). It builds the Go binary, builds and pushes the Docker image to GHCR, and packages the Helm chart.

- [ ] **Step 1: Create workflows directory**

```bash
mkdir -p .github/workflows
```

- [ ] **Step 2: Create release.yaml**

Create `.github/workflows/release.yaml`:

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: read
  packages: write

jobs:
  build-and-push:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: Run tests
        run: go test -v ./...

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Extract version from tag
        id: version
        run: echo "VERSION=${GITHUB_REF_NAME#v}" >> "$GITHUB_OUTPUT"

      - name: Build and push Docker image
        uses: docker/build-push-action@v6
        with:
          context: .
          push: true
          tags: |
            ghcr.io/${{ github.repository }}:${{ steps.version.outputs.VERSION }}
            ghcr.io/${{ github.repository }}:latest
          platforms: linux/amd64

      - name: Install Helm
        uses: azure/setup-helm@v4

      - name: Package Helm chart
        run: |
          helm lint obsidisync/
          helm package obsidisync/ -d charts/

      - name: Upload chart artifact
        uses: actions/upload-artifact@v4
        with:
          name: helm-chart
          path: charts/*.tgz
```

- [ ] **Step 3: Commit**

```bash
git add .github/
git commit -m "ci: github actions workflow for docker + helm on tag push"
```

---

## Verification Checklist

After all tasks are complete, verify:

- [ ] `go test -v ./...` — all tests pass
- [ ] `go build -o obsidisync.exe .` — builds cleanly
- [ ] `helm lint obsidisync/` — no errors
- [ ] All 4 version fields are `0.1.0`
- [ ] All name fields are `obsidisync` (grep for `{{ .Release.Name }}` should find nothing in deployment.yaml)
- [ ] `obsidisync/i18n/en-US/` and `obsidisync/i18n/zh-CN/` both exist with valid YAML
- [ ] Entrance `authLevel: public` (required for Remotely Save)
- [ ] `requiredMemory` (64Mi) >= container memory request (64Mi) ✓
- [ ] `requiredCpu` (100m) >= container CPU request (100m) ✓
