package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type UserConfig struct {
	DBName        string `json:"db_name"`
	UseICloud     bool   `json:"use_icloud"`
	DataDir       string `json:"data_dir"`
	SetupComplete bool   `json:"setup_complete"`
}

var runtimeDataDir string
var runtimePort = 8000

func IsMacOS() bool {
	return runtime.GOOS == "darwin"
}

func IsWindows() bool {
	return runtime.GOOS == "windows"
}

func SetRuntimeDataDir(dir string) {
	runtimeDataDir = dir
}

func SetRuntimePort(port int) {
	if port > 0 {
		runtimePort = port
	}
}

func GetRuntimePort() int {
	return runtimePort
}

func userHomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return home, nil
}

func appConfigDir() (string, error) {
	if IsMacOS() {
		home, err := userHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "InvestLog"), nil
	}
	if IsWindows() {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			home, err := userHomeDir()
			if err != nil {
				return "", err
			}
			appData = home
		}
		return filepath.Join(appData, "InvestLog"), nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		home, err := userHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".config", "investlog"), nil
	}
	return filepath.Join(configDir, "investlog"), nil
}

func appConfigPath() (string, error) {
	dir, err := appConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func legacyConfigPath() string {
	if cwd, err := os.Getwd(); err == nil {
		candidate := filepath.Join(cwd, "config.json")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "config.json")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func IsFirstRun() bool {
	path, err := appConfigPath()
	if err != nil {
		return true
	}
	_, err = os.Stat(path)
	return err != nil
}

func IsICloudAvailable() bool {
	if !IsMacOS() {
		return false
	}
	base := iCloudBasePath()
	if base == "" {
		return false
	}
	_, err := os.Stat(base)
	return err == nil
}

func iCloudBasePath() string {
	home, err := userHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs")
}

func GetICloudAppFolder() string {
	base := iCloudBasePath()
	if base == "" {
		return ""
	}
	return filepath.Join(base, "InvestLog")
}

func LoadUserConfig() UserConfig {
	defaults := UserConfig{
		DBName:        "transactions.db",
		UseICloud:     true,
		DataDir:       "",
		SetupComplete: false,
	}
	configPath, err := appConfigPath()
	if err != nil {
		return defaults
	}
	pathToUse := ""
	if _, err := os.Stat(configPath); err == nil {
		pathToUse = configPath
	} else if legacy := legacyConfigPath(); legacy != "" {
		pathToUse = legacy
	}
	if pathToUse == "" {
		return defaults
	}
	file, err := os.Open(pathToUse)
	if err != nil {
		return defaults
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&defaults); err != nil {
		return defaults
	}
	if defaults.DBName == "" {
		defaults.DBName = "transactions.db"
	}
	return defaults
}

func SaveUserConfig(cfg UserConfig, useAppConfig bool) error {
	path := ""
	if useAppConfig {
		appPath, err := appConfigPath()
		if err != nil {
			return err
		}
		path = appPath
	} else {
		path = legacyConfigPath()
		if path == "" {
			if cwd, err := os.Getwd(); err == nil {
				path = filepath.Join(cwd, "config.json")
			} else {
				return errors.New("cannot determine config path")
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, srcFile); err != nil {
		return err
	}
	return out.Sync()
}

func CompleteSetup(useICloud bool, customDataDir, existingDBPath, dbName string) (string, error) {
	cfg := LoadUserConfig()
	selectedName := stringsTrim(dbName)
	if selectedName == "" {
		selectedName = cfg.DBName
	}
	if selectedName == "" {
		selectedName = "transactions.db"
	}

	if existingDBPath != "" {
		existingDBPath = filepath.Clean(existingDBPath)
		info, err := os.Stat(existingDBPath)
		if err != nil {
			return "", err
		}
		if info.IsDir() {
			return "", fmt.Errorf("database path is a directory")
		}

		selectedName = filepath.Base(existingDBPath)
		switch {
		case useICloud && IsMacOS():
			if !IsICloudAvailable() {
				return "", errors.New("iCloud Drive not available")
			}
			targetDir := GetICloudAppFolder()
			if targetDir == "" {
				return "", errors.New("iCloud path unavailable")
			}
			targetPath := filepath.Join(targetDir, selectedName)
			if err := copyFile(existingDBPath, targetPath); err != nil {
				return "", err
			}
			cfg.UseICloud = true
			cfg.DataDir = ""
			cfg.DBName = selectedName
			cfg.SetupComplete = true
			if err := SaveUserConfig(cfg, true); err != nil {
				return "", err
			}
			return targetDir, nil
		case customDataDir != "":
			targetDir := filepath.Clean(customDataDir)
			if err := os.MkdirAll(targetDir, 0o755); err != nil {
				return "", err
			}
			targetPath := filepath.Join(targetDir, selectedName)
			if err := copyFile(existingDBPath, targetPath); err != nil {
				return "", err
			}
			cfg.UseICloud = false
			cfg.DataDir = targetDir
			cfg.DBName = selectedName
			cfg.SetupComplete = true
			if err := SaveUserConfig(cfg, true); err != nil {
				return "", err
			}
			return targetDir, nil
		default:
			dir := filepath.Dir(existingDBPath)
			cfg.UseICloud = false
			cfg.DataDir = dir
			cfg.DBName = selectedName
			cfg.SetupComplete = true
			if err := SaveUserConfig(cfg, true); err != nil {
				return "", err
			}
			return dir, nil
		}
	}

	if useICloud && IsMacOS() {
		if !IsICloudAvailable() {
			return "", errors.New("iCloud Drive not available")
		}
		dataDir := GetICloudAppFolder()
		if dataDir == "" {
			return "", errors.New("iCloud path unavailable")
		}
		if err := os.MkdirAll(dataDir, 0o755); err != nil {
			return "", err
		}
		cfg.UseICloud = true
		cfg.DataDir = ""
		cfg.DBName = selectedName
		cfg.SetupComplete = true
		if err := SaveUserConfig(cfg, true); err != nil {
			return "", err
		}
		return dataDir, nil
	}

	if customDataDir != "" {
		dataDir := filepath.Clean(customDataDir)
		if err := os.MkdirAll(dataDir, 0o755); err != nil {
			return "", err
		}
		cfg.UseICloud = false
		cfg.DataDir = dataDir
		cfg.DBName = selectedName
		cfg.SetupComplete = true
		if err := SaveUserConfig(cfg, true); err != nil {
			return "", err
		}
		return dataDir, nil
	}

	defaultDir, err := appConfigDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(defaultDir, 0o755); err != nil {
		return "", err
	}
	cfg.UseICloud = false
	cfg.DataDir = defaultDir
	cfg.DBName = selectedName
	cfg.SetupComplete = true
	if err := SaveUserConfig(cfg, true); err != nil {
		return "", err
	}
	return defaultDir, nil
}

func GetDataDir() (string, error) {
	if runtimeDataDir != "" {
		if err := os.MkdirAll(runtimeDataDir, 0o755); err != nil {
			return "", err
		}
		return runtimeDataDir, nil
	}
	if envDir := os.Getenv("INVEST_LOG_DATA_DIR"); envDir != "" {
		if err := os.MkdirAll(envDir, 0o755); err != nil {
			return "", err
		}
		return envDir, nil
	}
	cfg := LoadUserConfig()
	if cfg.DataDir != "" {
		if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
			return "", err
		}
		return cfg.DataDir, nil
	}
	if cfg.UseICloud && IsMacOS() && IsICloudAvailable() {
		icloudDir := GetICloudAppFolder()
		if icloudDir == "" {
			return "", errors.New("iCloud path unavailable")
		}
		if err := os.MkdirAll(icloudDir, 0o755); err != nil {
			return "", err
		}
		return icloudDir, nil
	}
	defaultDir, err := appConfigDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(defaultDir, 0o755); err != nil {
		return "", err
	}
	return defaultDir, nil
}

func GetDBPath() (string, error) {
	if envPath := os.Getenv("INVEST_LOG_DB_PATH"); envPath != "" {
		return envPath, nil
	}
	cfg := LoadUserConfig()
	dataDir, err := GetDataDir()
	if err != nil {
		return "", err
	}
	name := cfg.DBName
	if name == "" {
		name = "transactions.db"
	}
	return filepath.Join(dataDir, name), nil
}

func stringsTrim(value string) string {
	return strings.TrimSpace(value)
}
