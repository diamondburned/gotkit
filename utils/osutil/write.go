package osutil

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/pkg/errors"
)

// WriteFile writes b to the file in path atomically. It doesn't have to do with
// configs, but it is exported for convenience.
func WriteFile(path string, b []byte) error {
	return UseFile(path, func(f *os.File) error {
		_, err := f.Write(b)
		return err
	})
}

// preferFileLocking is a flag that determines whether to
// prefer file locking over temp files.
const preferFileLocking = runtime.GOOS == "windows"

// UseFile is a lower-level function that opens a file and calls fn with it. The
// file is closed after fn returns. The file may be a temporary file so that it
// can be atomically moved.
func UseFile(path string, fn func(*os.File) error) error {
	return UseFileWithPattern(path, ".tmp.*", fn)
}

var windowsFileLock sync.Mutex

// UseFileWithPattern is the same as UseFile, but it also takes a temporary file
// pattern. The pattern may not be used on all platforms.
func UseFileWithPattern(path, tmpPattern string, fn func(*os.File) error) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return errors.Wrap(err, "cannot mkdir -p")
	}

	if runtime.GOOS == "windows" {
		// Prefer slow lock, because flock is being weird on Windows.
		windowsFileLock.Lock()
		defer windowsFileLock.Unlock()

		// Windows doesn't have rename(2) semantics. We can only write directly
		// to the file.
		f, err := os.Create(path)
		if err != nil {
			return errors.Wrap(err, "cannot create dst file")
		}
		defer f.Close()

		if err := fn(f); err != nil {
			return err
		}
	} else {
		f, err := os.CreateTemp(dir, tmpPattern)
		if err != nil {
			return errors.Wrap(err, "cannot mktemp")
		}
		defer os.Remove(f.Name())
		defer f.Close()

		if err := fn(f); err != nil {
			return err
		}

		if err := f.Close(); err != nil {
			return errors.Wrap(err, "temp file error")
		}

		if err := os.Rename(f.Name(), path); err != nil {
			return errors.Wrap(err, "cannot swap new prefs file")
		}
	}

	return nil
}
