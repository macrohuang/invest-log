package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRuntimePort(t *testing.T) {
	orig := GetRuntimePort()
	defer SetRuntimePort(orig)

	SetRuntimePort(0)
	if got := GetRuntimePort(); got != orig {
		t.Fatalf("expected port to remain %d, got %d", orig, got)
	}

	SetRuntimePort(9090)
	if got := GetRuntimePort(); got != 9090 {
		t.Fatalf("expected port 9090, got %d", got)
	}
}

func TestRuntimeDataDirAndEnv(t *testing.T) {
	SetRuntimeDataDir("")
	defer SetRuntimeDataDir("")

	tmp := t.TempDir()
	SetRuntimeDataDir(tmp)
	dir, err := GetDataDir()
	if err != nil {
		t.Fatalf("GetDataDir: %v", err)
	}
	if dir != tmp {
		t.Fatalf("expected runtime dir %q, got %q", tmp, dir)
	}

	SetRuntimeDataDir("")
	tmpEnv := filepath.Join(t.TempDir(), "data")
	t.Setenv("INVEST_LOG_DATA_DIR", tmpEnv)
	dir, err = GetDataDir()
	if err != nil {
		t.Fatalf("GetDataDir env: %v", err)
	}
	if dir != tmpEnv {
		t.Fatalf("expected env dir %q, got %q", tmpEnv, dir)
	}
}

func TestGetDBPathEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	t.Setenv("INVEST_LOG_DB_PATH", path)
	got, err := GetDBPath()
	if err != nil {
		t.Fatalf("GetDBPath: %v", err)
	}
	if got != path {
		t.Fatalf("expected %q, got %q", path, got)
	}
}

func TestIsMacOSWindows(t *testing.T) {
	if IsMacOS() != (runtime.GOOS == "darwin") {
		t.Fatalf("IsMacOS mismatch")
	}
	if IsWindows() != (runtime.GOOS == "windows") {
		t.Fatalf("IsWindows mismatch")
	}
}

func TestIsFirstRunAndLoadSaveConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if !IsFirstRun() {
		t.Fatalf("expected first run with no config")
	}

	cfg := UserConfig{
		DBName:        "my.db",
		UseICloud:     false,
		DataDir:       filepath.Join(home, "data"),
		SetupComplete: true,
	}
	if err := SaveUserConfig(cfg, true); err != nil {
		t.Fatalf("SaveUserConfig: %v", err)
	}

	if IsFirstRun() {
		t.Fatalf("expected not first run after save")
	}

	loaded := LoadUserConfig()
	if loaded.DBName != cfg.DBName || loaded.DataDir != cfg.DataDir || loaded.UseICloud != cfg.UseICloud || loaded.SetupComplete != cfg.SetupComplete {
		t.Fatalf("loaded config mismatch: %+v", loaded)
	}
}

func TestLegacyConfigPath(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(cwd)
	}()

	path := filepath.Join(tmp, "config.json")
	if err := os.WriteFile(path, []byte(`{"db_name":"legacy.db"}`), 0o644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	legacy := legacyConfigPath()
	if legacy == "" {
		t.Fatalf("expected legacy path, got empty")
	}
	legacyEval, legacyErr := filepath.EvalSymlinks(legacy)
	pathEval, pathErr := filepath.EvalSymlinks(path)
	if legacyErr == nil && pathErr == nil {
		if legacyEval != pathEval {
			t.Fatalf("expected legacy path %q, got %q", pathEval, legacyEval)
		}
	} else if legacy != path {
		t.Fatalf("expected legacy path %q, got %q", path, legacy)
	}
}

func TestCompleteSetupCustomDirAndExistingDB(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Existing DB file
	existing := filepath.Join(t.TempDir(), "existing.db")
	if err := os.WriteFile(existing, []byte("data"), 0o644); err != nil {
		t.Fatalf("write existing db: %v", err)
	}
	customDir := filepath.Join(t.TempDir(), "custom")

	dataDir, err := CompleteSetup(false, customDir, existing, "custom.db")
	if err != nil {
		t.Fatalf("CompleteSetup: %v", err)
	}
	if dataDir != customDir {
		t.Fatalf("expected data dir %q, got %q", customDir, dataDir)
	}
	expectedName := filepath.Base(existing)
	if _, err := os.Stat(filepath.Join(customDir, expectedName)); err != nil {
		t.Fatalf("expected copied db: %v", err)
	}
}

func TestCompleteSetupExistingDBIsDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := t.TempDir()
	if _, err := CompleteSetup(false, "", dir, ""); err == nil {
		t.Fatalf("expected error for directory path")
	}
}

func TestICloudPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if IsMacOS() {
		base := filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs")
		if err := os.MkdirAll(base, 0o755); err != nil {
			t.Fatalf("mkdir icloud: %v", err)
		}
		if !IsICloudAvailable() {
			t.Fatalf("expected icloud available")
		}
		appFolder := GetICloudAppFolder()
		if appFolder == "" {
			t.Fatalf("expected icloud app folder")
		}
	}
}

func TestStringsTrim(t *testing.T) {
	if got := stringsTrim("  hello  "); got != "hello" {
		t.Fatalf("stringsTrim: got %q", got)
	}
}

func TestSaveUserConfigLegacyPath(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(cwd)
	}()

	cfg := UserConfig{DBName: "legacy.db", UseICloud: false}
	if err := SaveUserConfig(cfg, false); err != nil {
		t.Fatalf("SaveUserConfig legacy: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "config.json")); err != nil {
		t.Fatalf("expected legacy config file: %v", err)
	}
}

func TestGetDataDirFromConfigAndICloud(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	customDir := filepath.Join(t.TempDir(), "data")
	cfg := UserConfig{DBName: "db.db", UseICloud: false, DataDir: customDir, SetupComplete: true}
	if err := SaveUserConfig(cfg, true); err != nil {
		t.Fatalf("SaveUserConfig: %v", err)
	}
	dir, err := GetDataDir()
	if err != nil {
		t.Fatalf("GetDataDir: %v", err)
	}
	if dir != customDir {
		t.Fatalf("expected data dir %q, got %q", customDir, dir)
	}

	// iCloud path when enabled and available.
	if IsMacOS() {
		icloudBase := filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs")
		if err := os.MkdirAll(icloudBase, 0o755); err != nil {
			t.Fatalf("mkdir icloud: %v", err)
		}
		cfg = UserConfig{DBName: "db.db", UseICloud: true, DataDir: "", SetupComplete: true}
		if err := SaveUserConfig(cfg, true); err != nil {
			t.Fatalf("SaveUserConfig icloud: %v", err)
		}
		dir, err = GetDataDir()
		if err != nil {
			t.Fatalf("GetDataDir icloud: %v", err)
		}
		if dir != filepath.Join(icloudBase, "InvestLog") {
			t.Fatalf("expected icloud dir, got %q", dir)
		}
	}
}

func TestCompleteSetupVariants(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// existing DB, no custom dir -> use existing dir
	existingDir := t.TempDir()
	existing := filepath.Join(existingDir, "existing.db")
	if err := os.WriteFile(existing, []byte("data"), 0o644); err != nil {
		t.Fatalf("write existing: %v", err)
	}
	dir, err := CompleteSetup(false, "", existing, "")
	if err != nil {
		t.Fatalf("CompleteSetup existing: %v", err)
	}
	if dir != existingDir {
		t.Fatalf("expected dir %q, got %q", existingDir, dir)
	}

	// no existing DB, custom dir
	custom := filepath.Join(t.TempDir(), "custom")
	dir, err = CompleteSetup(false, custom, "", "")
	if err != nil {
		t.Fatalf("CompleteSetup custom: %v", err)
	}
	if dir != custom {
		t.Fatalf("expected custom dir %q, got %q", custom, dir)
	}

	// no existing DB, default dir
	dir, err = CompleteSetup(false, "", "", "")
	if err != nil {
		t.Fatalf("CompleteSetup default: %v", err)
	}
	if dir == "" {
		t.Fatalf("expected default dir")
	}
}

func TestCompleteSetupICloudUnavailable(t *testing.T) {
	if !IsMacOS() {
		return
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	existing := filepath.Join(t.TempDir(), "existing.db")
	if err := os.WriteFile(existing, []byte("data"), 0o644); err != nil {
		t.Fatalf("write existing: %v", err)
	}
	if _, err := CompleteSetup(true, "", existing, ""); err == nil {
		t.Fatalf("expected iCloud unavailable error")
	}
}

func TestGetDBPathFromConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := UserConfig{DBName: "config.db", UseICloud: false, DataDir: filepath.Join(home, "data"), SetupComplete: true}
	if err := SaveUserConfig(cfg, true); err != nil {
		t.Fatalf("SaveUserConfig: %v", err)
	}
	path, err := GetDBPath()
	if err != nil {
		t.Fatalf("GetDBPath: %v", err)
	}
	if path != filepath.Join(cfg.DataDir, cfg.DBName) {
		t.Fatalf("expected db path %q, got %q", filepath.Join(cfg.DataDir, cfg.DBName), path)
	}
}
