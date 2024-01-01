package app

import "context"

// This file contains what would be v2 of the state API.
// All future state APIs should be based on this.

// StateKey defines a constant key for a state. It exposes a type-safe API to
// acquire, get and restore state.
type StateKey[StateT any] struct {
	tails []string
}

// NewStateKey creates a new StateKey with the given state type and the config
// path tails.
func NewStateKey[StateT any](tails ...string) StateKey[StateT] {
	return StateKey[StateT]{tails: tails}
}

func (s StateKey[StateT]) Acquire(ctx context.Context) *TypedState[StateT] {
	state := AcquireState(ctx, s.tails...)
	return (*TypedState[StateT])(state)
}

// TypedState is a type-safe wrapper around State.
type TypedState[StateT any] State

// Each loops over each key in the map. It automatically unmarshals the value
// before calling f. To avoid this, use EachKey.
func (s *TypedState[StateT]) Each(f func(key string, value StateT) (done bool)) {
	state := (*State)(s)
	state.Each(func(key string, unmarshal func(interface{}) bool) bool {
		var value StateT
		if !unmarshal(&value) {
			return false
		}
		return f(key, value)
	})
}

// EachKey loops over each key in the map.
func (s *TypedState[StateT]) EachKey(f func(key string) (done bool)) {
	state := (*State)(s)
	state.Each(func(key string, _ func(interface{}) bool) bool {
		return f(key)
	})
}

// Get gets the value of the key.
func (s *TypedState[StateT]) Get(key string, f func(StateT, bool)) {
	var value StateT
	state := (*State)(s)
	state.GetAsync(key, &value, func(ok bool) { f(value, ok) })
}

// Exists returns true if key exists.
func (s *TypedState[StateT]) Exists(key string) bool {
	state := (*State)(s)
	return state.Exists(key)
}

// Set sets the value of the key.
func (s *TypedState[StateT]) Set(key string, value StateT) {
	state := (*State)(s)
	state.Set(key, value)
}

// Delete deletes the key.
func (s *TypedState[StateT]) Delete(key string) {
	state := (*State)(s)
	state.Delete(key)
}
