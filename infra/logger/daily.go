package logger

import (
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const dateLayout = "2006-01-02"

type dailyWriterOptions struct {
	location     *time.Location
	dailyRotate  bool
	maxSizeBytes int64
	maxAgeDays   int
	compress     bool
}

// dailyWriter writes one module's encoded stream either to date-based files or
// to fixed files. Daily rotation uses
// <rootDir>/<YYYY-MM>/<YYYY-MM-DD>_<module>.log and switches when the configured
// timezone's local date changes. Fixed mode uses <rootDir>/<module>.log. In
// both modes, a positive maxSizeBytes splits an oversized file into numbered
// parts. A mutex is required because each zap core may be used concurrently.
type dailyWriter struct {
	rootDir      string
	module       string
	location     *time.Location
	dailyRotate  bool
	maxSizeBytes int64
	maxAgeDays   int
	compress     bool

	mu   sync.Mutex
	day  string
	part int
	size int64
	file *os.File

	maintenanceWG sync.WaitGroup
	maintaining   atomic.Bool
}

func newDailyWriter(rootDir, module string, now time.Time, opts dailyWriterOptions) (*dailyWriter, error) {
	if opts.location == nil {
		opts.location = time.Local
	}
	w := &dailyWriter{
		rootDir:      rootDir,
		module:       module,
		location:     opts.location,
		dailyRotate:  opts.dailyRotate,
		maxSizeBytes: opts.maxSizeBytes,
		maxAgeDays:   opts.maxAgeDays,
		compress:     opts.compress,
	}

	w.mu.Lock()
	err := w.open(now)
	w.mu.Unlock()
	if err != nil {
		return nil, err
	}

	w.startMaintenance(now)
	return w, nil
}

func (w *dailyWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now().In(w.location)
	if w.dailyRotate {
		today := now.Format(dateLayout)
		if w.file == nil || w.day != today {
			previousDay := w.day
			if err := w.openDaily(today); err != nil {
				return 0, err
			}
			if previousDay != "" {
				w.startMaintenance(now)
			}
		}

		if w.maxSizeBytes > 0 && w.size > 0 && w.size+int64(len(p)) > w.maxSizeBytes {
			if err := w.openDailyPart(today, w.part+1); err != nil {
				return 0, err
			}
		}
	} else {
		if w.file == nil {
			if err := w.openFixed(); err != nil {
				return 0, err
			}
		}
		if w.maxSizeBytes > 0 && w.size > 0 && w.size+int64(len(p)) > w.maxSizeBytes {
			if err := w.openFixedPart(w.part + 1); err != nil {
				return 0, err
			}
			w.startMaintenance(now)
		}
	}

	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
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
	w.maintenanceWG.Wait()

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
	if !w.dailyRotate {
		return w.openFixed()
	}
	return w.openDaily(now.In(w.location).Format(dateLayout))
}

func (w *dailyWriter) openDaily(day string) error {
	part, err := w.latestDailyPart(day)
	if err != nil {
		return err
	}
	return w.openDailyPart(day, part)
}

func (w *dailyWriter) openDailyPart(day string, part int) error {
	dir := filepath.Join(w.rootDir, day[:7])
	return w.openPath(filepath.Join(dir, logFileName(day, w.module, part)), day, part)
}

func (w *dailyWriter) openFixed() error {
	part, err := w.latestFixedPart()
	if err != nil {
		return err
	}
	return w.openFixedPart(part)
}

func (w *dailyWriter) openFixedPart(part int) error {
	return w.openPath(filepath.Join(w.rootDir, fixedFileName(w.module, part)), "", part)
}

func (w *dailyWriter) openPath(path, day string, part int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create log directory %q: %w", filepath.Dir(path), err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log file %q: %w", path, err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return fmt.Errorf("stat log file %q: %w", path, err)
	}

	previous := w.file
	w.file = file
	w.day = day
	w.part = part
	w.size = info.Size()
	if previous != nil {
		_ = previous.Close()
	}
	return nil
}

func (w *dailyWriter) latestDailyPart(day string) (int, error) {
	dir := filepath.Join(w.rootDir, day[:7])
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read log directory %q: %w", dir, err)
	}

	prefix := day + "_" + w.module
	latest := -1
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		part, ok := parsePart(entry.Name(), prefix)
		if ok && part > latest {
			latest = part
		}
	}
	if latest < 0 {
		return 0, nil
	}

	info, err := os.Stat(filepath.Join(dir, logFileName(day, w.module, latest)))
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if w.maxSizeBytes > 0 && info.Size() >= w.maxSizeBytes {
		return latest + 1, nil
	}
	return latest, nil
}

func (w *dailyWriter) latestFixedPart() (int, error) {
	entries, err := os.ReadDir(w.rootDir)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read log directory %q: %w", w.rootDir, err)
	}

	latest := -1
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		part, ok := parseFixedPart(entry.Name(), w.module)
		if ok && part > latest {
			latest = part
		}
	}
	if latest < 0 {
		return 0, nil
	}

	info, err := os.Stat(filepath.Join(w.rootDir, fixedFileName(w.module, latest)))
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if w.maxSizeBytes > 0 && info.Size() >= w.maxSizeBytes {
		return latest + 1, nil
	}
	return latest, nil
}

func parsePart(name, prefix string) (int, bool) {
	if name == prefix+".log" {
		return 0, true
	}
	rest, ok := strings.CutPrefix(name, prefix+".")
	if !ok || !strings.HasSuffix(rest, ".log") {
		return 0, false
	}
	digits := strings.TrimSuffix(rest, ".log")
	part, err := strconv.Atoi(digits)
	if err != nil || part < 1 {
		return 0, false
	}
	return part, true
}

func logFileName(day, module string, part int) string {
	base := day + "_" + module
	if part == 0 {
		return base + ".log"
	}
	return base + "." + strconv.Itoa(part) + ".log"
}

func parseFixedPart(name, module string) (int, bool) {
	if name == module+".log" {
		return 0, true
	}
	rest, ok := strings.CutPrefix(name, module+".")
	if !ok || !strings.HasSuffix(rest, ".log") {
		return 0, false
	}
	digits := strings.TrimSuffix(rest, ".log")
	part, err := strconv.Atoi(digits)
	if err != nil || part < 1 {
		return 0, false
	}
	return part, true
}

func fixedFileName(module string, part int) string {
	if part == 0 {
		return module + ".log"
	}
	return module + "." + strconv.Itoa(part) + ".log"
}

func (w *dailyWriter) startMaintenance(now time.Time) {
	if w.maxAgeDays <= 0 && !w.compress {
		return
	}
	if !w.maintaining.CompareAndSwap(false, true) {
		return
	}

	w.maintenanceWG.Add(1)
	go func() {
		defer w.maintenanceWG.Done()
		defer w.maintaining.Store(false)
		w.maintain(now)
	}()
}

func (w *dailyWriter) maintain(now time.Time) {
	if !w.dailyRotate {
		w.maintainFixed(now)
		return
	}
	w.maintainDaily(now)
}

func (w *dailyWriter) maintainDaily(now time.Time) {
	today := startOfDay(now, w.location)
	cutoff := today.AddDate(0, 0, -w.maxAgeDays)

	err := filepath.WalkDir(w.rootDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}

		fileDate, compressed, ok := w.matchFile(entry.Name())
		if !ok {
			return nil
		}
		if w.maxAgeDays > 0 && fileDate.Before(cutoff) {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				reportMaintenanceError(path, err)
			}
			return nil
		}
		if w.compress && !compressed && fileDate.Before(today) {
			if err := gzipFile(path); err != nil {
				reportMaintenanceError(path, err)
			}
		}
		return nil
	})
	if err != nil {
		reportMaintenanceError(w.rootDir, err)
	}
}

func (w *dailyWriter) maintainFixed(now time.Time) {
	today := startOfDay(now, w.location)
	cutoff := today.AddDate(0, 0, -w.maxAgeDays)
	active := filepath.Clean(w.activePath())

	entries, err := os.ReadDir(w.rootDir)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		reportMaintenanceError(w.rootDir, err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		_, compressed, ok := matchFixedFile(entry.Name(), w.module)
		if !ok {
			continue
		}

		path := filepath.Join(w.rootDir, entry.Name())
		if filepath.Clean(path) == active {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			reportMaintenanceError(path, err)
			continue
		}
		fileDate := startOfDay(info.ModTime(), w.location)
		if w.maxAgeDays > 0 && fileDate.Before(cutoff) {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				reportMaintenanceError(path, err)
			}
			continue
		}
		if w.compress && !compressed && fileDate.Before(today) {
			if err := gzipFile(path); err != nil {
				reportMaintenanceError(path, err)
			}
		}
	}
}

func (w *dailyWriter) activePath() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return ""
	}
	return w.file.Name()
}

func matchFixedFile(name, module string) (int, bool, bool) {
	compressed := strings.HasSuffix(name, ".gz")
	base := strings.TrimSuffix(name, ".gz")
	part, ok := parseFixedPart(base, module)
	if !ok {
		return 0, false, false
	}
	return part, compressed, true
}

func (w *dailyWriter) matchFile(name string) (time.Time, bool, bool) {
	if len(name) < len(dateLayout)+len(w.module)+5 {
		return time.Time{}, false, false
	}
	fileDate, err := time.ParseInLocation(dateLayout, name[:len(dateLayout)], w.location)
	if err != nil {
		return time.Time{}, false, false
	}

	rest, ok := strings.CutPrefix(name[len(dateLayout):], "_"+w.module)
	if !ok {
		return time.Time{}, false, false
	}
	compressed := false
	if strings.HasSuffix(rest, ".gz") {
		compressed = true
		rest = strings.TrimSuffix(rest, ".gz")
	}
	if rest == ".log" {
		return fileDate, compressed, true
	}
	if strings.HasPrefix(rest, ".") && strings.HasSuffix(rest, ".log") {
		digits := strings.TrimSuffix(strings.TrimPrefix(rest, "."), ".log")
		if _, err := strconv.Atoi(digits); err == nil {
			return fileDate, compressed, true
		}
	}
	return time.Time{}, false, false
}

func gzipFile(path string) error {
	gzPath := path + ".gz"
	if _, err := os.Stat(gzPath); err == nil {
		return os.Remove(path)
	} else if !os.IsNotExist(err) {
		return err
	}

	src, err := os.Open(path)
	if err != nil {
		return err
	}

	info, err := src.Stat()
	if err != nil {
		_ = src.Close()
		return err
	}

	tmpPath := gzPath + ".tmp"
	dst, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		_ = src.Close()
		return err
	}

	gz := gzip.NewWriter(dst)
	_, copyErr := io.Copy(gz, src)
	gzipErr := gz.Close()
	closeErr := dst.Close()
	sourceErr := src.Close()
	if err := firstError(copyErr, gzipErr, closeErr, sourceErr); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Chtimes(tmpPath, info.ModTime(), info.ModTime()); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, gzPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Remove(path)
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func startOfDay(now time.Time, location *time.Location) time.Time {
	local := now.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}

func reportMaintenanceError(path string, err error) {
	fmt.Fprintf(os.Stderr, "logger maintenance %s: %v\n", path, err)
}
