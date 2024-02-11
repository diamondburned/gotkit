package logui

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/diamondburned/gotk4/pkg/core/gioutil"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
)

var hookFunc = sync.OnceFunc(func() {
	stderrHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	handler := MultiHandler(
		stderrHandler,
		DefaultLogHandler(),
	)
	slog.SetDefault(slog.New(handler))
})

func init() {
	// Hook the slog handler only once the main loop is running.
	// Otherwise, the logging is useless.
	coreglib.IdleAddPriority(coreglib.PriorityHigh, func() {
		hookFunc()
	})
}

// Hook hooks the default slog handler to the default log handler.
// This function is called automatically after the main loop starts, but you may
// call this earlier if you want to log before the main loop starts.
func Hook() {
	hookFunc()
}

// RecordsToString returns a string representation of the given log records.
func RecordsToString(iter func(yield func(slog.Record) bool)) string {
	var text strings.Builder

	h := slog.NewTextHandler(&text, &slog.HandlerOptions{Level: slog.LevelDebug})
	iter(func(record slog.Record) bool {
		h.Handle(context.Background(), record)
		return true
	})

	return text.String()
}

// DefaultMaxEntries is the default maximum number of log entries.
const DefaultMaxEntries = 1000

type LogListModel = gioutil.ListModel[slog.Record]

var LogListModelType = gioutil.NewListModelType[slog.Record]()

// LogHandler is a slog.Handler that stores logs in a list model.
// To obtain the list model, use the [ListModel] method.
type LogHandler struct {
	level *atomic.Pointer[slog.Leveler]
	list  *LogListModel

	attrs  []slog.Attr
	groups string
	max    int
}

var _ slog.Handler = (*LogHandler)(nil)

// NewLogHandler creates a new LogHandler with the given options.
// If maxEntries is 0, then the list model will have no limit.
func NewLogHandler(maxEntries int, opts *slog.HandlerOptions) *LogHandler {
	h := &LogHandler{
		level: new(atomic.Pointer[slog.Leveler]),
		list:  LogListModelType.New(),
		max:   maxEntries,
	}
	h.level.Store(&opts.Level)
	return h
}

// NewDebugLogHandler creates a new LogHandler that logs all levels, including debug.
func NewDebugLogHandler(maxEntries int) *LogHandler {
	return NewLogHandler(maxEntries, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
}

var (
	defaultOnce    sync.Once
	defaultHandler *LogHandler
)

// DefaultLogHandler returns the default log handler.
// Debug logs are enabled by default.
func DefaultLogHandler() *LogHandler {
	defaultOnce.Do(func() { defaultHandler = NewDebugLogHandler(DefaultMaxEntries) })
	return defaultHandler
}

// SetDefaultLevel sets the default level for the default log handler.
func SetDefaultLevel(level slog.Leveler) {
	h := DefaultLogHandler()
	h.SetLevel(level)
}

// ListModel returns the list model that contains the logs.
// Only the main thread should access this list model.
func (h *LogHandler) ListModel() *LogListModel {
	return h.list
}

// Level returns the current level of the handler.
// This method is thread-safe.
func (h *LogHandler) Level() slog.Leveler {
	return *h.level.Load()
}

// SetLevel sets the level of the handler.
// This method is thread-safe.
func (h *LogHandler) SetLevel(level slog.Leveler) {
	h.level.Store(&level)
}

func (h *LogHandler) clone() *LogHandler {
	return &LogHandler{
		level:  h.level,
		list:   h.list,
		attrs:  append([]slog.Attr{}, h.attrs...),
		groups: h.groups,
		max:    h.max,
	}
}

func (h *LogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.Level().Level()
}

func (h *LogHandler) Handle(_ context.Context, record slog.Record) error {
	record = record.Clone()
	record.AddAttrs(h.attrs...)

	coreglib.IdleAdd(func() {
		h.list.Append(record)

		if h.max > 0 {
			n := h.list.NItems()
			if n > h.max {
				h.list.Splice(0, n-h.max)
			}
		}
	})

	return nil
}

func (h *LogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h = h.clone()
	for _, attr := range attrs {
		h.attrs = append(h.attrs, slog.Attr{
			Key:   joinGroups(h.groups, attr.Key),
			Value: attr.Value,
		})
	}
	return h
}

func (h *LogHandler) WithGroup(name string) slog.Handler {
	h = h.clone()
	h.groups = joinGroups(h.groups, name)
	return h
}

func joinGroups(base string, tail string) string {
	if base == "" {
		return tail
	}
	return base + "." + tail
}
