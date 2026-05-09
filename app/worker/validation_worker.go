package worker

import (
	"context"
	"fmt"
	"time"

	"mailvault/domain/validation"

	"github.com/gofrs/uuid/v5"
	"log/slog"
)

// ValidationWorker performs domain validation tasks
type ValidationWorker struct {
	id                string
	queue            Queue
	validationUseCase ValidationUseCase
	dnsService       validation.DNSService
	config           validation.ValidationConfig
	logger           *slog.Logger
	stopCh           chan struct{}
	doneCh           chan struct{}
	running          bool
}

// ValidationUseCase defines the interface for validation business logic
type ValidationUseCase interface {
	ValidateDomain(ctx context.Context, domainID uuid.UUID) (*validation.FullValidationResult, error)
	ValidateMXRecords(ctx context.Context, domainID uuid.UUID) error
	ValidateTXTRecord(ctx context.Context, domainID uuid.UUID) error
	UpdateValidationStatus(ctx context.Context, domainID uuid.UUID, status validation.VerificationStatus, errorMsg *string) error
	GetDomainValidationInfo(ctx context.Context, domainID uuid.UUID) (*validation.DomainValidationInfo, error)
	GetPendingValidations(ctx context.Context, limit int) ([]*validation.DomainValidationInfo, error)
}

// NewValidationWorker creates a new validation worker
func NewValidationWorker(
	id string,
	queue Queue,
	validationUseCase ValidationUseCase,
	dnsService validation.DNSService,
	config validation.ValidationConfig,
	logger *slog.Logger,
) *ValidationWorker {
	return &ValidationWorker{
		id:                id,
		queue:            queue,
		validationUseCase: validationUseCase,
		dnsService:       dnsService,
		config:           config,
		logger:           logger,
		stopCh:           make(chan struct{}),
		doneCh:           make(chan struct{}),
	}
}

// Start starts the worker
func (w *ValidationWorker) Start(ctx context.Context) {
	w.running = true
	w.logger.Info( "Starting validation worker", "worker_id", w.id)

	go w.run(ctx)
}

// Stop stops the worker
func (w *ValidationWorker) Stop() {
	if !w.running {
		return
	}

	w.logger.Info( "Stopping validation worker", "worker_id", w.id)
	close(w.stopCh)
	<-w.doneCh
	w.running = false
}

// IsRunning returns whether the worker is running
func (w *ValidationWorker) IsRunning() bool {
	return w.running
}

// run is the main worker loop
func (w *ValidationWorker) run(ctx context.Context) {
	defer close(w.doneCh)

	for {
		select {
		case <-w.stopCh:
			w.logger.Info( "Worker stopped", "worker_id", w.id)
			return
		case <-ctx.Done():
			w.logger.Info( "Worker context cancelled", "worker_id", w.id)
			return
		default:
			// Try to get a job from the queue
			job, err := w.queue.Pop(ctx)
			if err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded {
					w.logger.Info( "Worker context done", "worker_id", w.id)
					return
				}
				if err == ErrQueueClosed {
					w.logger.Info( "Queue closed, worker stopping", "worker_id", w.id)
					return
				}
				w.logger.Error( "Error getting job from queue",
					"worker_id", w.id,
					"error", err,
				)
				time.Sleep(1 * time.Second) // Brief pause before retrying
				continue
			}

			// Process the job
			w.processJob(ctx, job)
		}
	}
}

// processJob processes a single validation job
func (w *ValidationWorker) processJob(ctx context.Context, job *validation.ValidationJob) {
	startTime := time.Now()

	w.logger.Info( "Processing validation job",
		"worker_id", w.id,
		"job_id", job.ID,
		"domain_id", job.DomainID,
		"domain_name", job.DomainName,
		"validation_type", job.Type,
		"attempts", job.Attempts,
	)

	// Create a timeout context for the job
	jobCtx, cancel := context.WithTimeout(ctx, w.config.ValidationTimeout)
	defer cancel()

	var err error
	switch job.Type {
	case validation.ValidationTypeMXRecord:
		err = w.validationUseCase.ValidateMXRecords(jobCtx, job.DomainID)
	case validation.ValidationTypeTXTRecord:
		err = w.validationUseCase.ValidateTXTRecord(jobCtx, job.DomainID)
	case validation.ValidationTypeFullValidation:
		_, err = w.validationUseCase.ValidateDomain(jobCtx, job.DomainID)
	case validation.ValidationTypeOwnership:
		// For ownership validation, we typically validate TXT record
		err = w.validationUseCase.ValidateTXTRecord(jobCtx, job.DomainID)
	default:
		err = fmt.Errorf("unknown validation type: %s", job.Type)
	}

	duration := time.Since(startTime)

	if err != nil {
		w.logger.Error( "Validation job failed",
			"worker_id", w.id,
			"job_id", job.ID,
			"domain_id", job.DomainID,
			"domain_name", job.DomainName,
			"validation_type", job.Type,
			"attempts", job.Attempts,
			"duration", duration,
			"error", err,
		)

		// Update job with error
		job.LastError = err.Error()
		job.Attempts++

		// Handle retry logic
		w.handleJobRetry(ctx, job)
	} else {
		w.logger.Info( "Validation job completed successfully",
			"worker_id", w.id,
			"job_id", job.ID,
			"domain_id", job.DomainID,
			"domain_name", job.DomainName,
			"validation_type", job.Type,
			"attempts", job.Attempts,
			"duration", duration,
		)
	}
}

// handleJobRetry handles retry logic for failed jobs
func (w *ValidationWorker) handleJobRetry(ctx context.Context, job *validation.ValidationJob) {
	if job.Attempts >= w.config.MaxRetries {
		w.logger.Warn( "Max retries reached for validation job",
			"worker_id", w.id,
			"job_id", job.ID,
			"domain_id", job.DomainID,
			"domain_name", job.DomainName,
			"validation_type", job.Type,
			"attempts", job.Attempts,
			"max_retries", w.config.MaxRetries,
		)

		// Update domain status to failed
		errorMsg := fmt.Sprintf("Max retries reached (%d): %s", job.Attempts, job.LastError)
		err := w.validationUseCase.UpdateValidationStatus(ctx, job.DomainID, validation.VerificationStatusFailed, &errorMsg)
		if err != nil {
			w.logger.Error( "Failed to update domain validation status to failed",
				"worker_id", w.id,
				"domain_id", job.DomainID,
				"error", err,
			)
		}
		return
	}

	// Calculate retry delay
	retryDelay := CalculateRetryDelay(job.Attempts, w.config.RetryDelay)
	retryTime := time.Now().Add(retryDelay)

	w.logger.Info( "Scheduling validation job for retry",
		"worker_id", w.id,
		"job_id", job.ID,
		"domain_id", job.DomainID,
		"domain_name", job.DomainName,
		"validation_type", job.Type,
		"attempts", job.Attempts,
		"retry_delay", retryDelay,
		"retry_time", retryTime,
	)

	// Note: In a real implementation, you would schedule this job for retry
	// This could be done by adding it to a scheduler or putting it back in the queue with a delay
	// For now, we'll just log the retry information
}

// WorkerPool manages a pool of validation workers
type WorkerPool struct {
	workers           []*ValidationWorker
	queue            Queue
	validationUseCase ValidationUseCase
	dnsService       validation.DNSService
	config           validation.ValidationConfig
	logger           *slog.Logger
	size             int
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(
	size int,
	queue Queue,
	validationUseCase ValidationUseCase,
	dnsService validation.DNSService,
	config validation.ValidationConfig,
	logger *slog.Logger,
) *WorkerPool {
	workers := make([]*ValidationWorker, size)
	for i := 0; i < size; i++ {
		workerID := fmt.Sprintf("worker-%d", i+1)
		workers[i] = NewValidationWorker(
			workerID,
			queue,
			validationUseCase,
			dnsService,
			config,
			logger,
		)
	}

	return &WorkerPool{
		workers:           workers,
		queue:            queue,
		validationUseCase: validationUseCase,
		dnsService:       dnsService,
		config:           config,
		logger:           logger,
		size:             size,
	}
}

// Start starts all workers in the pool
func (wp *WorkerPool) Start(ctx context.Context) {
	wp.logger.Info( "Starting worker pool", "pool_size", wp.size)

	for _, worker := range wp.workers {
		worker.Start(ctx)
	}
}

// Stop stops all workers in the pool
func (wp *WorkerPool) Stop() {
	wp.logger.Info( "Stopping worker pool", "pool_size", wp.size)

	for _, worker := range wp.workers {
		worker.Stop()
	}
}

// GetRunningWorkers returns the number of running workers
func (wp *WorkerPool) GetRunningWorkers() int {
	count := 0
	for _, worker := range wp.workers {
		if worker.IsRunning() {
			count++
		}
	}
	return count
}

// GetPoolStats returns statistics about the worker pool
func (wp *WorkerPool) GetPoolStats() WorkerPoolStats {
	runningWorkers := wp.GetRunningWorkers()
	queueSize := wp.queue.Size()

	return WorkerPoolStats{
		TotalWorkers:   wp.size,
		RunningWorkers: runningWorkers,
		QueueSize:      queueSize,
	}
}

// WorkerPoolStats contains statistics about the worker pool
type WorkerPoolStats struct {
	TotalWorkers   int `json:"total_workers"`
	RunningWorkers int `json:"running_workers"`
	QueueSize      int `json:"queue_size"`
}