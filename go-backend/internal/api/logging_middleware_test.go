package api

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"investlog/pkg/investlog"
)

func setupRouterWithLogger(t *testing.T, logger *slog.Logger) (http.Handler, func()) {
	t.Helper()

	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")
	core, err := investlog.OpenWithOptions(investlog.Options{DBPath: dbPath, Logger: logger})
	if err != nil {
		t.Fatalf("open core: %v", err)
	}

	router := NewRouter(core)
	cleanup := func() {
		_ = core.Close()
	}

	return router, cleanup
}

func TestNewRouterLogsRequestCompleted(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	router, cleanup := setupRouterWithLogger(t, logger)
	defer cleanup()

	rr := doRequest(router, http.MethodGet, "/api/health", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	logs := buf.String()
	if !strings.Contains(logs, "http request completed") {
		t.Fatalf("expected request completion log, got %q", logs)
	}
	if !strings.Contains(logs, "method=GET") {
		t.Fatalf("expected method field, got %q", logs)
	}
	if !strings.Contains(logs, "path=/api/health") {
		t.Fatalf("expected path field, got %q", logs)
	}
	if !strings.Contains(logs, "status=200") {
		t.Fatalf("expected status field, got %q", logs)
	}
	if !strings.Contains(logs, "request_id=") {
		t.Fatalf("expected request_id field, got %q", logs)
	}
}

func TestNewRouterLogsWarnForBadRequest(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	router, cleanup := setupRouterWithLogger(t, logger)
	defer cleanup()

	rr := doRequest(router, http.MethodDelete, "/api/transactions/invalid", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}

	logs := buf.String()
	if !strings.Contains(logs, "level=WARN") {
		t.Fatalf("expected warn level log, got %q", logs)
	}
	if !strings.Contains(logs, "status=400") {
		t.Fatalf("expected status=400 in log, got %q", logs)
	}
	if !strings.Contains(logs, "error_message=\"invalid id\"") {
		t.Fatalf("expected error message in log, got %q", logs)
	}
}

func TestNewRouterRecoversPanicWithStructuredLog(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	oldDefault := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() {
		slog.SetDefault(oldDefault)
	})

	router := NewRouter(nil)
	rr := doRequest(router, http.MethodGet, "/api/holdings", nil)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, `{"error":"internal server error"}`) {
		t.Fatalf("expected structured error response, got %q", body)
	}

	logs := buf.String()
	if !strings.Contains(logs, "panic recovered") {
		t.Fatalf("expected panic recovery log, got %q", logs)
	}
	if !strings.Contains(logs, "request_id=") {
		t.Fatalf("expected request id in panic log, got %q", logs)
	}
}

func TestNewRouterLogsServerErrorAsErrorLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	oldDefault := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() {
		slog.SetDefault(oldDefault)
	})

	router := NewRouter(nil)
	rr := doRequest(router, http.MethodGet, "/api/holdings", nil)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}

	logs := buf.String()
	if !strings.Contains(logs, "level=ERROR") {
		t.Fatalf("expected error level log, got %q", logs)
	}
	if !strings.Contains(logs, "status=500") {
		t.Fatalf("expected status=500 in log, got %q", logs)
	}
}

func TestNewRouterUsesCoreLoggerForRequestLogs(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	router, cleanup := setupRouterWithLogger(t, logger)
	defer cleanup()

	defaultBuf := bytes.Buffer{}
	oldDefault := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&defaultBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() {
		slog.SetDefault(oldDefault)
	})

	rr := doRequest(router, http.MethodGet, "/api/health", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	if !strings.Contains(buf.String(), "http request completed") {
		t.Fatalf("expected logs written through core logger, got %q", buf.String())
	}
	if defaultBuf.Len() != 0 {
		t.Fatalf("expected no log written to slog default, got %q", defaultBuf.String())
	}
}

func TestNewRouterLoggingIncludesUserAgent(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	router, cleanup := setupRouterWithLogger(t, logger)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("User-Agent", "invest-log-test-agent")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	if !strings.Contains(buf.String(), "user_agent=invest-log-test-agent") {
		t.Fatalf("expected user_agent field in request logs, got %q", buf.String())
	}
}

func TestNewRouterRequestLogContainsDuration(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	router, cleanup := setupRouterWithLogger(t, logger)
	defer cleanup()

	rr := doRequest(router, http.MethodGet, "/api/health", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	logs := buf.String()
	if !strings.Contains(logs, "duration_ms=") {
		t.Fatalf("expected duration field in request logs, got %q", logs)
	}
}
