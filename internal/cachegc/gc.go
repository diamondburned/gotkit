package cachegc

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/errors"
)

const gcPeriod = 10 * time.Minute

var (
	gcs  = map[string]*cacheGC{}
	gcMu sync.Mutex
)

// Do runs the garbage collector on the given path asynchronously. All files
// older than age will be cleared.
func Do(path string, age time.Duration) {
	gcMu.Lock()

	gc, ok := gcs[path]
	if !ok {
		gc = &cacheGC{}
		gcs[path] = gc
	}

	gcMu.Unlock()

	gc.do(path, age)
}

type cacheGC struct {
	mut     sync.Mutex
	lastRun time.Time
	running bool
}

// do runs the GC asynchronously.
func (c *cacheGC) do(path string, age time.Duration) {
	now := time.Now()

	// Only run the GC after the set period and once the previous GC job is
	// done.
	c.mut.Lock()
	if c.running || c.lastRun.Add(gcPeriod).After(now) {
		c.mut.Unlock()
		return
	}
	c.running = true
	c.lastRun = now
	c.mut.Unlock()

	go func() {
		files, _ := os.ReadDir(path)

		for _, file := range files {
			s, err := file.Info()
			if err != nil {
				continue
			}

			if s.ModTime().Add(age).Before(now) {
				// Outdated.
				os.Remove(filepath.Join(path, file.Name()))
			}
		}

		c.mut.Lock()
		c.running = false
		c.mut.Unlock()
	}()
}

// IsFile returns true if the given path exists as a file.
func IsFile(path string) bool {
	s, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !s.IsDir()
}

// cacheError is an error that wraps erros that happen during caching.
type cacheError struct {
	Err error
	Msg string
}

// Error implements error.
func (err cacheError) Error() string {
	return err.Msg + ": " + err.Err.Error()
}

func (err cacheError) Unwrap() error { return err.Err }

// IsCacheError returns true if the error happened while caching.
func IsCacheError(err error) bool {
	var cacheErr cacheError
	return errors.As(err, &cacheErr)
}

// WithTmp gives f a premade tmp file and moves it back atomically.
func WithTmp(dst, pattern string, fn func(path string) error) error {
	return WithTmpFile(dst, pattern, func(f *os.File) error {
		return fn(f.Name())
	})
}

// WithTmpFile gives f a premade tmp os.File and moves it back atomically.
func WithTmpFile(dst, pattern string, fn func(*os.File) error) error {
	if IsFile(dst) {
		return nil
	}

	dir := filepath.Dir(dst)

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return cacheError{err, "cannot mkdir -p"}
	}

	f, err := os.CreateTemp(dir, ".tmp."+pattern)
	if err != nil {
		return cacheError{err, "cannot mktemp"}
	}
	defer f.Close()
	defer os.Remove(f.Name())

	if err := fn(f); err != nil {
		return err
	}

	if err := os.Rename(f.Name(), dst); err != nil {
		return cacheError{err, "cannot rename temp file"}
	}

	return nil
}
