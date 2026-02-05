package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"investlog/internal/config"
	"investlog/pkg/investlog"
)

func TestSanitizeDBName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty", input: "", wantErr: true},
		{name: "spaces", input: "   ", wantErr: true},
		{name: "path", input: "../test.db", wantErr: true},
		{name: "separator", input: "foo/bar.db", wantErr: true},
		{name: "base name", input: "alice", want: "alice.db"},
		{name: "db ext", input: "bob.db", want: "bob.db"},
		{name: "upper ext", input: "CARL.DB", want: "CARL.DB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sanitizeDBName(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestListDBFiles(t *testing.T) {
	dir := t.TempDir()
	files := []string{"a.db", "b.DB", "c.txt"}
	for _, name := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatalf("make dir: %v", err)
	}

	got, err := listDBFiles(dir)
	if err != nil {
		t.Fatalf("listDBFiles: %v", err)
	}
	want := []string{"a.db", "b.DB"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestContainsString(t *testing.T) {
	items := []string{"alpha", "beta"}
	if !containsString(items, "beta") {
		t.Fatalf("expected value to be found")
	}
	if containsString(items, "gamma") {
		t.Fatalf("expected value to be missing")
	}
}

func TestGetStorageInfo(t *testing.T) {
	router, cleanup, dataDir, dbName := setupStorageRouter(t)
	defer cleanup()

	extraPath := filepath.Join(dataDir, "beta.db")
	if err := os.WriteFile(extraPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("write extra db: %v", err)
	}

	rr := doRequest(router, http.MethodGet, "/api/storage", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/storage: expected 200, got %d", rr.Code)
	}
	var resp storageInfoResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.DBName != dbName {
		t.Fatalf("expected db_name %q, got %q", dbName, resp.DBName)
	}
	if filepath.Clean(resp.DataDir) != filepath.Clean(dataDir) {
		t.Fatalf("expected data_dir %q, got %q", dataDir, resp.DataDir)
	}
	if !containsString(resp.Available, dbName) || !containsString(resp.Available, "beta.db") {
		t.Fatalf("expected available list to include %q and %q, got %v", dbName, "beta.db", resp.Available)
	}
	if !resp.CanSwitch {
		t.Fatalf("expected can_switch to be true")
	}
}

func TestGetStorageInfoAddsMissingDB(t *testing.T) {
	router, cleanup, dataDir, _ := setupStorageRouter(t)
	defer cleanup()

	cfg := config.LoadUserConfig()
	cfg.DBName = "ghost.db"
	if err := config.SaveUserConfig(cfg, true); err != nil {
		t.Fatalf("save config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "alpha.db"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write db file: %v", err)
	}

	rr := doRequest(router, http.MethodGet, "/api/storage", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/storage: expected 200, got %d", rr.Code)
	}
	var resp storageInfoResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Available) == 0 || resp.Available[0] != "ghost.db" {
		t.Fatalf("expected ghost.db to be prepended, got %v", resp.Available)
	}
}

func TestGetStorageInfoWithEnvPath(t *testing.T) {
	router, cleanup, _, _ := setupStorageRouter(t)
	defer cleanup()

	envDir := filepath.Join(t.TempDir(), "nested")
	t.Setenv("INVEST_LOG_DB_PATH", filepath.Join(envDir, "custom.db"))

	rr := doRequest(router, http.MethodGet, "/api/storage", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/storage: expected 200, got %d", rr.Code)
	}
	var resp storageInfoResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.DBName != "custom.db" {
		t.Fatalf("expected db_name custom.db, got %q", resp.DBName)
	}
	if resp.CanSwitch {
		t.Fatalf("expected can_switch to be false")
	}
	if resp.DataDir != envDir {
		t.Fatalf("expected data_dir %q, got %q", envDir, resp.DataDir)
	}
}

func TestSwitchStorage(t *testing.T) {
	router, cleanup, dataDir, dbName := setupStorageRouter(t)
	defer cleanup()

	t.Run("missing file", func(t *testing.T) {
		rr := doRequest(router, http.MethodPost, "/api/storage/switch", map[string]interface{}{
			"db_name": "missing.db",
		})
		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rr.Code)
		}
	})

	t.Run("active", func(t *testing.T) {
		rr := doRequest(router, http.MethodPost, "/api/storage/switch", map[string]interface{}{
			"db_name": dbName,
		})
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		payload := parseJSON(rr)
		if payload["status"] != "active" {
			t.Fatalf("expected status active, got %v", payload["status"])
		}
	})

	t.Run("path is directory", func(t *testing.T) {
		dirName := "dir.db"
		if err := os.Mkdir(filepath.Join(dataDir, dirName), 0o755); err != nil {
			t.Fatalf("make dir: %v", err)
		}
		rr := doRequest(router, http.MethodPost, "/api/storage/switch", map[string]interface{}{
			"db_name": dirName,
		})
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("create and switch", func(t *testing.T) {
		rr := doRequest(router, http.MethodPost, "/api/storage/switch", map[string]interface{}{
			"db_name": "new-user.db",
			"create":  true,
		})
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		payload := parseJSON(rr)
		if payload["status"] != "switched" {
			t.Fatalf("expected status switched, got %v", payload["status"])
		}
	})
}

func TestSwitchStorageDisabledByEnv(t *testing.T) {
	router, cleanup, _, _ := setupStorageRouter(t)
	defer cleanup()

	t.Setenv("INVEST_LOG_DB_PATH", filepath.Join(t.TempDir(), "locked.db"))
	rr := doRequest(router, http.MethodPost, "/api/storage/switch", map[string]interface{}{
		"db_name": "ignored.db",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func setupStorageRouter(t *testing.T) (http.Handler, func(), string, string) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	dataDir := filepath.Join(home, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}

	config.SetRuntimeDataDir(dataDir)
	t.Cleanup(func() {
		config.SetRuntimeDataDir("")
	})

	dbName := "alpha.db"
	cfg := config.LoadUserConfig()
	cfg.DBName = dbName
	cfg.UseICloud = false
	cfg.SetupComplete = true
	if err := config.SaveUserConfig(cfg, true); err != nil {
		t.Fatalf("save config: %v", err)
	}

	dbPath := filepath.Join(dataDir, dbName)
	core, err := investlog.Open(dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	router := NewRouter(core)

	cleanup := func() {
		_ = core.Close()
	}

	return router, cleanup, dataDir, dbName
}
