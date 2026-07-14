package handler

import (
	"sync"
	"time"
)

const (
	// maxBufferedEvents bounds each job's replay buffer.
	maxBufferedEvents = 5000
	// jobTTL keeps finished jobs available for reconnect replay.
	jobTTL = 10 * time.Minute
)

// bufferedJob accumulates every event of one job so late or reconnecting
// subscribers can replay from any index and then follow live.
type bufferedJob[T any] struct {
	mu      sync.Mutex
	events  []T
	done    bool
	changed chan struct{}
}

func (j *bufferedJob[T]) append(evt T) {
	j.mu.Lock()
	if len(j.events) < maxBufferedEvents {
		j.events = append(j.events, evt)
	}
	close(j.changed)
	j.changed = make(chan struct{})
	j.mu.Unlock()
}

func (j *bufferedJob[T]) markDone() {
	j.mu.Lock()
	j.done = true
	close(j.changed)
	j.changed = make(chan struct{})
	j.mu.Unlock()
}

// snapshotAfter returns the events past index from, the done flag, and a
// channel closed on the next change.
func (j *bufferedJob[T]) snapshotAfter(from int) ([]T, bool, <-chan struct{}) {
	j.mu.Lock()
	defer j.mu.Unlock()
	var evts []T
	if from < len(j.events) {
		evts = append(evts, j.events[from:]...)
	}
	return evts, j.done, j.changed
}

// JobBuffer tracks jobs by id, replacing the consume-once job stores so
// SSE clients can reconnect and replay.
type JobBuffer[T any] struct {
	mu   sync.Mutex
	jobs map[string]*bufferedJob[T]
	ttl  time.Duration
}

func NewJobBuffer[T any]() *JobBuffer[T] {
	return &JobBuffer[T]{jobs: map[string]*bufferedJob[T]{}, ttl: jobTTL}
}

// Start registers a job and drains its event channel in the background.
// When the channel closes the job is marked done and evicted after the
// TTL.
func (b *JobBuffer[T]) Start(id string, ch <-chan T) {
	job := &bufferedJob[T]{changed: make(chan struct{})}
	b.mu.Lock()
	b.jobs[id] = job
	b.mu.Unlock()

	go func() {
		for evt := range ch {
			job.append(evt)
		}
		job.markDone()
		time.AfterFunc(b.ttl, func() {
			b.mu.Lock()
			if b.jobs[id] == job {
				delete(b.jobs, id)
			}
			b.mu.Unlock()
		})
	}()
}

func (b *JobBuffer[T]) Get(id string) (*bufferedJob[T], bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	job, ok := b.jobs[id]
	return job, ok
}
