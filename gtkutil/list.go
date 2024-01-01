package gtkutil

import (
	"encoding/json"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// ListModel is a wrapper around gtk.StringList that allows any Go type to be
// used as a list model. Internally, the values are encoded as JSON strings
// before being stored in the list model.
type ListModel[T any] struct {
	*gio.ListModel
	list *gtk.StringList
}

// NewListModel creates a new list model.
func NewListModel[T any]() *ListModel[T] {
	list := gtk.NewStringList(nil)
	return &ListModel[T]{
		ListModel: &list.ListModel,
		list:      list,
	}
}

// Append appends a value to the list.
func (l *ListModel[T]) Append(v T) {
	l.list.Append(mustEncodeListItem(v))
}

// Get returns the value at the given index.
func (l *ListModel[T]) Get(index uint) T {
	return mustDecodeListItem[T](l.list.String(index))
}

// Remove removes the value at the given index.
func (l *ListModel[T]) Remove(index uint) {
	l.list.Remove(index)
}

// Splice removes the values in the given range and replaces them with the
// given values.
func (l *ListModel[T]) Splice(position, remove uint, values ...T) {
	items := make([]string, len(values))
	for i, v := range values {
		items[i] = mustEncodeListItem(v)
	}
	l.list.Splice(position, remove, items)
}

func mustEncodeListItem[T any](v T) string {
	var s strings.Builder
	if err := json.NewEncoder(&s).Encode(v); err != nil {
		panic(err)
	}
	return s.String()
}

func mustDecodeListItem[T any](s string) T {
	var v T
	if err := json.NewDecoder(strings.NewReader(s)).Decode(&v); err != nil {
		panic(err)
	}
	return v
}
