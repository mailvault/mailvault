package worker

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"time"

	"mailvault/domain/validation"

	"github.com/gofrs/uuid/v5"
)

// Queue defines the interface for job queues
type Queue interface {
	Push(job *validation.ValidationJob) error
	Pop(ctx context.Context) (*validation.ValidationJob, error)
	Size() int
	Clear()
	Close() error
}

// PriorityQueue implements a thread-safe priority queue for validation jobs
type PriorityQueue struct {
	mutex   sync.RWMutex
	jobs    jobQueue
	cond    *sync.Cond
	closed  bool
	maxSize int
}

// jobQueue implements heap.Interface for priority queue
type jobQueue []*validation.ValidationJob

func (jq jobQueue) Len() int { return len(jq) }

func (jq jobQueue) Less(i, j int) bool {
	// Higher priority first (higher number = higher priority)
	if jq[i].Priority != jq[j].Priority {
		return jq[i].Priority > jq[j].Priority
	}
	// If same priority, older jobs first (FIFO)
	return jq[i].CreatedAt.Before(jq[j].CreatedAt)
}

func (jq jobQueue) Swap(i, j int) {
	jq[i], jq[j] = jq[j], jq[i]
}

func (jq *jobQueue) Push(x interface{}) {
	*jq = append(*jq, x.(*validation.ValidationJob))
}

func (jq *jobQueue) Pop() interface{} {
	old := *jq
	n := len(old)
	job := old[n-1]
	*jq = old[0 : n-1]
	return job
}

// NewPriorityQueue creates a new priority queue
func NewPriorityQueue(maxSize int) Queue {
	pq := &PriorityQueue{
		jobs:    make(jobQueue, 0),
		maxSize: maxSize,
	}
	pq.cond = sync.NewCond(&pq.mutex)
	heap.Init(&pq.jobs)
	return pq
}

// Push adds a job to the queue
func (pq *PriorityQueue) Push(job *validation.ValidationJob) error {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()

	if pq.closed {
		return ErrQueueClosed
	}

	if pq.maxSize > 0 && len(pq.jobs) >= pq.maxSize {
		return ErrQueueFull
	}

	heap.Push(&pq.jobs, job)
	pq.cond.Signal()
	return nil
}

// Pop removes and returns the highest priority job from the queue
func (pq *PriorityQueue) Pop(ctx context.Context) (*validation.ValidationJob, error) {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()

	for len(pq.jobs) == 0 && !pq.closed {
		// Check if context is done while waiting
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		pq.cond.Wait()
	}

	if pq.closed && len(pq.jobs) == 0 {
		return nil, ErrQueueClosed
	}

	job := heap.Pop(&pq.jobs).(*validation.ValidationJob)
	return job, nil
}

// Size returns the current size of the queue
func (pq *PriorityQueue) Size() int {
	pq.mutex.RLock()
	defer pq.mutex.RUnlock()
	return len(pq.jobs)
}

// Clear removes all jobs from the queue
func (pq *PriorityQueue) Clear() {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()
	pq.jobs = pq.jobs[:0]
}

// Close closes the queue and signals waiting goroutines
func (pq *PriorityQueue) Close() error {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()

	if pq.closed {
		return nil
	}

	pq.closed = true
	pq.cond.Broadcast()
	return nil
}

// InMemoryQueue implements a simple in-memory queue for validation jobs
type InMemoryQueue struct {
	mutex   sync.RWMutex
	jobs    []*validation.ValidationJob
	cond    *sync.Cond
	closed  bool
	maxSize int
}

// NewInMemoryQueue creates a new in-memory queue
func NewInMemoryQueue(maxSize int) Queue {
	q := &InMemoryQueue{
		jobs:    make([]*validation.ValidationJob, 0),
		maxSize: maxSize,
	}
	q.cond = sync.NewCond(&q.mutex)
	return q
}

// Push adds a job to the queue (FIFO)
func (q *InMemoryQueue) Push(job *validation.ValidationJob) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.closed {
		return ErrQueueClosed
	}

	if q.maxSize > 0 && len(q.jobs) >= q.maxSize {
		return ErrQueueFull
	}

	q.jobs = append(q.jobs, job)
	q.cond.Signal()
	return nil
}

// Pop removes and returns the first job from the queue (FIFO)
func (q *InMemoryQueue) Pop(ctx context.Context) (*validation.ValidationJob, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	for len(q.jobs) == 0 && !q.closed {
		// Check if context is done while waiting
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		q.cond.Wait()
	}

	if q.closed && len(q.jobs) == 0 {
		return nil, ErrQueueClosed
	}

	job := q.jobs[0]
	q.jobs = q.jobs[1:]
	return job, nil
}

// Size returns the current size of the queue
func (q *InMemoryQueue) Size() int {
	q.mutex.RLock()
	defer q.mutex.RUnlock()
	return len(q.jobs)
}

// Clear removes all jobs from the queue
func (q *InMemoryQueue) Clear() {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.jobs = q.jobs[:0]
}

// Close closes the queue and signals waiting goroutines
func (q *InMemoryQueue) Close() error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.closed {
		return nil
	}

	q.closed = true
	q.cond.Broadcast()
	return nil
}

// JobScheduler manages scheduled jobs and retries
type JobScheduler struct {
	mutex     sync.RWMutex
	scheduled map[uuid.UUID]*ScheduledJob
	ticker    *time.Ticker
	queue     Queue
	stopCh    chan struct{}
	stopped   bool
}

// ScheduledJob represents a job scheduled for future execution
type ScheduledJob struct {
	Job       *validation.ValidationJob
	ScheduledAt time.Time
	RetryCount  int
	MaxRetries  int
}

// NewJobScheduler creates a new job scheduler
func NewJobScheduler(queue Queue, checkInterval time.Duration) *JobScheduler {
	return &JobScheduler{
		scheduled: make(map[uuid.UUID]*ScheduledJob),
		ticker:    time.NewTicker(checkInterval),
		queue:     queue,
		stopCh:    make(chan struct{}),
	}
}

// ScheduleJob schedules a job for future execution
func (js *JobScheduler) ScheduleJob(job *validation.ValidationJob, scheduleTime time.Time, maxRetries int) {
	js.mutex.Lock()
	defer js.mutex.Unlock()

	js.scheduled[job.ID] = &ScheduledJob{
		Job:         job,
		ScheduledAt: scheduleTime,
		RetryCount:  job.Attempts,
		MaxRetries:  maxRetries,
	}
}

// Start starts the scheduler
func (js *JobScheduler) Start(ctx context.Context) {
	go js.run(ctx)
}

// Stop stops the scheduler
func (js *JobScheduler) Stop() {
	js.mutex.Lock()
	defer js.mutex.Unlock()

	if js.stopped {
		return
	}

	js.stopped = true
	js.ticker.Stop()
	close(js.stopCh)
}

// run is the main scheduler loop
func (js *JobScheduler) run(ctx context.Context) {
	for {
		select {
		case <-js.ticker.C:
			js.checkScheduledJobs()
		case <-js.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// checkScheduledJobs checks for jobs that are ready to be executed
func (js *JobScheduler) checkScheduledJobs() {
	js.mutex.Lock()
	defer js.mutex.Unlock()

	now := time.Now()
	var toRemove []uuid.UUID

	for id, scheduled := range js.scheduled {
		if now.After(scheduled.ScheduledAt) || now.Equal(scheduled.ScheduledAt) {
			// Job is ready to be executed
			err := js.queue.Push(scheduled.Job)
			if err == nil {
				toRemove = append(toRemove, id)
			}
			// If push failed, keep the job scheduled and retry later
		}
	}

	// Remove successfully queued jobs
	for _, id := range toRemove {
		delete(js.scheduled, id)
	}
}

// GetScheduledJobsCount returns the number of scheduled jobs
func (js *JobScheduler) GetScheduledJobsCount() int {
	js.mutex.RLock()
	defer js.mutex.RUnlock()
	return len(js.scheduled)
}

// Common errors
var (
	ErrQueueClosed = fmt.Errorf("queue is closed")
	ErrQueueFull   = fmt.Errorf("queue is full")
)

// JobPriority constants
const (
	PriorityHigh   = 100
	PriorityNormal = 50
	PriorityLow    = 10
)

// CreateValidationJob creates a new validation job
func CreateValidationJob(domainID uuid.UUID, domainName string, validationType validation.ValidationType, priority int) *validation.ValidationJob {
	return &validation.ValidationJob{
		ID:         uuid.Must(uuid.NewV4()),
		DomainID:   domainID,
		DomainName: domainName,
		Type:       validationType,
		Priority:   priority,
		CreatedAt:  time.Now(),
		Attempts:   0,
	}
}

// CalculateRetryDelay calculates the delay for the next retry using exponential backoff
func CalculateRetryDelay(attempt int, baseDelay time.Duration) time.Duration {
	if attempt <= 0 {
		return baseDelay
	}

	// Exponential backoff: baseDelay * 2^(attempt-1)
	// Capped at 24 hours
	multiplier := 1 << (attempt - 1)
	delay := time.Duration(int64(baseDelay) * int64(multiplier))
	maxDelay := 24 * time.Hour
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}