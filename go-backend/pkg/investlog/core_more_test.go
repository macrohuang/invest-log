package investlog

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenWithOptionsAndDBPath(t *testing.T) {
	if _, err := OpenWithOptions(Options{}); err == nil {
		t.Fatalf("expected error for empty DBPath")
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.db")
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	core, err := OpenWithOptions(Options{DBPath: path, Logger: logger})
	if err != nil {
		t.Fatalf("OpenWithOptions: %v", err)
	}
	defer core.Close()

	if core.DBPath() != filepath.Clean(path) {
		t.Fatalf("DBPath mismatch: %q", core.DBPath())
	}
}

func TestDefaultHelpers(t *testing.T) {
	if got := defaultDuration(0, 5*time.Second); got != 5*time.Second {
		t.Fatalf("defaultDuration fallback: %v", got)
	}
	if got := defaultDuration(2*time.Second, 5*time.Second); got != 2*time.Second {
		t.Fatalf("defaultDuration value: %v", got)
	}
	if got := defaultInt(0, 7); got != 7 {
		t.Fatalf("defaultInt fallback: %d", got)
	}
	if got := defaultInt(3, 7); got != 3 {
		t.Fatalf("defaultInt value: %d", got)
	}
}
