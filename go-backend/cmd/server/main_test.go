package main

import (
	"flag"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func TestDirExists(t *testing.T) {
	dir := t.TempDir()
	if !dirExists(dir) {
		t.Fatalf("expected dir to exist")
	}
	file := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if dirExists(file) {
		t.Fatalf("expected file to not be dir")
	}
	if dirExists(filepath.Join(dir, "missing")) {
		t.Fatalf("expected missing path to be false")
	}
}

func TestResolveWebDir(t *testing.T) {
	tmp := t.TempDir()
	staticDir := filepath.Join(tmp, "static")
	if err := os.MkdirAll(staticDir, 0o755); err != nil {
		t.Fatalf("mkdir static: %v", err)
	}

	if got := resolveWebDir(staticDir); got != staticDir {
		t.Fatalf("expected input dir, got %q", got)
	}
	if got := resolveWebDir(filepath.Join(tmp, "missing")); got != "" {
		t.Fatalf("expected empty for missing, got %q", got)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(cwd)
	}()

	if got := resolveWebDir(""); got != "static" {
		t.Fatalf("expected static, got %q", got)
	}
}

func TestWatchParentExits(t *testing.T) {
	origGetppid := getppid
	origSleep := sleep
	origExit := exit
	defer func() {
		getppid = origGetppid
		sleep = origSleep
		exit = origExit
	}()

	getppid = func() int { return 1 }
	sleep = func(time.Duration) {}

	done := make(chan struct{})
	exit = func(code int) {
		close(done)
		runtime.Goexit()
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	go watchParent(logger)

	select {
	case <-done:
		// ok
	case <-time.After(1 * time.Second):
		t.Fatalf("watchParent did not exit")
	}
}

func TestMainLifecycle(t *testing.T) {
	tmp := t.TempDir()

	origArgs := os.Args
	origCommandLine := flag.CommandLine
	defer func() {
		os.Args = origArgs
		flag.CommandLine = origCommandLine
	}()

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = []string{
		"server",
		"--data-dir", tmp,
		"--port", "0",
		"--host", "127.0.0.1",
	}

	done := make(chan struct{})
	go func() {
		time.Sleep(150 * time.Millisecond)
		if p, err := os.FindProcess(os.Getpid()); err == nil {
			_ = p.Signal(syscall.SIGTERM)
		}
	}()

	go func() {
		main()
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(3 * time.Second):
		t.Fatalf("main did not exit")
	}
}
