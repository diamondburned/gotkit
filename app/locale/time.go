package locale

import (
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
)

// doubleSpaceCollider is used for some formatted timestamps to get rid of
// padding spaces.
var doubleSpaceCollider = strings.NewReplacer("  ", " ")

// Time formats the given timestamp as a locale-compatible timestamp.
func Time(t time.Time, long bool) string {
	glibTime := glib.NewDateTimeFromGo(t.Local())
	if long {
		return doubleSpaceCollider.Replace(glibTime.Format("%c"))
	}
	return glibTime.Format("%X")
}

const (
	Day  = 24 * time.Hour
	Week = 7 * Day
	Year = 365 * Day
)

type truncator struct {
	d time.Duration
	s string
}

var longTruncators = []truncator{
	{d: 1 * Day, s: "Today at %X"},
	{d: 2 * Day, s: "Yesterday at %X"},
	{d: Week, s: "%A at %X"},
	{s: "%X %x"},
}

// TimeAgo formats a long string that expresses the relative time difference
// from now until t.
func TimeAgo(t time.Time) string {
	t = t.Local()

	trunc := t
	now := time.Now().Local()

	for i, truncator := range longTruncators {
		trunc = trunc.Truncate(truncator.d)
		now = now.Truncate(truncator.d)

		if trunc.Equal(now) || i == len(longTruncators)-1 {
			glibTime := glib.NewDateTimeFromGo(t)
			return glibTime.Format(GetFromDomain("gotkit", truncator.s))
		}
	}

	panic("unreachable")
}
