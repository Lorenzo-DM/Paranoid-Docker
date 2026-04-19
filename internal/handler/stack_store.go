package handler

import (
	"backend/internal/model"
	"sync"
)

type StackJobStore struct {
	mu   sync.RWMutex
	jobs map[string]chan model.StackEvent
}

func NewStackJobStore() *StackJobStore {
	return &StackJobStore{jobs: make(map[string]chan model.StackEvent)}
}

func (s *StackJobStore) Set(name string, ch chan model.StackEvent) {
	s.mu.Lock()
	s.jobs[name] = ch
	s.mu.Unlock()
}

func (s *StackJobStore) Get(name string) (chan model.StackEvent, bool) {
	s.mu.RLock()
	ch, ok := s.jobs[name]
	s.mu.RUnlock()
	return ch, ok
}

func (s *StackJobStore) Delete(name string) {
	s.mu.Lock()
	delete(s.jobs, name)
	s.mu.Unlock()
}
