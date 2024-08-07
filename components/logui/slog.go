package logui

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/diamondburned/gotk4/pkg/core/gioutil"
	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
)

// WrapLogger wraps the given logger with the default logui's log handler.
// Call this function only once the main loop is running.
func WrapLogger(logger *slog.Logger) *slog.Logger {
	handler := MultiHandler(logger.Handler(), DefaultLogHandler())
	return slog.New(handler)
}

// SetLogger sets the default slog.Logger to the given logger.
// The logger is automatically wrapped with the default log handler.
// Call this function only once the main loop is running.
func SetLogger(logger *slog.Logger) {
	slog.SetDefault(WrapLogger(logger))
}

func init() {
	handler := MultiHandler(
		tint.NewHandler(os.Stderr, &tint.Options{
			TimeFormat: "15:04:05.000",
			Level:      slog.LevelDebug,
			NoColor:    runtime.GOOS == "windows" || !isatty.IsTerminal(os.Stderr.Fd()),
		}),
		DefaultLogHandler(),
	)
	slog.SetDefault(slog.New(handler))
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

type LogListModel = gioutil.ListModel[slog.Record]

var LogListModelType = gioutil.NewListModelType[slog.Record]()

// LogHandler is a slog.Handler that stores logs in a list model.
// To obtain the list model, use the [ListModel] method.
type LogHandler struct {
	level *atomic.Pointer[slog.Leveler]
	list  *LogListModel

	attrs  []slog.Attr
	groups string
	max    atomic.Int32
}

var _ slog.Handler = (*LogHandler)(nil)

// NewLogHandler creates a new LogHandler with the given options.
// If maxEntries is 0, then the list model will have no limit.
func NewLogHandler(maxEntries int, opts *slog.HandlerOptions) *LogHandler {
	h := &LogHandler{
		level: new(atomic.Pointer[slog.Leveler]),
		list:  LogListModelType.New(),
	}
	h.max.Store(int32(maxEntries))
	h.level.Store(&opts.Level)
	return h
}

var (
	defaultOnce    sync.Once
	defaultHandler *LogHandler
)

// DefaultLogHandler returns the default log handler.
// Debug logs are enabled by default.
func DefaultLogHandler() *LogHandler {
	defaultOnce.Do(func() {
		defaultHandler = NewLogHandler(250, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	})
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

// MaxEntries returns the maximum number of log entries.
func (h *LogHandler) MaxEntries() int {
	return int(h.max.Load())
}

// SetMaxEntries sets the maximum number of log entries.
func (h *LogHandler) SetMaxEntries(n int) {
	h.max.Store(int32(n))
}

func (h *LogHandler) clone() *LogHandler {
	h2 := &LogHandler{
		level:  h.level,
		list:   h.list,
		attrs:  append([]slog.Attr{}, h.attrs...),
		groups: h.groups,
	}
	h2.max.Store(h.max.Load())
	return h2
}

func (h *LogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.Level().Level()
}

func (h *LogHandler) Handle(_ context.Context, record slog.Record) error {
	record = record.Clone()
	record.AddAttrs(h.attrs...)

	coreglib.IdleAdd(func() {
		h.list.Append(record)

		max := int(h.max.Load())
		if max > 0 {
			n := h.list.Len()
			if n > max {
				h.list.Splice(0, n-max)
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
