package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const defaultPrefix = "invest-log"

// DailyWriter writes logs into a date-based file and prunes old files.
type DailyWriter struct {
	dir           string
	prefix        string
	retentionDays int
	mu            sync.Mutex
	currentDate   string
	file          *os.File
}

// NewDailyWriter creates a daily rotating writer in the provided directory.
func NewDailyWriter(dir string, retentionDays int) (*DailyWriter, error) {
	return NewDailyWriterWithPrefix(dir, defaultPrefix, retentionDays)
}

// NewDailyWriterWithPrefix creates a daily rotating writer with a custom prefix.
func NewDailyWriterWithPrefix(dir, prefix string, retentionDays int) (*DailyWriter, error) {
	if retentionDays <= 0 {
		retentionDays = 7
	}
	if prefix == "" {
		prefix = defaultPrefix
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	w := &DailyWriter{
		dir:           dir,
		prefix:        prefix,
		retentionDays: retentionDays,
	}
	if err := w.rotateIfNeeded(time.Now()); err != nil {
		return nil, err
	}
	return w, nil
}

// Write implements io.Writer.
func (w *DailyWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.rotateIfNeeded(time.Now()); err != nil {
		return 0, err
	}
	return w.file.Write(p)
}

// Close closes the underlying file.
func (w *DailyWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	return w.file.Close()
}

func (w *DailyWriter) rotateIfNeeded(now time.Time) error {
	date := now.Format("20060102")
	if date == w.currentDate && w.file != nil {
		return nil
	}
	if w.file != nil {
		_ = w.file.Close()
	}
	w.currentDate = date
	path := filepath.Join(w.dir, fmt.Sprintf("%s-%s.log", w.prefix, date))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	w.file = file
	w.cleanup(now)
	return nil
}

func (w *DailyWriter) cleanup(now time.Time) {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}
	cutoff := now.AddDate(0, 0, -w.retentionDays)
	prefix := w.prefix + "-"
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".log") {
			continue
		}
		datePart := strings.TrimSuffix(strings.TrimPrefix(name, prefix), ".log")
		if len(datePart) != 8 {
			continue
		}
		date, err := time.Parse("20060102", datePart)
		if err != nil {
			continue
		}
		if date.Before(cutoff) {
			_ = os.Remove(filepath.Join(w.dir, name))
		}
	}
}

// NewLogger creates a slog.Logger writing to stdout and a daily file.
func NewLogger(logDir string) (*slog.Logger, *DailyWriter, error) {
	writer, err := NewDailyWriter(logDir, 7)
	if err != nil {
		return nil, nil, err
	}
	multi := io.MultiWriter(os.Stdout, writer)
	handler := slog.NewTextHandler(multi, &slog.HandlerOptions{Level: slog.LevelInfo})
	return slog.New(handler), writer, nil
}
