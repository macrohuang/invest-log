package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"investlog/internal/api"
	"investlog/internal/config"
	"investlog/internal/logging"
	"investlog/pkg/investlog"
)

// buildMode is set via -ldflags at build time. Default "dev" enables debug logging.
// Build with: go build -ldflags "-X main.buildMode=release" ./cmd/server
var buildMode = "dev"

var getppid = os.Getppid
var sleep = time.Sleep
var exit = os.Exit

func main() {
	var dataDir string
	var port int
	var host string
	var webDir string
	var debug bool

	flag.StringVar(&dataDir, "data-dir", "", "Directory for storing database and application data")
	flag.IntVar(&port, "port", 8000, "Port to run the server on")
	flag.StringVar(&host, "host", "127.0.0.1", "Host to bind the server to")
	flag.StringVar(&webDir, "web-dir", "", "Directory for SPA static files (optional)")
	flag.BoolVar(&debug, "debug", false, "Enable debug logging (overrides build mode)")
	flag.Parse()

	if dataDir != "" {
		config.SetRuntimeDataDir(dataDir)
	}
	config.SetRuntimePort(port)

	resolvedDataDir, err := config.GetDataDir()
	if err != nil {
		slog.Error("failed to resolve data directory", "err", err)
		os.Exit(1)
	}
	logDir := filepath.Join(resolvedDataDir, "logs")
	logLevel := slog.LevelInfo
	if debug || buildMode == "dev" {
		logLevel = slog.LevelDebug
	}
	logger, writer, err := logging.NewLogger(logDir, logLevel)
	if err != nil {
		slog.Error("failed to initialize logger", "err", err)
		os.Exit(1)
	}
	logger.Info("logger initialized",
		"build_mode", buildMode,
		"log_level", logLevel.String(),
		"log_dir", logDir,
	)
	defer func() {
		if err := writer.Close(); err != nil {
			logger.Error("failed to close log writer", "err", err)
		}
	}()

	dbPath, err := config.GetDBPath()
	if err != nil {
		logger.Error("failed to resolve db path", "err", err)
		os.Exit(1)
	}

	core, err := investlog.OpenWithOptions(investlog.Options{DBPath: dbPath, Logger: logger})
	if err != nil {
		logger.Error("failed to initialize core", "err", err)
		os.Exit(1)
	}
	defer func() {
		if err := core.Close(); err != nil {
			logger.Error("failed to close core", "err", err)
		}
	}()

	if os.Getenv("INVEST_LOG_PARENT_WATCH") == "1" {
		logger.Info("parent watcher enabled")
		go watchParent(logger)
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	handler := api.NewRouter(core)
	if resolvedWebDir := resolveWebDir(webDir); resolvedWebDir != "" {
		logger.Info("serving SPA", "web_dir", resolvedWebDir)
		handler = api.WithSPA(handler, resolvedWebDir)
	}
	handler = middleware.Compress(5)(handler)

	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	logger.Info("server starting",
		"addr", addr,
		"host", host,
		"port", port,
		"pid", os.Getpid(),
		"data_dir", resolvedDataDir,
		"db_path", dbPath,
	)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "err", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	received := <-stop

	logger.Info("server shutting down", "signal", received.String())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", "err", err)
		return
	}
	logger.Info("server shutdown completed")
}

func watchParent(logger *slog.Logger) {
	for {
		sleep(1 * time.Second)
		ppid := getppid()
		if ppid == 1 {
			logger.Info("parent process exited; shutting down", "parent_pid", ppid)
			exit(0)
		}
	}
}

func resolveWebDir(input string) string {
	if input != "" {
		if dirExists(input) {
			return input
		}
		return ""
	}

	candidates := []string{"static", "../static"}
	for _, candidate := range candidates {
		if dirExists(candidate) {
			return candidate
		}
	}
	if exe, err := os.Executable(); err == nil {
		base := filepath.Dir(exe)
		for _, candidate := range candidates {
			path := filepath.Join(base, candidate)
			if dirExists(path) {
				return path
			}
		}
	}
	return ""
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
