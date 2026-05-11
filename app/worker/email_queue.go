package worker

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"time"
)

// Common errors
var (
	ErrEmailQueueClosed = fmt.Errorf("email queue is closed")
	ErrEmailQueueFull   = fmt.Errorf("email queue is full")
	ErrEmailQueueEmpty  = fmt.Errorf("email queue is empty")
)

// EmailQueue defines the interface for email job queues
type EmailQueue interface {
	Push(job *EmailSendingJob) error
	Pop(ctx context.Context) (*EmailSendingJob, error)
	PopWithTimeout(timeout time.Duration) (*EmailSendingJob, error)
	Size() int
	Clear()
	Close() error
}

// EmailPriorityQueue implements a thread-safe priority queue for email jobs
type EmailPriorityQueue struct {
	mutex   sync.RWMutex
	jobs    emailJobQueue
	cond    *sync.Cond
	closed  bool
	maxSize int
}

// emailJobQueue implements heap.Interface for priority queue
type emailJobQueue []*EmailSendingJob

func (eq emailJobQueue) Len() int { return len(eq) }

func (eq emailJobQueue) Less(i, j int) bool {
	// Higher priority first (higher number = higher priority)
	if eq[i].Priority != eq[j].Priority {
		return eq[i].Priority > eq[j].Priority
	}
	// If same priority, scheduled time first
	return eq[i].ScheduledAt.Before(eq[j].ScheduledAt)
}

func (eq emailJobQueue) Swap(i, j int) {
	eq[i], eq[j] = eq[j], eq[i]
}

func (eq *emailJobQueue) Push(x interface{}) {
	*eq = append(*eq, x.(*EmailSendingJob))
}

func (eq *emailJobQueue) Pop() interface{} {
	old := *eq
	n := len(old)
	job := old[n-1]
	*eq = old[0 : n-1]
	return job
}

// NewEmailPriorityQueue creates a new email priority queue
func NewEmailPriorityQueue(maxSize int) *EmailPriorityQueue {
	pq := &EmailPriorityQueue{
		jobs:    make(emailJobQueue, 0),
		maxSize: maxSize,
	}
	pq.cond = sync.NewCond(&pq.mutex)
	heap.Init(&pq.jobs)
	return pq
}

// Push adds an email job to the queue
func (eq *EmailPriorityQueue) Push(job *EmailSendingJob) error {
	eq.mutex.Lock()
	defer eq.mutex.Unlock()

	if eq.closed {
		return ErrEmailQueueClosed
	}

	if eq.maxSize > 0 && len(eq.jobs) >= eq.maxSize {
		return ErrEmailQueueFull
	}

	heap.Push(&eq.jobs, job)
	eq.cond.Signal()
	return nil
}

// Pop removes and returns the highest priority email job from the queue
func (eq *EmailPriorityQueue) Pop(ctx context.Context) (*EmailSendingJob, error) {
	eq.mutex.Lock()
	defer eq.mutex.Unlock()

	for len(eq.jobs) == 0 && !eq.closed {
		// Check if context is done while waiting
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Wait for a job to be available
		eq.cond.Wait()
	}

	if eq.closed {
		return nil, ErrEmailQueueClosed
	}

	if len(eq.jobs) == 0 {
		return nil, ErrEmailQueueEmpty
	}

	job := heap.Pop(&eq.jobs).(*EmailSendingJob)
	return job, nil
}

// PopWithTimeout removes and returns the highest priority email job with timeout
func (eq *EmailPriorityQueue) PopWithTimeout(timeout time.Duration) (*EmailSendingJob, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return eq.Pop(ctx)
}

// Size returns the current queue size
func (eq *EmailPriorityQueue) Size() int {
	eq.mutex.RLock()
	defer eq.mutex.RUnlock()
	return len(eq.jobs)
}

// Clear removes all jobs from the queue
func (eq *EmailPriorityQueue) Clear() {
	eq.mutex.Lock()
	defer eq.mutex.Unlock()
	eq.jobs = eq.jobs[:0]
	heap.Init(&eq.jobs)
}

// Close closes the queue and signals waiting goroutines
func (eq *EmailPriorityQueue) Close() error {
	eq.mutex.Lock()
	defer eq.mutex.Unlock()

	if eq.closed {
		return nil
	}

	eq.closed = true
	eq.cond.Broadcast()
	return nil
}
