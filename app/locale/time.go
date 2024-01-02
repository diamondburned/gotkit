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

const (
	relativeToday     = "Today at %X"
	relativeYesterday = "Yesterday at %X"
	relativeWeek      = "%A at %X"
	relativeDefault   = "%X %x"
)

// TimeAgo formats a long string that expresses the relative time difference
// from now until t.
func TimeAgo(timestamp time.Time) string {
	timestamp = timestamp.Local()

	tts := timestamp
	now := time.Now().Local()

	ttsDay := truncateDay(tts)
	nowDay := truncateDay(now)
	if ttsDay.Equal(nowDay) {
		return renderTime(timestamp, relativeToday)
	}

	if ttsDay.Equal(truncateDay(now.Add(-Day))) {
		return renderTime(timestamp, relativeYesterday)
	}

	ttsWeek := truncateWeek(tts)
	nowWeek := truncateWeek(now)
	if ttsWeek.Equal(nowWeek) {
		return renderTime(timestamp, relativeWeek)
	}

	return renderTime(timestamp, relativeDefault)
}

func renderTime(t time.Time, f string) string {
	glibTime := glib.NewDateTimeFromGo(t)
	return glibTime.Format(GetFromDomain("gotkit", f))
}

// truncateDay truncates the given time to the given day.
// It differs from time.Truncate in that it truncates to the start of the day
// according to the local timezone.
func truncateDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// truncateWeek truncates the given time to the given week.
// It differs from time.Truncate in that it truncates to the start of the week
// according to the local timezone.
func truncateWeek(t time.Time) time.Time {
	y, m, d := t.Date()
	weekday := int(t.Weekday())
	return time.Date(y, m, d-weekday, 0, 0, 0, 0, t.Location())
}
