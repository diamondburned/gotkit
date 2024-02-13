package logui

import (
	"context"
	"errors"
	"log/slog"
)

type ctxKey uint8

const (
	_ ctxKey = iota
	preventRecursionKey
)

// Code borrowed from searKing's multiHandler.
// https://github.com/searKing/golang/blob/go/v1.2.115/go/log/slog/multi.go
//
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file within that repository.

var _ slog.Handler = (*multiHandler)(nil)

type multiHandler []slog.Handler

func (t multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, w := range t {
		if w != nil && w.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (t multiHandler) Handle(ctx context.Context, record slog.Record) error {
	if _, ok := ctx.Value(preventRecursionKey).(struct{}); ok {
		// Avoid infinite recursion.
		// Thank slog.defaultHandler for being so awfully designed.
		return nil
	}

	ctx = context.WithValue(ctx, preventRecursionKey, struct{}{})

	var errs []error
	for _, w := range t {
		if w == nil || !w.Enabled(ctx, record.Level) {
			continue
		}
		if err := w.Handle(ctx, record); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (t multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, 0, len(t))
	for _, w := range t {
		if w != nil {
			handlers = append(handlers, w.WithAttrs(attrs))
		}
	}
	return MultiHandler(handlers...)
}

func (t multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, 0, len(t))
	for _, w := range t {
		if w != nil {
			handlers = append(handlers, w.WithGroup(name))
		}
	}
	return MultiHandler(handlers...)
}

// MultiHandler creates a slog.Handler that duplicates its writes to all the
// provided handlers, similar to the Unix tee(1) command.
//
// Each write is written to each listed writer, one at a time.
// If a listed writer returns an error, that overall write operation
// stops and returns the error; it does not continue down the list.
func MultiHandler(handlers ...slog.Handler) slog.Handler {
	allHandlers := make([]slog.Handler, 0, len(handlers))
	for _, w := range handlers {
		if w == nil {
			continue
		}
		if mw, ok := w.(multiHandler); ok {
			allHandlers = append(allHandlers, mw...)
		} else {
			allHandlers = append(allHandlers, w)
		}
	}
	return multiHandler(allHandlers)
}
