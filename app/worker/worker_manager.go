package worker

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mailvault/mailvault/domain/validation"

	"log/slog"

	"github.com/gofrs/uuid/v5"
)

// SimpleValidationUseCase defines the minimal interface used by WorkerManager
type SimpleValidationUseCase interface {
	ValidateDomain(ctx context.Context, domainID uuid.UUID) (*validation.FullValidationResult, error)
	GetPendingValidations(ctx context.Context, limit int) ([]*validation.DomainValidationInfo, error)
}

// ManagerConfig is a simplified configuration for the WorkerManager
type ManagerConfig struct {
	WorkerCount       int
	QueueSize         int
	PollInterval      time.Duration
	ProcessingTimeout time.Duration
	MaxRetries        int
	BaseRetryDelay    time.Duration
}

// WorkerManagerStats contains statistics for the WorkerManager
type WorkerManagerStats struct {
	WorkerCount    int
	QueueSize      int
	TotalProcessed int64
	TotalSuccess   int64
	TotalFailed    int64
}

// WorkerManager is a simplified worker manager for domain validation
type WorkerManager struct {
	useCase        SimpleValidationUseCase
	config         ManagerConfig
	logger         *slog.Logger
	queue          chan *validation.ValidationJob
	running        bool
	mutex          sync.RWMutex
	stopCh         chan struct{}
	wg             sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
	totalProcessed atomic.Int64
	totalSuccess   atomic.Int64
	totalFailed    atomic.Int64
}

// NewWorkerManager creates a new simplified WorkerManager
func NewWorkerManager(useCase SimpleValidationUseCase, config ManagerConfig, logger *slog.Logger) *WorkerManager {
	return &WorkerManager{
		useCase: useCase,
		config:  config,
		logger:  logger,
		queue:   make(chan *validation.ValidationJob, config.QueueSize),
		stopCh:  make(chan struct{}),
	}
}

// Start starts the worker manager
func (wm *WorkerManager) Start() error {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	if wm.running {
		return fmt.Errorf("worker manager is already running")
	}

	wm.ctx, wm.cancel = context.WithCancel(context.Background())
	wm.stopCh = make(chan struct{})
	wm.running = true

	// Start worker goroutines
	for i := 0; i < wm.config.WorkerCount; i++ {
		wm.wg.Add(1)
		go wm.worker(i)
	}

	// Start discovery goroutine
	wm.wg.Add(1)
	go wm.discoveryLoop()

	return nil
}

// Stop stops the worker manager
func (wm *WorkerManager) Stop() {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	if !wm.running {
		return
	}

	wm.cancel()
	close(wm.stopCh)
	wm.wg.Wait()
	wm.running = false
}

// IsRunning returns whether the manager is running
func (wm *WorkerManager) IsRunning() bool {
	wm.mutex.RLock()
	defer wm.mutex.RUnlock()
	return wm.running
}

// QueueJob adds a validation job to the queue
func (wm *WorkerManager) QueueJob(job *validation.ValidationJob) error {
	select {
	case wm.queue <- job:
		return nil
	default:
		return fmt.Errorf("queue is full")
	}
}

// GetStats returns current statistics
func (wm *WorkerManager) GetStats() WorkerManagerStats {
	return WorkerManagerStats{
		WorkerCount:    wm.config.WorkerCount,
		QueueSize:      len(wm.queue),
		TotalProcessed: wm.totalProcessed.Load(),
		TotalSuccess:   wm.totalSuccess.Load(),
		TotalFailed:    wm.totalFailed.Load(),
	}
}

// worker processes jobs from the queue
func (wm *WorkerManager) worker(id int) {
	defer wm.wg.Done()

	for {
		select {
		case <-wm.stopCh:
			return
		case job, ok := <-wm.queue:
			if !ok {
				return
			}
			wm.processJob(job, 0)
		}
	}
}

// processJob processes a single validation job with retry support
func (wm *WorkerManager) processJob(job *validation.ValidationJob, attempt int) {
	ctx, cancel := context.WithTimeout(wm.ctx, wm.config.ProcessingTimeout)
	defer cancel()

	_, err := wm.useCase.ValidateDomain(ctx, job.DomainID)
	wm.totalProcessed.Add(1)

	if err != nil {
		wm.totalFailed.Add(1)
		if attempt < wm.config.MaxRetries {
			delay := wm.config.BaseRetryDelay * time.Duration(1<<uint(attempt))
			select {
			case <-wm.stopCh:
				return
			case <-time.After(delay):
				wm.processJob(job, attempt+1)
			}
		}
		return
	}

	wm.totalSuccess.Add(1)
}

// discoveryLoop periodically discovers pending validations
func (wm *WorkerManager) discoveryLoop() {
	defer wm.wg.Done()

	ticker := time.NewTicker(wm.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-wm.stopCh:
			return
		case <-ticker.C:
			wm.discoverPendingValidations()
		}
	}
}

// discoverPendingValidations queries for pending validations and queues them
func (wm *WorkerManager) discoverPendingValidations() {
	ctx, cancel := context.WithTimeout(wm.ctx, 10*time.Second)
	defer cancel()

	pending, err := wm.useCase.GetPendingValidations(ctx, 100)
	if err != nil {
		wm.logger.Error("Failed to get pending validations", "error", err)
		return
	}

	for _, info := range pending {
		job := &validation.ValidationJob{
			ID:         uuid.Must(uuid.NewV4()),
			DomainID:   info.ID,
			DomainName: info.Domain,
			Type:       validation.ValidationTypeFullValidation,
			Priority:   50,
			CreatedAt:  time.Now(),
		}

		if err := wm.QueueJob(job); err != nil {
			wm.logger.Warn("Failed to queue pending validation job", "domain_id", info.ID, "error", err)
		}
	}
}
