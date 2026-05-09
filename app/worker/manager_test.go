package worker

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"mailvault/domain/validation"

	"github.com/gofrs/uuid/v5"
	"github.com/guilhermebr/gox/logger"
)

// mockValidateDomainFunc is a function field type for overriding in tests
type mockValidateDomainFunc func(ctx context.Context, domainID uuid.UUID) (*validation.FullValidationResult, error)

// Mock validation use case for testing
type mockValidationUseCase struct {
	validateResult            *validation.FullValidationResult
	validateError             error
	pendingValidations        []*validation.DomainValidationInfo
	getPendingValidationsCall int
	validateDomainCalls       []uuid.UUID
	mutex                     sync.Mutex
	// overrideFunc allows tests to override ValidateDomain behavior
	overrideFunc              mockValidateDomainFunc
}

func (m *mockValidationUseCase) ValidateDomain(ctx context.Context, domainID uuid.UUID) (*validation.FullValidationResult, error) {
	if m.overrideFunc != nil {
		return m.overrideFunc(ctx, domainID)
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.validateDomainCalls = append(m.validateDomainCalls, domainID)

	if m.validateError != nil {
		return nil, m.validateError
	}
	if m.validateResult != nil {
		return m.validateResult, nil
	}
	return &validation.FullValidationResult{
		Domain:       "test.com",
		OverallValid: true,
		TotalTime:    100 * time.Millisecond,
	}, nil
}

func (m *mockValidationUseCase) GetPendingValidations(ctx context.Context, limit int) ([]*validation.DomainValidationInfo, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.getPendingValidationsCall++
	return m.pendingValidations, nil
}

func (m *mockValidationUseCase) getValidateDomainCalls() []uuid.UUID {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	calls := make([]uuid.UUID, len(m.validateDomainCalls))
	copy(calls, m.validateDomainCalls)
	return calls
}

func TestWorkerManager_StartStop(t *testing.T) {
	logger, _ := logger.NewLogger("")
	useCase := &mockValidationUseCase{}

	config := ManagerConfig{
		WorkerCount:        2,
		QueueSize:         10,
		PollInterval:      100 * time.Millisecond,
		ProcessingTimeout: 5 * time.Second,
		MaxRetries:        3,
		BaseRetryDelay:    1 * time.Second,
	}

	manager := NewWorkerManager(useCase, config, logger)

	// Start manager
	err := manager.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Verify it's running
	if !manager.IsRunning() {
		t.Error("Manager should be running after Start()")
	}

	// Stop manager
	manager.Stop()

	// Give it time to stop
	time.Sleep(50 * time.Millisecond)

	// Verify it's stopped
	if manager.IsRunning() {
		t.Error("Manager should not be running after Stop()")
	}
}

func TestWorkerManager_ProcessJob(t *testing.T) {
	logger, _ := logger.NewLogger("")
	useCase := &mockValidationUseCase{}

	config := ManagerConfig{
		WorkerCount:        1,
		QueueSize:         10,
		PollInterval:      100 * time.Millisecond,
		ProcessingTimeout: 5 * time.Second,
		MaxRetries:        3,
		BaseRetryDelay:    1 * time.Second,
	}

	manager := NewWorkerManager(useCase, config, logger)

	err := manager.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer manager.Stop()

	// Create and queue a job
	job := &validation.ValidationJob{
		ID:         uuid.Must(uuid.NewV4()),
		DomainID:   uuid.Must(uuid.NewV4()),
		DomainName: "test.com",
		Type:       validation.ValidationTypeFullValidation,
		Priority:   100,
		Attempts:   0,
		CreatedAt:  time.Now(),
	}

	err = manager.QueueJob(job)
	if err != nil {
		t.Fatalf("QueueJob() error = %v", err)
	}

	// Wait for job to be processed
	time.Sleep(200 * time.Millisecond)

	// Check that validate domain was called
	calls := useCase.getValidateDomainCalls()
	if len(calls) != 1 {
		t.Errorf("Expected 1 ValidateDomain call, got %d", len(calls))
	} else if calls[0] != job.DomainID {
		t.Errorf("Expected ValidateDomain call with domain ID %v, got %v", job.DomainID, calls[0])
	}
}

func TestWorkerManager_ProcessJobWithRetry(t *testing.T) {
	logger, _ := logger.NewLogger("")
	useCase := &mockValidationUseCase{
		validateError: fmt.Errorf("temporary DNS error"),
	}

	config := ManagerConfig{
		WorkerCount:        1,
		QueueSize:         10,
		PollInterval:      10 * time.Millisecond,
		ProcessingTimeout: 100 * time.Millisecond,
		MaxRetries:        2,
		BaseRetryDelay:    50 * time.Millisecond,
	}

	manager := NewWorkerManager(useCase, config, logger)

	err := manager.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer manager.Stop()

	// Create and queue a job
	job := &validation.ValidationJob{
		ID:         uuid.Must(uuid.NewV4()),
		DomainID:   uuid.Must(uuid.NewV4()),
		DomainName: "test.com",
		Type:       validation.ValidationTypeFullValidation,
		Priority:   100,
		Attempts:   0,
		CreatedAt:  time.Now(),
	}

	err = manager.QueueJob(job)
	if err != nil {
		t.Fatalf("QueueJob() error = %v", err)
	}

	// Wait for job to be processed and retried
	time.Sleep(500 * time.Millisecond)

	// Check that validate domain was called multiple times (original + retries)
	calls := useCase.getValidateDomainCalls()
	if len(calls) < 2 {
		t.Errorf("Expected at least 2 ValidateDomain calls (with retries), got %d", len(calls))
	}
}

func TestWorkerManager_QueueMultipleJobs(t *testing.T) {
	logger, _ := logger.NewLogger("")
	useCase := &mockValidationUseCase{}

	config := ManagerConfig{
		WorkerCount:        2,
		QueueSize:         10,
		PollInterval:      50 * time.Millisecond,
		ProcessingTimeout: 5 * time.Second,
		MaxRetries:        3,
		BaseRetryDelay:    1 * time.Second,
	}

	manager := NewWorkerManager(useCase, config, logger)

	err := manager.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer manager.Stop()

	// Create and queue multiple jobs
	numJobs := 5
	jobIDs := make([]uuid.UUID, numJobs)

	for i := 0; i < numJobs; i++ {
		job := &validation.ValidationJob{
			ID:         uuid.Must(uuid.NewV4()),
			DomainID:   uuid.Must(uuid.NewV4()),
			DomainName: fmt.Sprintf("test%d.com", i),
			Type:       validation.ValidationTypeFullValidation,
			Priority:   100 - i, // Different priorities
			Attempts:   0,
			CreatedAt:  time.Now(),
		}
		jobIDs[i] = job.DomainID

		err = manager.QueueJob(job)
		if err != nil {
			t.Fatalf("QueueJob() error = %v", err)
		}
	}

	// Wait for all jobs to be processed
	time.Sleep(500 * time.Millisecond)

	// Check that all jobs were processed
	calls := useCase.getValidateDomainCalls()
	if len(calls) != numJobs {
		t.Errorf("Expected %d ValidateDomain calls, got %d", numJobs, len(calls))
	}

	// Verify all job IDs were called
	callMap := make(map[uuid.UUID]bool)
	for _, call := range calls {
		callMap[call] = true
	}

	for _, jobID := range jobIDs {
		if !callMap[jobID] {
			t.Errorf("Job ID %v was not processed", jobID)
		}
	}
}

func TestWorkerManager_GetStats(t *testing.T) {
	logger, _ := logger.NewLogger("")
	useCase := &mockValidationUseCase{}

	config := ManagerConfig{
		WorkerCount:        2,
		QueueSize:         10,
		PollInterval:      50 * time.Millisecond,
		ProcessingTimeout: 5 * time.Second,
		MaxRetries:        3,
		BaseRetryDelay:    1 * time.Second,
	}

	manager := NewWorkerManager(useCase, config, logger)

	err := manager.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer manager.Stop()

	// Get initial stats
	stats := manager.GetStats()

	if stats.WorkerCount != 2 {
		t.Errorf("Expected worker count 2, got %d", stats.WorkerCount)
	}

	if stats.QueueSize != 0 {
		t.Errorf("Expected empty queue, got size %d", stats.QueueSize)
	}

	if stats.TotalProcessed != 0 {
		t.Errorf("Expected 0 processed jobs initially, got %d", stats.TotalProcessed)
	}

	// Add a job and check stats update
	job := &validation.ValidationJob{
		ID:         uuid.Must(uuid.NewV4()),
		DomainID:   uuid.Must(uuid.NewV4()),
		DomainName: "test.com",
		Type:       validation.ValidationTypeFullValidation,
		Priority:   100,
		Attempts:   0,
		CreatedAt:  time.Now(),
	}

	err = manager.QueueJob(job)
	if err != nil {
		t.Fatalf("QueueJob() error = %v", err)
	}

	// Wait for job to be processed
	time.Sleep(200 * time.Millisecond)

	// Check updated stats
	updatedStats := manager.GetStats()

	if updatedStats.TotalProcessed == 0 {
		t.Error("Expected processed job count to increase")
	}
}

func TestWorkerManager_DiscoverPendingValidations(t *testing.T) {
	logger, _ := logger.NewLogger("")

	// Mock use case with pending validations
	pendingValidations := []*validation.DomainValidationInfo{
		{
			ID:     uuid.Must(uuid.NewV4()),
			Domain: "pending1.com",
		},
		{
			ID:     uuid.Must(uuid.NewV4()),
			Domain: "pending2.com",
		},
	}

	useCase := &mockValidationUseCase{
		pendingValidations: pendingValidations,
	}

	config := ManagerConfig{
		WorkerCount:        1,
		QueueSize:         10,
		PollInterval:      50 * time.Millisecond,
		ProcessingTimeout: 5 * time.Second,
		MaxRetries:        3,
		BaseRetryDelay:    1 * time.Second,
	}

	manager := NewWorkerManager(useCase, config, logger)

	err := manager.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer manager.Stop()

	// Wait for discovery to run
	time.Sleep(200 * time.Millisecond)

	// Check that pending validations were discovered and processed
	calls := useCase.getValidateDomainCalls()
	if len(calls) < 2 {
		t.Errorf("Expected at least 2 ValidateDomain calls for pending validations, got %d", len(calls))
	}

	// Check that GetPendingValidations was called
	if useCase.getPendingValidationsCall == 0 {
		t.Error("Expected GetPendingValidations to be called during discovery")
	}
}

func TestWorkerManager_QueueFullError(t *testing.T) {
	logger, _ := logger.NewLogger("")
	useCase := &mockValidationUseCase{}

	config := ManagerConfig{
		WorkerCount:        1,
		QueueSize:         1, // Very small queue
		PollInterval:      100 * time.Millisecond,
		ProcessingTimeout: 5 * time.Second,
		MaxRetries:        3,
		BaseRetryDelay:    1 * time.Second,
	}

	manager := NewWorkerManager(useCase, config, logger)

	err := manager.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer manager.Stop()

	// Fill the queue
	job1 := &validation.ValidationJob{
		ID:         uuid.Must(uuid.NewV4()),
		DomainID:   uuid.Must(uuid.NewV4()),
		DomainName: "test1.com",
		Type:       validation.ValidationTypeFullValidation,
		Priority:   100,
		Attempts:   0,
		CreatedAt:  time.Now(),
	}

	err = manager.QueueJob(job1)
	if err != nil {
		t.Fatalf("First QueueJob() should succeed, error = %v", err)
	}

	// Try to add another job to full queue
	job2 := &validation.ValidationJob{
		ID:         uuid.Must(uuid.NewV4()),
		DomainID:   uuid.Must(uuid.NewV4()),
		DomainName: "test2.com",
		Type:       validation.ValidationTypeFullValidation,
		Priority:   100,
		Attempts:   0,
		CreatedAt:  time.Now(),
	}

	err = manager.QueueJob(job2)
	if err == nil {
		t.Error("QueueJob() should return error when queue is full")
	}
}

func TestWorkerManager_StopWhileProcessing(t *testing.T) {
	logger, _ := logger.NewLogger("")

	// Mock use case that takes some time to process
	useCase := &mockValidationUseCase{
		validateResult: &validation.FullValidationResult{
			Domain:       "slow.com",
			OverallValid: true,
			TotalTime:    200 * time.Millisecond,
		},
	}

	// Add artificial delay to simulate slow processing
	useCase.overrideFunc = func(ctx context.Context, domainID uuid.UUID) (*validation.FullValidationResult, error) {
		time.Sleep(100 * time.Millisecond)
		return useCase.validateResult, nil
	}

	config := ManagerConfig{
		WorkerCount:        1,
		QueueSize:         10,
		PollInterval:      10 * time.Millisecond,
		ProcessingTimeout: 5 * time.Second,
		MaxRetries:        3,
		BaseRetryDelay:    1 * time.Second,
	}

	manager := NewWorkerManager(useCase, config, logger)

	err := manager.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Queue a job
	job := &validation.ValidationJob{
		ID:         uuid.Must(uuid.NewV4()),
		DomainID:   uuid.Must(uuid.NewV4()),
		DomainName: "slow.com",
		Type:       validation.ValidationTypeFullValidation,
		Priority:   100,
		Attempts:   0,
		CreatedAt:  time.Now(),
	}

	err = manager.QueueJob(job)
	if err != nil {
		t.Fatalf("QueueJob() error = %v", err)
	}

	// Wait a bit for job to start processing, then stop
	time.Sleep(50 * time.Millisecond)
	manager.Stop()

	// Should stop gracefully
	if manager.IsRunning() {
		t.Error("Manager should stop even while processing jobs")
	}
}

func BenchmarkWorkerManager_QueueJob(b *testing.B) {
	logger, _ := logger.NewLogger("")
	useCase := &mockValidationUseCase{}

	config := ManagerConfig{
		WorkerCount:        4,
		QueueSize:         1000,
		PollInterval:      100 * time.Millisecond,
		ProcessingTimeout: 5 * time.Second,
		MaxRetries:        3,
		BaseRetryDelay:    1 * time.Second,
	}

	manager := NewWorkerManager(useCase, config, logger)

	err := manager.Start()
	if err != nil {
		b.Fatalf("Start() error = %v", err)
	}
	defer manager.Stop()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		job := &validation.ValidationJob{
			ID:         uuid.Must(uuid.NewV4()),
			DomainID:   uuid.Must(uuid.NewV4()),
			DomainName: fmt.Sprintf("benchmark%d.com", i),
			Type:       validation.ValidationTypeFullValidation,
			Priority:   i % 100,
			Attempts:   0,
			CreatedAt:  time.Now(),
		}

		manager.QueueJob(job)
	}
}