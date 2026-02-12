package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestWithSPA_ServesStaticAndIndex(t *testing.T) {
	webDir := t.TempDir()
	indexPath := filepath.Join(webDir, "index.html")
	if err := os.WriteFile(indexPath, []byte("INDEX"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	assetPath := filepath.Join(webDir, "app.js")
	if err := os.WriteFile(assetPath, []byte("APP"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	apiHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("API"))
	})

	h := WithSPA(apiHandler, webDir)

	// API paths should be forwarded to the API handler.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Body.String() != "API" {
		t.Fatalf("expected API response, got %q", rr.Body.String())
	}

	// Root should serve index.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for index, got %d", rr.Code)
	}
	if rr.Body.String() != "INDEX" {
		t.Fatalf("expected index body, got %q", rr.Body.String())
	}
	if got := rr.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected no-store for index, got %q", got)
	}

	// Static asset should be served.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/app.js", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for asset, got %d", rr.Code)
	}
	if rr.Body.String() != "APP" {
		t.Fatalf("expected asset body, got %q", rr.Body.String())
	}
	if got := rr.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected no-store for asset, got %q", got)
	}

	// Unknown path should fall back to index.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/does/not/exist", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for fallback, got %d", rr.Code)
	}
	if rr.Body.String() != "INDEX" {
		t.Fatalf("expected fallback index, got %q", rr.Body.String())
	}
	if got := rr.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected no-store for fallback index, got %q", got)
	}
}

func TestWithSPA_IndexMissing(t *testing.T) {
	webDir := t.TempDir()
	apiHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("API"))
	})

	h := WithSPA(apiHandler, webDir)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
	if rr.Body.String() != "index.html not found" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}
