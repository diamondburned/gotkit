// Package aggressivegc enforces a GC every minute. The user shouldn't need this
// in most applications.
//
// Use it like so:
//
//    import _ "github.com/diamondburned/gotkit/gtkutil/aggressivegc"
//
package aggressivegc

import (
	"runtime"
	"time"
)

func init() {
	go func() {
		for range time.Tick(time.Minute) {
			runtime.GC()
		}
	}()
}
