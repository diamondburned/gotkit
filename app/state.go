package app

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"

	"github.com/diamondburned/gotkit/utils/config"
)

// State implements an easy API for storing persistent state that components can
// use.
type State struct {
	path  string
	store config.ConfigStore

	mut    sync.Mutex
	state  map[string]json.RawMessage
	loaded bool
}

// AcquireState creates a new Config instance.
func AcquireState(ctx context.Context, tails ...string) *State {
	app := FromContext(ctx)
	return acquireState(app.ConfigPath(tails...))
}

var registry = struct {
	sync.RWMutex
	cfgs map[string]*State
}{
	cfgs: map[string]*State{},
}

// acquireState creates a new state config.
func acquireState(path string) *State {
	registry.RLock()
	s, ok := registry.cfgs[path]
	registry.RUnlock()

	if ok {
		return s
	}

	registry.Lock()
	defer registry.Unlock()

	s, ok = registry.cfgs[path]
	if ok {
		return s
	}

	s = &State{path: path}
	s.store = config.NewConfigStore(s.snapshotFunc)

	registry.cfgs[path] = s
	return s
}

// Each loops over each key in the map.
func (s *State) Each(f func(key string, unmarshal func(interface{}) bool) (done bool)) {
	s.mut.Lock()
	s.load()
	for k, b := range s.state {
		unmarshal := func(dst interface{}) bool {
			err := json.Unmarshal(b, dst)
			return err == nil
		}
		if f(k, unmarshal) {
			return
		}
	}
	s.mut.Unlock()
}

// Get gets the value of the key.
func (s *State) Get(key string, dst interface{}) bool {
	s.mut.Lock()
	s.load()
	b, ok := s.state[key]
	s.mut.Unlock()

	if !ok {
		return false
	}

	if err := json.Unmarshal(b, dst); err != nil {
		log.Printf("cannot unmarshal %q into %T: %v", b, dst, err)
		return false
	}

	return true
}

// Exists returns true if key exists.
func (s *State) Exists(key string) bool {
	s.mut.Lock()
	s.load()
	_, ok := s.state[key]
	s.mut.Unlock()

	return ok
}

// Set sets the value of the key. If val = nil, then the key is deleted.
func (s *State) Set(key string, val interface{}) {
	var b []byte
	if val != nil {
		var err error

		b, err = json.Marshal(val)
		if err != nil {
			log.Panicf("cannot marshal %T: %v", val, err)
		}
	}

	s.mut.Lock()
	s.load()
	if val == nil {
		delete(s.state, key)
	} else {
		s.state[key] = b
	}
	s.mut.Unlock()

	s.store.Save()
}

// Delete calls Set(key, nil).
func (s *State) Delete(key string) {
	s.Set(key, nil)
}

func (s *State) load() {
	if s.loaded {
		return
	}
	s.loaded = true
	s.state = make(map[string]json.RawMessage)

	f, err := os.Open(s.path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Println("cannot open preference:", err)
		}
		return
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&s.state); err != nil {
		log.Printf("preference %q has invalid JSON: %v", s.path, err)
		return
	}
}

func (s *State) snapshotFunc() func() {
	s.mut.Lock()
	defer s.mut.Unlock()

	if !s.loaded {
		log.Panicf("cannot snapshot unloaded config %q", s.path)
	}

	b, err := json.MarshalIndent(s.state, "", "\t")
	if err != nil {
		log.Panicln("cannot marshal kvstate.State:", err)
	}

	return func() {
		if err := config.WriteFile(s.path, b); err != nil {
			log.Println("cannot save kvstate:", err)
		}
	}
}
