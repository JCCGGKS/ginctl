package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const dateLayout = "2006-01-02"

// dailyWriter writes one module's encoded stream to
// <rootDir>/<YYYY-MM-DD>/<module>.log and switches files when the local date
// changes. A mutex is required because each zap core may be used concurrently.
type dailyWriter struct {
	rootDir string
	module  string

	mu   sync.Mutex
	day  string
	file *os.File
}

func newDailyWriter(rootDir, module string, now time.Time) (*dailyWriter, error) {
	w := &dailyWriter{rootDir: rootDir, module: module}
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.open(now); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *dailyWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	if w.file == nil || w.day != now.Format(dateLayout) {
		if err := w.open(now); err != nil {
			return 0, err
		}
	}
	return w.file.Write(p)
}

func (w *dailyWriter) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	return w.file.Sync()
}

func (w *dailyWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func (w *dailyWriter) open(now time.Time) error {
	day := now.Format(dateLayout)
	dir := filepath.Join(w.rootDir, day)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create log directory %q: %w", dir, err)
	}

	path := filepath.Join(dir, w.module+".log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log file %q: %w", path, err)
	}

	if w.file != nil {
		_ = w.file.Close()
	}
	w.file = file
	w.day = day
	return nil
}
