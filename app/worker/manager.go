package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"mailvault/domain/validation"

	"log/slog"

	"github.com/gofrs/uuid/v5"
)

// Manager coordinates validation workers, scheduler, and job distribution
type Manager struct {
	workerPool *WorkerPool
	queue      Queue
	scheduler  *JobScheduler
	useCase    ValidationUseCase
	dnsService validation.DNSService
	config     WorkerConfig
	logger     *slog.Logger
	running    bool
	mutex      sync.RWMutex
	stopCh     chan struct{}
	doneCh     chan struct{}
	stats      ManagerStats
}

// WorkerConfig contains configuration for the worker manager
type WorkerConfig struct {
	// Worker pool settings
	WorkerCount int    `json:"worker_count"`
	QueueSize   int    `json:"queue_size"`
	QueueType   string `json:"queue_type"` // "priority" or "fifo"

	// Scheduling settings
	CheckInterval time.Duration `json:"check_interval"`
	MaxRetries    int           `json:"max_retries"`

	// Validation settings
	ValidationConfig validation.ValidationConfig `json:"validation_config"`

	// Monitoring settings
	StatsInterval time.Duration `json:"stats_interval"`
	Enabled       bool          `json:"enabled"`
}

// DefaultWorkerConfig returns a default worker configuration
func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		WorkerCount:      3,
		QueueSize:        1000,
		QueueType:        "priority",
		CheckInterval:    30 * time.Second,
		MaxRetries:       3,
		ValidationConfig: validation.DefaultValidationConfig(),
		StatsInterval:    1 * time.Minute,
		Enabled:          true,
	}
}

// ManagerStats contains statistics about the worker manager
type ManagerStats struct {
	StartTime          time.Time     `json:"start_time"`
	JobsProcessed      int64         `json:"jobs_processed"`
	JobsSuccessful     int64         `json:"jobs_successful"`
	JobsFailed         int64         `json:"jobs_failed"`
	JobsRetried        int64         `json:"jobs_retried"`
	DomainsValidated   int64         `json:"domains_validated"`
	AverageProcessTime time.Duration `json:"average_process_time"`
	LastStatsUpdate    time.Time     `json:"last_stats_update"`
}

// NewManager creates a new worker manager
func NewManager(
	useCase ValidationUseCase,
	dnsService validation.DNSService,
	config WorkerConfig,
	logger *slog.Logger,
) *Manager {
	// Create queue based on configuration
	var queue Queue
	switch config.QueueType {
	case "priority":
		queue = NewPriorityQueue(config.QueueSize)
	default:
		queue = NewInMemoryQueue(config.QueueSize)
	}

	// Create worker pool
	workerPool := NewWorkerPool(
		config.WorkerCount,
		queue,
		useCase,
		dnsService,
		config.ValidationConfig,
		logger,
	)

	// Create scheduler
	scheduler := NewJobScheduler(queue, config.CheckInterval)

	return &Manager{
		workerPool: workerPool,
		queue:      queue,
		scheduler:  scheduler,
		useCase:    useCase,
		dnsService: dnsService,
		config:     config,
		logger:     logger,
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
		stats: ManagerStats{
			LastStatsUpdate: time.Now(),
		},
	}
}

// Start starts the worker manager
func (m *Manager) Start(ctx context.Context) error {
	if !m.config.Enabled {
		m.logger.Info("Worker manager is disabled")
		return nil
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.running {
		return fmt.Errorf("worker manager is already running")
	}

	m.logger.Info("Starting worker manager",
		"worker_count", m.config.WorkerCount,
		"queue_size", m.config.QueueSize,
		"queue_type", m.config.QueueType,
	)

	m.running = true
	m.stats.StartTime = time.Now()

	// Start components
	m.workerPool.Start(ctx)
	m.scheduler.Start(ctx)

	// Start management goroutines
	go m.run(ctx)
	go m.statsCollector(ctx)

	return nil
}

// Stop stops the worker manager
func (m *Manager) Stop() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.running {
		return nil
	}

	m.logger.Info("Stopping worker manager")

	// Signal stop
	close(m.stopCh)

	// Stop components
	m.scheduler.Stop()
	m.workerPool.Stop()
	m.queue.Close()

	// Wait for management goroutines to finish
	<-m.doneCh

	m.running = false
	m.logger.Info("Worker manager stopped")

	return nil
}

// IsRunning returns whether the manager is running
func (m *Manager) IsRunning() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.running
}

// QueueValidationJob adds a validation job to the queue
func (m *Manager) QueueValidationJob(job *validation.ValidationJob) error {
	if !m.running {
		return fmt.Errorf("worker manager is not running")
	}

	m.logger.Info("Queueing validation job",
		"job_id", job.ID,
		"domain_id", job.DomainID,
		"domain_name", job.DomainName,
		"validation_type", job.Type,
		"priority", job.Priority,
	)

	return m.queue.Push(job)
}

// ScheduleValidationJob schedules a validation job for future execution
func (m *Manager) ScheduleValidationJob(job *validation.ValidationJob, scheduleTime time.Time) error {
	if !m.running {
		return fmt.Errorf("worker manager is not running")
	}

	m.logger.Info("Scheduling validation job",
		"job_id", job.ID,
		"domain_id", job.DomainID,
		"domain_name", job.DomainName,
		"validation_type", job.Type,
		"schedule_time", scheduleTime,
	)

	m.scheduler.ScheduleJob(job, scheduleTime, m.config.MaxRetries)
	return nil
}

// QueueDomainValidation queues a full domain validation
func (m *Manager) QueueDomainValidation(domainID uuid.UUID, domainName string, priority int) error {
	job := CreateValidationJob(domainID, domainName, validation.ValidationTypeFullValidation, priority)
	return m.QueueValidationJob(job)
}

// QueueMXValidation queues an MX record validation
func (m *Manager) QueueMXValidation(domainID uuid.UUID, domainName string, priority int) error {
	job := CreateValidationJob(domainID, domainName, validation.ValidationTypeMXRecord, priority)
	return m.QueueValidationJob(job)
}

// QueueTXTValidation queues a TXT record validation
func (m *Manager) QueueTXTValidation(domainID uuid.UUID, domainName string, priority int) error {
	job := CreateValidationJob(domainID, domainName, validation.ValidationTypeTXTRecord, priority)
	return m.QueueValidationJob(job)
}

// GetStats returns current manager statistics
func (m *Manager) GetStats() ManagerStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.stats
}

// GetDetailedStats returns detailed statistics including worker pool stats
func (m *Manager) GetDetailedStats() DetailedStats {
	m.mutex.RLock()
	managerStats := m.stats
	m.mutex.RUnlock()

	workerStats := m.workerPool.GetPoolStats()
	scheduledJobs := m.scheduler.GetScheduledJobsCount()

	return DetailedStats{
		Manager:       managerStats,
		WorkerPool:    workerStats,
		ScheduledJobs: scheduledJobs,
		QueueSize:     m.queue.Size(),
	}
}

// DetailedStats contains comprehensive statistics
type DetailedStats struct {
	Manager       ManagerStats    `json:"manager"`
	WorkerPool    WorkerPoolStats `json:"worker_pool"`
	ScheduledJobs int             `json:"scheduled_jobs"`
	QueueSize     int             `json:"queue_size"`
}

// run is the main management loop
func (m *Manager) run(ctx context.Context) {
	defer close(m.doneCh)

	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.performMaintenanceTasks(ctx)
		}
	}
}

// performMaintenanceTasks performs periodic maintenance
func (m *Manager) performMaintenanceTasks(ctx context.Context) {
	// Check for domains that need validation
	m.checkForPendingValidations(ctx)

	// Update statistics
	m.updateStats()

	// Log current status
	stats := m.GetDetailedStats()
	m.logger.Info("Worker manager status",
		"running_workers", stats.WorkerPool.RunningWorkers,
		"total_workers", stats.WorkerPool.TotalWorkers,
		"queue_size", stats.QueueSize,
		"scheduled_jobs", stats.ScheduledJobs,
		"jobs_processed", stats.Manager.JobsProcessed,
		"jobs_successful", stats.Manager.JobsSuccessful,
		"jobs_failed", stats.Manager.JobsFailed,
	)
}

// checkForPendingValidations checks for domains that need validation
func (m *Manager) checkForPendingValidations(ctx context.Context) {
	// This would query the database for domains that need validation
	// For now, we'll just log that we're checking
	m.logger.Debug("Checking for pending domain validations")

	// In a real implementation, you would:
	// 1. Query the validation repository for domains needing validation
	// 2. Create validation jobs for those domains
	// 3. Queue or schedule the jobs appropriately
}

// updateStats updates internal statistics
func (m *Manager) updateStats() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.stats.LastStatsUpdate = time.Now()
	// Other statistics would be updated here based on completed jobs
}

// statsCollector collects and logs statistics periodically
func (m *Manager) statsCollector(ctx context.Context) {
	ticker := time.NewTicker(m.config.StatsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			stats := m.GetDetailedStats()
			m.logger.Info("Worker manager statistics",
				"uptime", time.Since(stats.Manager.StartTime),
				"jobs_processed", stats.Manager.JobsProcessed,
				"jobs_successful", stats.Manager.JobsSuccessful,
				"jobs_failed", stats.Manager.JobsFailed,
				"success_rate", m.calculateSuccessRate(),
				"running_workers", stats.WorkerPool.RunningWorkers,
				"queue_size", stats.QueueSize,
				"scheduled_jobs", stats.ScheduledJobs,
			)
		}
	}
}

// calculateSuccessRate calculates the current success rate
func (m *Manager) calculateSuccessRate() float64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.stats.JobsProcessed == 0 {
		return 0.0
	}

	return float64(m.stats.JobsSuccessful) / float64(m.stats.JobsProcessed) * 100.0
}
