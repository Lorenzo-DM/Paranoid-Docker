package handler

import (
	"backend/internal/service"
	"sync"
)

type JobStore struct {
	mu   sync.RWMutex
	jobs map[string]chan service.PullEvent
}

func NewJobStore() *JobStore {
	return &JobStore{jobs: make(map[string]chan service.PullEvent)}
}

func (s *JobStore) Set(id string, ch chan service.PullEvent) {
	s.mu.Lock()
	s.jobs[id] = ch
	s.mu.Unlock()
}

func (s *JobStore) Get(id string) (chan service.PullEvent, bool) {
	s.mu.RLock()
	ch, ok := s.jobs[id]
	s.mu.RUnlock()
	return ch, ok
}

func (s *JobStore) Delete(id string) {
	s.mu.Lock()
	delete(s.jobs, id)
	s.mu.Unlock()
}

type SaveJobStore struct {
	mu   sync.RWMutex
	jobs map[string]chan service.SaveProgress
}

func NewSaveJobStore() *SaveJobStore {
	return &SaveJobStore{jobs: make(map[string]chan service.SaveProgress)}
}

func (s *SaveJobStore) Set(id string, ch chan service.SaveProgress) {
	s.mu.Lock()
	s.jobs[id] = ch
	s.mu.Unlock()
}

func (s *SaveJobStore) Get(id string) (chan service.SaveProgress, bool) {
	s.mu.RLock()
	ch, ok := s.jobs[id]
	s.mu.RUnlock()
	return ch, ok
}

func (s *SaveJobStore) Delete(id string) {
	s.mu.Lock()
	delete(s.jobs, id)
	s.mu.Unlock()
}
