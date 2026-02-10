package api

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"investlog/internal/config"
	"investlog/pkg/investlog"
)

func (h *handler) coreLockMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/storage/switch" {
			next.ServeHTTP(w, r)
			return
		}
		h.coreMu.RLock()
		defer h.coreMu.RUnlock()
		next.ServeHTTP(w, r)
	})
}

func (h *handler) getStorageInfo(w http.ResponseWriter, r *http.Request) {
	envDBPath := strings.TrimSpace(os.Getenv("INVEST_LOG_DB_PATH"))
	cfg := config.LoadUserConfig()

	var (
		dataDir string
		dbPath  string
		err     error
	)
	if envDBPath != "" {
		dbPath = envDBPath
		dataDir = filepath.Dir(envDBPath)
	} else {
		dataDir, err = config.GetDataDir()
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("load data dir: %w", err).Error())
			return
		}
		dbPath, err = config.GetDBPath()
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("resolve db path: %w", err).Error())
			return
		}
	}

	dbName := cfg.DBName
	if envDBPath != "" {
		dbName = filepath.Base(envDBPath)
	} else if dbName == "" {
		dbName = filepath.Base(dbPath)
	}

	available, err := listDBFiles(dataDir)
	if err != nil {
		if envDBPath == "" || !errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("list storage files: %w", err).Error())
			return
		}
		available = []string{}
	}
	if dbName != "" && !containsString(available, dbName) {
		available = append([]string{dbName}, available...)
	}

	canSwitch := envDBPath == ""
	reason := ""
	if !canSwitch {
		reason = "Switching disabled when INVEST_LOG_DB_PATH is set."
	}

	writeJSON(w, http.StatusOK, storageInfoResponse{
		DBName:       dbName,
		DBPath:       dbPath,
		DataDir:      dataDir,
		UseICloud:    cfg.UseICloud,
		Available:    available,
		CanSwitch:    canSwitch,
		SwitchReason: reason,
	})
}

func (h *handler) switchStorage(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(os.Getenv("INVEST_LOG_DB_PATH")) != "" {
		writeError(w, http.StatusBadRequest, "switching disabled when INVEST_LOG_DB_PATH is set")
		return
	}

	var payload storageSwitchPayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	dbName, err := sanitizeDBName(payload.DBName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	dataDir, err := config.GetDataDir()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("load data dir: %w", err).Error())
		return
	}
	targetPath := filepath.Join(dataDir, dbName)

	h.coreMu.RLock()
	currentPath := ""
	if h.core != nil {
		currentPath = h.core.DBPath()
	}
	h.coreMu.RUnlock()
	if currentPath != "" && filepath.Clean(currentPath) == filepath.Clean(targetPath) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "active", "db_name": dbName})
		return
	}

	if info, err := os.Stat(targetPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if !payload.Create {
				writeError(w, http.StatusNotFound, "storage file not found")
				return
			}
		} else {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("stat storage file: %w", err).Error())
			return
		}
	} else if info.IsDir() {
		writeError(w, http.StatusBadRequest, "storage path is a directory")
		return
	}

	logger := h.logger
	newCore, err := investlog.OpenWithOptions(investlog.Options{
		DBPath: targetPath,
		Logger: logger,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("open storage file: %w", err).Error())
		return
	}

	cfg := config.LoadUserConfig()
	cfg.DBName = dbName
	cfg.SetupComplete = true
	if err := config.SaveUserConfig(cfg, true); err != nil {
		if closeErr := newCore.Close(); closeErr != nil {
			logger.Error("failed to close new core after config save error", "err", closeErr)
		}
		writeError(w, http.StatusInternalServerError, fmt.Errorf("save config: %w", err).Error())
		return
	}

	h.coreMu.Lock()
	oldCore := h.core
	h.core = newCore
	h.coreMu.Unlock()

	if oldCore != nil {
		if closeErr := oldCore.Close(); closeErr != nil {
			logger.Error("failed to close old core after storage switch", "err", closeErr)
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "switched", "db_name": dbName})
}

func sanitizeDBName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", errors.New("storage file name is required")
	}
	if strings.ContainsAny(name, `/\`) {
		return "", errors.New("storage file name must not include a path")
	}
	name = filepath.Base(name)
	if name == "." || name == ".." {
		return "", errors.New("invalid storage file name")
	}
	if !strings.HasSuffix(strings.ToLower(name), ".db") {
		name += ".db"
	}
	return name, nil
}

func listDBFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.EqualFold(filepath.Ext(name), ".db") {
			files = append(files, name)
		}
	}
	sort.Strings(files)
	return files, nil
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
