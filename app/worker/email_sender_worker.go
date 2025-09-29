package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"mailvault/domain/email_sending"
	"mailvault/internal/providers"

	"github.com/gofrs/uuid/v5"
	"log/slog"
)

// EmailSenderWorker handles email sending jobs
type EmailSenderWorker struct {
	id              int
	queue           EmailQueue
	emailUseCase    *email_sending.UseCase
	emailSender     providers.EmailSender
	logger          *slog.Logger
	running         bool
	mutex           sync.RWMutex
	stopCh          chan struct{}
	doneCh          chan struct{}
	stats           EmailWorkerStats
	config          EmailWorkerConfig
}

// EmailWorkerConfig contains configuration for email workers
type EmailWorkerConfig struct {
	BatchSize     int           `json:"batch_size"`     // Number of emails to process in one batch
	RetryInterval time.Duration `json:"retry_interval"` // Interval between retry attempts
	MaxRetries    int           `json:"max_retries"`    // Max retries per email
	Timeout       time.Duration `json:"timeout"`        // Timeout for sending individual emails
}

// DefaultEmailWorkerConfig returns default email worker configuration
func DefaultEmailWorkerConfig() EmailWorkerConfig {
	return EmailWorkerConfig{
		BatchSize:     10,
		RetryInterval: 5 * time.Minute,
		MaxRetries:    3,
		Timeout:       30 * time.Second,
	}
}

// EmailWorkerStats contains statistics for email workers
type EmailWorkerStats struct {
	WorkerID         int           `json:"worker_id"`
	StartTime        time.Time     `json:"start_time"`
	EmailsProcessed  int64         `json:"emails_processed"`
	EmailsSuccessful int64         `json:"emails_successful"`
	EmailsFailed     int64         `json:"emails_failed"`
	EmailsRetried    int64         `json:"emails_retried"`
	AverageTime      time.Duration `json:"average_time"`
	LastJobTime      time.Time     `json:"last_job_time"`
	IsActive         bool          `json:"is_active"`
}

// EmailSendingJob represents an email sending job
type EmailSendingJob struct {
	ID              uuid.UUID                      `json:"id"`
	Type            EmailJobType                   `json:"type"`
	SentEmailID     uuid.UUID                      `json:"sent_email_id,omitempty"`
	EmailRequest    *email_sending.SendEmailRequest `json:"email_request,omitempty"`
	Priority        int                            `json:"priority"`
	ScheduledAt     time.Time                      `json:"scheduled_at"`
	Attempts        int                            `json:"attempts"`
	MaxAttempts     int                            `json:"max_attempts"`
	LastAttemptAt   time.Time                      `json:"last_attempt_at"`
	LastError       string                         `json:"last_error,omitempty"`
	CreatedAt       time.Time                      `json:"created_at"`
}

// EmailJobType represents the type of email job
type EmailJobType string

const (
	EmailJobTypeSend  EmailJobType = "send"  // Send a new email
	EmailJobTypeRetry EmailJobType = "retry" // Retry a failed email
)

// GetPriority returns the job priority for queue sorting
func (j *EmailSendingJob) GetPriority() int {
	return j.Priority
}

// GetID returns the job ID
func (j *EmailSendingJob) GetID() string {
	return j.ID.String()
}

// NewEmailSenderWorker creates a new email sender worker
func NewEmailSenderWorker(
	id int,
	queue EmailQueue,
	emailUseCase *email_sending.UseCase,
	emailSender providers.EmailSender,
	config EmailWorkerConfig,
	logger *slog.Logger,
) *EmailSenderWorker {
	return &EmailSenderWorker{
		id:           id,
		queue:        queue,
		emailUseCase: emailUseCase,
		emailSender:  emailSender,
		config:       config,
		logger:       logger,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
		stats: EmailWorkerStats{
			WorkerID:  id,
			IsActive:  false,
		},
	}
}

// Start starts the email worker
func (w *EmailSenderWorker) Start(ctx context.Context) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.running {
		return
	}

	w.logger.Info("Starting email sender worker",
		"worker_id", w.id)

	w.running = true
	w.stats.StartTime = time.Now()
	w.stats.IsActive = true

	go w.run(ctx)
}

// Stop stops the email worker
func (w *EmailSenderWorker) Stop() {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if !w.running {
		return
	}

	w.logger.Info("Stopping email sender worker",
		"worker_id", w.id)

	close(w.stopCh)
	<-w.doneCh

	w.running = false
	w.stats.IsActive = false
}

// GetStats returns worker statistics
func (w *EmailSenderWorker) GetStats() EmailWorkerStats {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	return w.stats
}

// IsRunning returns whether the worker is running
func (w *EmailSenderWorker) IsRunning() bool {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	return w.running
}

// run is the main worker loop
func (w *EmailSenderWorker) run(ctx context.Context) {
	defer close(w.doneCh)

	w.logger.Info("Email sender worker started",
		"worker_id", w.id)

	for {
		select {
		case <-w.stopCh:
			w.logger.Info("Email sender worker stopped",
				"worker_id", w.id)
			return
		case <-ctx.Done():
			w.logger.Info("Email sender worker context cancelled",
				"worker_id", w.id)
			return
		default:
			w.processJobs(ctx)
		}
	}
}

// processJobs processes email jobs from the queue
func (w *EmailSenderWorker) processJobs(ctx context.Context) {
	// Try to get a job from the queue
	emailJob, err := w.queue.PopWithTimeout(1 * time.Second) // Wait up to 1 second for a job
	if err != nil {
		// No job available or queue error
		return
	}

	w.processEmailJob(ctx, emailJob)
}

// processEmailJob processes a single email job
func (w *EmailSenderWorker) processEmailJob(ctx context.Context, job *EmailSendingJob) {
	startTime := time.Now()

	w.logger.Info("Processing email job",
		"worker_id", w.id,
		"job_id", job.ID,
		"job_type", job.Type,
		"attempt", job.Attempts+1,
		"max_attempts", job.MaxAttempts)

	// Update stats
	w.mutex.Lock()
	w.stats.EmailsProcessed++
	w.stats.LastJobTime = startTime
	w.mutex.Unlock()

	// Create context with timeout
	jobCtx, cancel := context.WithTimeout(ctx, w.config.Timeout)
	defer cancel()

	// Process the job based on type
	var err error
	switch job.Type {
	case EmailJobTypeSend:
		err = w.processSendJob(jobCtx, job)
	case EmailJobTypeRetry:
		err = w.processRetryJob(jobCtx, job)
	default:
		err = fmt.Errorf("unknown job type: %s", job.Type)
	}

	duration := time.Since(startTime)

	// Update job attempts
	job.Attempts++
	job.LastAttemptAt = startTime

	if err != nil {
		w.handleJobFailure(job, err, duration)
	} else {
		w.handleJobSuccess(job, duration)
	}
}

// processSendJob processes a send email job
func (w *EmailSenderWorker) processSendJob(ctx context.Context, job *EmailSendingJob) error {
	if job.EmailRequest == nil {
		return fmt.Errorf("email request is nil")
	}

	w.logger.Debug("Sending email",
		"worker_id", w.id,
		"job_id", job.ID,
		"from", job.EmailRequest.From,
		"to_count", len(job.EmailRequest.ToAddresses),
		"subject", job.EmailRequest.Subject)

	// Send the email using the email use case
	_, err := w.emailUseCase.SendEmail(ctx, *job.EmailRequest)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	w.logger.Info("Email sent successfully",
		"worker_id", w.id,
		"job_id", job.ID,
		"message_id", job.EmailRequest.MessageID)

	return nil
}

// processRetryJob processes a retry email job
func (w *EmailSenderWorker) processRetryJob(ctx context.Context, job *EmailSendingJob) error {
	if job.SentEmailID == uuid.Nil {
		return fmt.Errorf("sent email ID is nil")
	}

	w.logger.Debug("Retrying failed email",
		"worker_id", w.id,
		"job_id", job.ID,
		"sent_email_id", job.SentEmailID)

	// Resend the email using the email use case
	_, err := w.emailUseCase.ResendEmail(ctx, job.SentEmailID)
	if err != nil {
		return fmt.Errorf("failed to resend email: %w", err)
	}

	w.logger.Info("Email retry successful",
		"worker_id", w.id,
		"job_id", job.ID,
		"sent_email_id", job.SentEmailID)

	return nil
}

// handleJobSuccess handles successful job completion
func (w *EmailSenderWorker) handleJobSuccess(job *EmailSendingJob, duration time.Duration) {
	w.mutex.Lock()
	w.stats.EmailsSuccessful++
	w.updateAverageTime(duration)
	w.mutex.Unlock()

	w.logger.Info("Email job completed successfully",
		"worker_id", w.id,
		"job_id", job.ID,
		"job_type", job.Type,
		"duration", duration,
		"attempts", job.Attempts)
}

// handleJobFailure handles failed job completion
func (w *EmailSenderWorker) handleJobFailure(job *EmailSendingJob, err error, duration time.Duration) {
	job.LastError = err.Error()

	w.logger.Error("Email job failed",
		"worker_id", w.id,
		"job_id", job.ID,
		"job_type", job.Type,
		"attempt", job.Attempts,
		"max_attempts", job.MaxAttempts,
		"duration", duration,
		"error", err)

	// Check if we should retry
	if job.Attempts < job.MaxAttempts {
		// Schedule retry
		w.scheduleRetry(job)
		w.mutex.Lock()
		w.stats.EmailsRetried++
		w.mutex.Unlock()
	} else {
		// Max attempts reached, mark as failed
		w.mutex.Lock()
		w.stats.EmailsFailed++
		w.mutex.Unlock()

		w.logger.Error("Email job failed permanently",
			"worker_id", w.id,
			"job_id", job.ID,
			"job_type", job.Type,
			"total_attempts", job.Attempts,
			"last_error", err)
	}

	w.mutex.Lock()
	w.updateAverageTime(duration)
	w.mutex.Unlock()
}

// scheduleRetry schedules a job for retry
func (w *EmailSenderWorker) scheduleRetry(job *EmailSendingJob) {
	// Calculate exponential backoff delay
	backoffDelay := w.config.RetryInterval * time.Duration(1<<uint(job.Attempts-1))
	if backoffDelay > 1*time.Hour {
		backoffDelay = 1 * time.Hour // Cap at 1 hour
	}

	retryTime := time.Now().Add(backoffDelay)

	w.logger.Info("Scheduling email job retry",
		"worker_id", w.id,
		"job_id", job.ID,
		"retry_time", retryTime,
		"backoff_delay", backoffDelay,
		"attempt", job.Attempts)

	// Reset scheduled time for retry
	job.ScheduledAt = retryTime

	// Re-queue the job (in a real implementation, you'd use a delayed queue or scheduler)
	go func() {
		time.Sleep(backoffDelay)
		if err := w.queue.Push(job); err != nil {
			w.logger.Error("Failed to re-queue job for retry",
				"worker_id", w.id,
				"job_id", job.ID,
				"error", err)
		}
	}()
}

// updateAverageTime updates the average processing time
func (w *EmailSenderWorker) updateAverageTime(duration time.Duration) {
	if w.stats.EmailsProcessed == 1 {
		w.stats.AverageTime = duration
	} else {
		// Calculate rolling average
		w.stats.AverageTime = time.Duration(
			(int64(w.stats.AverageTime)*int64(w.stats.EmailsProcessed-1) + int64(duration)) / int64(w.stats.EmailsProcessed),
		)
	}
}

// EmailWorkerPool manages multiple email sender workers
type EmailWorkerPool struct {
	workers     []*EmailSenderWorker
	queue       EmailQueue
	config      EmailWorkerConfig
	logger      *slog.Logger
	running     bool
	mutex       sync.RWMutex
}

// NewEmailWorkerPool creates a new email worker pool
func NewEmailWorkerPool(
	workerCount int,
	queue EmailQueue,
	emailUseCase *email_sending.UseCase,
	emailSender providers.EmailSender,
	config EmailWorkerConfig,
	logger *slog.Logger,
) *EmailWorkerPool {
	workers := make([]*EmailSenderWorker, workerCount)
	for i := 0; i < workerCount; i++ {
		workers[i] = NewEmailSenderWorker(
			i+1,
			queue,
			emailUseCase,
			emailSender,
			config,
			logger,
		)
	}

	return &EmailWorkerPool{
		workers: workers,
		queue:   queue,
		config:  config,
		logger:  logger,
	}
}

// Start starts all workers in the pool
func (p *EmailWorkerPool) Start(ctx context.Context) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.running {
		return
	}

	p.logger.Info("Starting email worker pool",
		"worker_count", len(p.workers))

	p.running = true

	for _, worker := range p.workers {
		worker.Start(ctx)
	}
}

// Stop stops all workers in the pool
func (p *EmailWorkerPool) Stop() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if !p.running {
		return
	}

	p.logger.Info("Stopping email worker pool")

	for _, worker := range p.workers {
		worker.Stop()
	}

	p.running = false
}

// GetPoolStats returns aggregated stats for all workers
func (p *EmailWorkerPool) GetPoolStats() EmailPoolStats {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	var totalProcessed, totalSuccessful, totalFailed, totalRetried int64
	var totalAverageTime time.Duration
	runningWorkers := 0

	for _, worker := range p.workers {
		stats := worker.GetStats()
		totalProcessed += stats.EmailsProcessed
		totalSuccessful += stats.EmailsSuccessful
		totalFailed += stats.EmailsFailed
		totalRetried += stats.EmailsRetried
		totalAverageTime += stats.AverageTime

		if worker.IsRunning() {
			runningWorkers++
		}
	}

	// Calculate overall average
	var overallAverageTime time.Duration
	if len(p.workers) > 0 {
		overallAverageTime = totalAverageTime / time.Duration(len(p.workers))
	}

	return EmailPoolStats{
		TotalWorkers:    len(p.workers),
		RunningWorkers:  runningWorkers,
		EmailsProcessed: totalProcessed,
		EmailsSuccessful: totalSuccessful,
		EmailsFailed:    totalFailed,
		EmailsRetried:   totalRetried,
		AverageTime:     overallAverageTime,
		QueueSize:       p.queue.Size(),
	}
}

// EmailPoolStats contains aggregated email worker pool statistics
type EmailPoolStats struct {
	TotalWorkers     int           `json:"total_workers"`
	RunningWorkers   int           `json:"running_workers"`
	EmailsProcessed  int64         `json:"emails_processed"`
	EmailsSuccessful int64         `json:"emails_successful"`
	EmailsFailed     int64         `json:"emails_failed"`
	EmailsRetried    int64         `json:"emails_retried"`
	AverageTime      time.Duration `json:"average_time"`
	QueueSize        int           `json:"queue_size"`
}

// CreateEmailSendingJob creates a new email sending job
func CreateEmailSendingJob(req *email_sending.SendEmailRequest, priority int) *EmailSendingJob {
	return &EmailSendingJob{
		ID:           uuid.Must(uuid.NewV4()),
		Type:         EmailJobTypeSend,
		EmailRequest: req,
		Priority:     priority,
		ScheduledAt:  time.Now(),
		MaxAttempts:  3,
		CreatedAt:    time.Now(),
	}
}

// CreateEmailRetryJob creates a new email retry job
func CreateEmailRetryJob(sentEmailID uuid.UUID, priority int) *EmailSendingJob {
	return &EmailSendingJob{
		ID:          uuid.Must(uuid.NewV4()),
		Type:        EmailJobTypeRetry,
		SentEmailID: sentEmailID,
		Priority:    priority,
		ScheduledAt: time.Now(),
		MaxAttempts: 3,
		CreatedAt:   time.Now(),
	}
}