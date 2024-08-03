// Package config provides configuration facilities.
package config

import (
	"github.com/diamondburned/gotkit/utils/osutil"
)

// WriteFile writes b to the file in path atomically. It doesn't have to do with
// configs, but it is exported for convenience.
func WriteFile(path string, b []byte) error {
	return osutil.WriteFile(path, b)
}
