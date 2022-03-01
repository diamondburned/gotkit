// Package config provides configuration facilities.
package config

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// WriteFile writes b to the file in path atomically. It doesn't have to do with
// configs, but it is exported for convenience.
func WriteFile(path string, b []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return errors.Wrap(err, "cannot mkdir -p")
	}

	tmp, err := os.CreateTemp(dir, ".tmp.*")
	if err != nil {
		return errors.Wrap(err, "cannot mktemp")
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	if _, err := tmp.Write(b); err != nil {
		return errors.Wrap(err, "cannot write to temp file")
	}
	if err := tmp.Close(); err != nil {
		return errors.Wrap(err, "temp file error")
	}

	if err := os.Rename(tmp.Name(), path); err != nil {
		return errors.Wrap(err, "cannot swap new prefs file")
	}

	return nil
}
