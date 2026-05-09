package worker

import (
	"testing"
	"time"

	"mailvault/domain/validation"

	"github.com/gofrs/uuid/v5"
)

func TestPriorityQueue_EnqueueDequeue(t *testing.T) {
	queue := NewPriorityQueue()

	// Create test jobs with different priorities
	job1 := &validation.ValidationJob{
		ID:       uuid.Must(uuid.NewV4()),
		Priority: 10,
		DomainName: "low-priority.com",
	}
	job2 := &validation.ValidationJob{
		ID:       uuid.Must(uuid.NewV4()),
		Priority: 100,
		DomainName: "high-priority.com",
	}
	job3 := &validation.ValidationJob{
		ID:       uuid.Must(uuid.NewV4()),
		Priority: 50,
		DomainName: "medium-priority.com",
	}

	// Enqueue jobs in random order
	queue.Enqueue(job1)
	queue.Enqueue(job2)
	queue.Enqueue(job3)

	// Should dequeue in priority order (highest first)
	dequeued1 := queue.Dequeue()
	if dequeued1 == nil || dequeued1.Priority != 100 {
		t.Errorf("Expected highest priority job (100), got %v", dequeued1)
	}

	dequeued2 := queue.Dequeue()
	if dequeued2 == nil || dequeued2.Priority != 50 {
		t.Errorf("Expected medium priority job (50), got %v", dequeued2)
	}

	dequeued3 := queue.Dequeue()
	if dequeued3 == nil || dequeued3.Priority != 10 {
		t.Errorf("Expected low priority job (10), got %v", dequeued3)
	}

	// Queue should be empty now
	if !queue.IsEmpty() {
		t.Error("Queue should be empty after dequeuing all jobs")
	}

	// Dequeuing from empty queue should return nil
	emptyDequeue := queue.Dequeue()
	if emptyDequeue != nil {
		t.Error("Dequeuing from empty queue should return nil")
	}
}

func TestPriorityQueue_Size(t *testing.T) {
	queue := NewPriorityQueue()

	if queue.Size() != 0 {
		t.Errorf("Expected empty queue size 0, got %d", queue.Size())
	}

	// Add jobs
	for i := 0; i < 5; i++ {
		job := &validation.ValidationJob{
			ID:       uuid.Must(uuid.NewV4()),
			Priority: i,
			DomainName: "test.com",
		}
		queue.Enqueue(job)

		expectedSize := i + 1
		if queue.Size() != expectedSize {
			t.Errorf("Expected queue size %d, got %d", expectedSize, queue.Size())
		}
	}

	// Remove jobs
	for i := 4; i >= 0; i-- {
		queue.Dequeue()

		if queue.Size() != i {
			t.Errorf("Expected queue size %d, got %d", i, queue.Size())
		}
	}
}

func TestPriorityQueue_IsEmpty(t *testing.T) {
	queue := NewPriorityQueue()

	if !queue.IsEmpty() {
		t.Error("New queue should be empty")
	}

	job := &validation.ValidationJob{
		ID:       uuid.Must(uuid.NewV4()),
		Priority: 10,
		DomainName: "test.com",
	}
	queue.Enqueue(job)

	if queue.IsEmpty() {
		t.Error("Queue with job should not be empty")
	}

	queue.Dequeue()

	if !queue.IsEmpty() {
		t.Error("Queue should be empty after removing all jobs")
	}
}

func TestPriorityQueue_ConcurrentAccess(t *testing.T) {
	queue := NewPriorityQueue()
	done := make(chan bool)

	// Producer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			job := &validation.ValidationJob{
				ID:       uuid.Must(uuid.NewV4()),
				Priority: i % 10,
				DomainName: "test.com",
			}
			queue.Enqueue(job)
		}
		done <- true
	}()

	// Consumer goroutine
	go func() {
		consumed := 0
		for consumed < 100 {
			if !queue.IsEmpty() {
				job := queue.Dequeue()
				if job != nil {
					consumed++
				}
			}
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Queue should be empty
	if !queue.IsEmpty() {
		t.Errorf("Queue should be empty, but has %d items", queue.Size())
	}
}

func TestJobScheduler_Schedule(t *testing.T) {
	queue := NewPriorityQueue()
	scheduler := NewJobScheduler(queue)

	job := &validation.ValidationJob{
		ID:       uuid.Must(uuid.NewV4()),
		Priority: 10,
		DomainName: "test.com",
		Attempts: 1,
	}

	// Schedule for immediate execution. The scheduler's ticker (default 10ms)
	// promotes due jobs into the queue, so wait briefly for it to run.
	scheduler.Schedule(job, time.Now())

	deadline := time.Now().Add(500 * time.Millisecond)
	for queue.Size() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	// Job should be in queue
	if queue.Size() != 1 {
		t.Errorf("Expected 1 job in queue, got %d", queue.Size())
	}

	dequeuedJob := queue.Dequeue()
	if dequeuedJob == nil || dequeuedJob.ID != job.ID {
		t.Error("Scheduled job should be available in queue")
	}
}

func TestJobScheduler_ScheduleDelayed(t *testing.T) {
	queue := NewPriorityQueue()
	scheduler := NewJobScheduler(queue)

	job := &validation.ValidationJob{
		ID:       uuid.Must(uuid.NewV4()),
		Priority: 10,
		DomainName: "test.com",
		Attempts: 1,
	}

	// Schedule for future execution
	futureTime := time.Now().Add(100 * time.Millisecond)
	scheduler.Schedule(job, futureTime)

	// Job should not be in queue immediately
	if queue.Size() != 0 {
		t.Errorf("Expected 0 jobs in queue immediately, got %d", queue.Size())
	}

	// Wait for job to be scheduled
	time.Sleep(150 * time.Millisecond)

	// Job should now be in queue
	if queue.Size() != 1 {
		t.Errorf("Expected 1 job in queue after delay, got %d", queue.Size())
	}

	dequeuedJob := queue.Dequeue()
	if dequeuedJob == nil || dequeuedJob.ID != job.ID {
		t.Error("Delayed job should be available in queue after delay")
	}
}

func TestJobScheduler_Stop(t *testing.T) {
	queue := NewPriorityQueue()
	scheduler := NewJobScheduler(queue)

	// Schedule a job for the future
	job := &validation.ValidationJob{
		ID:       uuid.Must(uuid.NewV4()),
		Priority: 10,
		DomainName: "test.com",
		Attempts: 1,
	}
	futureTime := time.Now().Add(200 * time.Millisecond)
	scheduler.Schedule(job, futureTime)

	// Stop scheduler immediately
	scheduler.Stop()

	// Wait longer than the scheduled time
	time.Sleep(300 * time.Millisecond)

	// Job should not have been scheduled since scheduler was stopped
	if queue.Size() != 0 {
		t.Errorf("Expected 0 jobs in queue after stopping scheduler, got %d", queue.Size())
	}
}

func TestJobScheduler_MultipleJobs(t *testing.T) {
	queue := NewPriorityQueue()
	scheduler := NewJobScheduler(queue)
	defer scheduler.Stop()

	now := time.Now()
	jobs := []*validation.ValidationJob{
		{
			ID:       uuid.Must(uuid.NewV4()),
			Priority: 30,
			DomainName: "third.com",
		},
		{
			ID:       uuid.Must(uuid.NewV4()),
			Priority: 10,
			DomainName: "first.com",
		},
		{
			ID:       uuid.Must(uuid.NewV4()),
			Priority: 20,
			DomainName: "second.com",
		},
	}

	// Schedule jobs with different delays
	scheduler.Schedule(jobs[0], now.Add(50*time.Millisecond))
	scheduler.Schedule(jobs[1], now.Add(10*time.Millisecond))
	scheduler.Schedule(jobs[2], now.Add(30*time.Millisecond))

	// Wait for all jobs to be scheduled
	time.Sleep(100 * time.Millisecond)

	// All jobs should be in queue
	if queue.Size() != 3 {
		t.Errorf("Expected 3 jobs in queue, got %d", queue.Size())
	}

	// Jobs should be dequeued in priority order, not schedule order
	firstJob := queue.Dequeue()
	if firstJob.Priority != 30 {
		t.Errorf("Expected first job priority 30, got %d", firstJob.Priority)
	}

	secondJob := queue.Dequeue()
	if secondJob.Priority != 20 {
		t.Errorf("Expected second job priority 20, got %d", secondJob.Priority)
	}

	thirdJob := queue.Dequeue()
	if thirdJob.Priority != 10 {
		t.Errorf("Expected third job priority 10, got %d", thirdJob.Priority)
	}
}

func BenchmarkPriorityQueue_Enqueue(b *testing.B) {
	queue := NewPriorityQueue()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		job := &validation.ValidationJob{
			ID:       uuid.Must(uuid.NewV4()),
			Priority: i % 100,
			DomainName: "benchmark.com",
		}
		queue.Enqueue(job)
	}
}

func BenchmarkPriorityQueue_Dequeue(b *testing.B) {
	queue := NewPriorityQueue()

	// Pre-populate queue
	for i := 0; i < b.N; i++ {
		job := &validation.ValidationJob{
			ID:       uuid.Must(uuid.NewV4()),
			Priority: i % 100,
			DomainName: "benchmark.com",
		}
		queue.Enqueue(job)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		queue.Dequeue()
	}
}