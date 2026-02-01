package api

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"investlog/pkg/investlog"
)

func setupClosedRouter(t *testing.T) (http.Handler, func()) {
	t.Helper()
	base := t.TempDir()
	dbPath := filepath.Join(base, "test.db")
	core, err := investlog.Open(dbPath)
	if err != nil {
		t.Fatalf("open core: %v", err)
	}
	router := NewRouter(core)
	_ = core.Close()

	cleanup := func() {
		_ = os.RemoveAll(base)
	}
	return router, cleanup
}

func TestHandlersInternalErrorsWhenDBClosed(t *testing.T) {
	router, cleanup := setupClosedRouter(t)
	defer cleanup()

	endpoints := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/api/holdings", nil},
		{http.MethodGet, "/api/holdings-by-currency", nil},
		{http.MethodGet, "/api/holdings-by-symbol", nil},
		{http.MethodGet, "/api/holdings-by-currency-account", nil},
		{http.MethodGet, "/api/transactions", nil},
		{http.MethodGet, "/api/accounts", nil},
		{http.MethodGet, "/api/asset-types", nil},
		{http.MethodGet, "/api/symbols", nil},
		{http.MethodGet, "/api/operation-logs", nil},
		{http.MethodGet, "/api/portfolio-history", nil},
	}

	for _, ep := range endpoints {
		rr := doRequest(router, ep.method, ep.path, ep.body)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("%s %s: expected 500, got %d", ep.method, ep.path, rr.Code)
		}
	}

	// Update symbol auto-update should return bad request on DB error.
	rr := doRequest(router, http.MethodPost, "/api/symbols/AAA/auto-update", map[string]any{"auto_update": 1})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("updateSymbolAutoUpdate: expected 400, got %d", rr.Code)
	}

	rr = doRequest(router, http.MethodPut, "/api/symbols/AAA", map[string]any{"name": "Name"})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("updateSymbol: expected 400, got %d", rr.Code)
	}
}
