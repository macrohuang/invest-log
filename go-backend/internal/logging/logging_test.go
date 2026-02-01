package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDailyWriterWriteAndDefaults(t *testing.T) {
	dir := t.TempDir()
	writer, err := NewDailyWriterWithPrefix(dir, "", 0)
	if err != nil {
		t.Fatalf("NewDailyWriterWithPrefix: %v", err)
	}
	defer writer.Close()

	_, err = writer.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	date := time.Now().Format("20060102")
	path := filepath.Join(dir, defaultPrefix+"-"+date+".log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(data), "hello") {
		t.Fatalf("log content missing")
	}
}

func TestDailyWriterRotateAndCleanup(t *testing.T) {
	dir := t.TempDir()
	prefix := "test"
	retention := 1

	// Create an old log file and a recent one.
	oldDate := time.Now().AddDate(0, 0, -3).Format("20060102")
	recentDate := time.Now().Format("20060102")
	oldPath := filepath.Join(dir, prefix+"-"+oldDate+".log")
	recentPath := filepath.Join(dir, prefix+"-"+recentDate+".log")
	if err := os.WriteFile(oldPath, []byte("old"), 0o644); err != nil {
		t.Fatalf("write old: %v", err)
	}
	if err := os.WriteFile(recentPath, []byte("recent"), 0o644); err != nil {
		t.Fatalf("write recent: %v", err)
	}

	writer, err := NewDailyWriterWithPrefix(dir, prefix, retention)
	if err != nil {
		t.Fatalf("NewDailyWriterWithPrefix: %v", err)
	}
	defer writer.Close()

	if err := writer.rotateIfNeeded(time.Now()); err != nil {
		t.Fatalf("rotateIfNeeded: %v", err)
	}

	if _, err := os.Stat(oldPath); err == nil {
		t.Fatalf("expected old log to be removed")
	}
	if _, err := os.Stat(recentPath); err != nil {
		t.Fatalf("expected recent log to remain: %v", err)
	}
}

func TestDailyWriterCloseNil(t *testing.T) {
	w := &DailyWriter{}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestNewLogger(t *testing.T) {
	dir := t.TempDir()
	logger, writer, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	if logger == nil || writer == nil {
		t.Fatalf("expected logger and writer")
	}
	_ = writer.Close()
}
