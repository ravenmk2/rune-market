package server

import (
	"net/http"
	"sync"
)

// Switcher is the root HTTP handler that lets the setup wizard hot-swap the
// setup engine for the normal engine without restarting the process (§5).
type Switcher struct {
	mu      sync.RWMutex
	handler http.Handler
}

func NewSwitcher(h http.Handler) *Switcher {
	return &Switcher{handler: h}
}

func (s *Switcher) Swap(h http.Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler = h
}

func (s *Switcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	h := s.handler
	s.mu.RUnlock()
	h.ServeHTTP(w, r)
}
