package webhook

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// AsyncWebhookWorker handles asynchronous webhook delivery with retry logic
type AsyncWebhookWorker struct {
	config           AsyncWorkerConfig
	webhookQueue     chan WebhookJob
	retryQueue       chan WebhookJob
	workers          []worker
	retryTimer       *time.Timer
	stopCh           chan struct{}
	wg               sync.WaitGroup
	mu               sync.RWMutex
	stats            WorkerStats
	running          bool
}

// AsyncWorkerConfig configures the async webhook worker
type AsyncWorkerConfig struct {
	BufferSize       int                // Size of webhook queue buffer
	WorkerCount      int                // Number of concurrent workers
	RetryInterval    time.Duration      // Interval between retry attempts
	MaxRetryAge      time.Duration      // Maximum age before giving up on retries
	Logger           *slog.Logger       // Logger instance
	HTTPClient       *HTTPClient        // HTTP client for webhook delivery
	MetricsCollector *MetricsCollector  // Metrics collector
}

// WebhookJob represents a webhook delivery job
type WebhookJob struct {
	Request       WebhookRequest     `json:"request"`
	Event         *IncomingEmailEvent `json:"event"`
	CreatedAt     time.Time          `json:"created_at"`
	LastAttemptAt time.Time          `json:"last_attempt_at"`
	Attempts      int                `json:"attempts"`
	MaxRetries    int                `json:"max_retries"`
	NextRetryAt   time.Time          `json:"next_retry_at"`
}

// worker represents a webhook worker goroutine
type worker struct {
	id       int
	stopCh   chan struct{}
	jobsCh   chan WebhookJob
	retryCh  chan WebhookJob
	client   *HTTPClient
	logger   *slog.Logger
	metrics  *MetricsCollector
}

// WorkerStats represents worker statistics
type WorkerStats struct {
	QueueSize       int   `json:"queue_size"`
	PendingRetries  int   `json:"pending_retries"`
	ActiveWorkers   int   `json:"active_workers"`
	TotalProcessed  int64 `json:"total_processed"`
	TotalRetries    int64 `json:"total_retries"`
	TotalAbandoned  int64 `json:"total_abandoned"`
}

// NewAsyncWebhookWorker creates a new async webhook worker
func NewAsyncWebhookWorker(config AsyncWorkerConfig) *AsyncWebhookWorker {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	return &AsyncWebhookWorker{
		config:       config,
		webhookQueue: make(chan WebhookJob, config.BufferSize),
		retryQueue:   make(chan WebhookJob, config.BufferSize),
		stopCh:       make(chan struct{}),
		stats:        WorkerStats{},
	}
}

// Start starts the async webhook worker
func (w *AsyncWebhookWorker) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		return nil
	}

	w.config.Logger.Info("starting async webhook worker",
		slog.Int("worker_count", w.config.WorkerCount),
		slog.Int("buffer_size", w.config.BufferSize))

	// Start workers
	w.workers = make([]worker, w.config.WorkerCount)
	for i := 0; i < w.config.WorkerCount; i++ {
		w.workers[i] = worker{
			id:       i,
			stopCh:   make(chan struct{}),
			jobsCh:   w.webhookQueue,
			retryCh:  w.retryQueue,
			client:   w.config.HTTPClient,
			logger:   w.config.Logger,
			metrics:  w.config.MetricsCollector,
		}

		w.wg.Add(1)
		go w.runWorker(ctx, &w.workers[i])
	}

	// Start retry processor
	w.wg.Add(1)
	go w.runRetryProcessor(ctx)

	w.running = true
	return nil
}

// Stop stops the async webhook worker
func (w *AsyncWebhookWorker) Stop(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return nil
	}

	w.config.Logger.Info("stopping async webhook worker")

	// Signal all workers to stop
	close(w.stopCh)

	// Stop retry timer
	if w.retryTimer != nil {
		w.retryTimer.Stop()
	}

	// Wait for all workers to finish
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	// Wait for graceful shutdown or timeout
	select {
	case <-done:
		w.config.Logger.Info("async webhook worker stopped gracefully")
	case <-ctx.Done():
		w.config.Logger.Warn("async webhook worker stop timed out")
	}

	w.running = false
	return nil
}

// EnqueueWebhook enqueues a webhook for async delivery
func (w *AsyncWebhookWorker) EnqueueWebhook(ctx context.Context, request WebhookRequest, event *IncomingEmailEvent) error {
	job := WebhookJob{
		Request:     request,
		Event:       event,
		CreatedAt:   time.Now(),
		MaxRetries:  w.config.HTTPClient.maxRetries,
	}

	select {
	case w.webhookQueue <- job:
		w.updateStats(func(s *WorkerStats) {
			s.QueueSize++
		})
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Queue is full
		w.config.Logger.Warn("webhook queue is full, dropping webhook",
			slog.String("event_id", event.EventID.String()),
			slog.String("url", request.URL))

		if w.config.MetricsCollector != nil {
			w.config.MetricsCollector.RecordWebhookFailure("incoming_email", event.Domain.Name, "queue_full")
		}

		return fmt.Errorf("webhook queue is full")
	}
}

// runWorker runs a single webhook worker
func (w *AsyncWebhookWorker) runWorker(ctx context.Context, worker *worker) {
	defer w.wg.Done()

	worker.logger.Debug("webhook worker started", slog.Int("worker_id", worker.id))

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-worker.stopCh:
			return
		case job := <-worker.jobsCh:
			w.processWebhookJob(ctx, worker, job)
		case job := <-worker.retryCh:
			w.processWebhookJob(ctx, worker, job)
		}
	}
}

// processWebhookJob processes a single webhook job
func (w *AsyncWebhookWorker) processWebhookJob(ctx context.Context, worker *worker, job WebhookJob) {
	start := time.Now()
	job.Attempts++
	job.LastAttemptAt = start

	worker.logger.Debug("processing webhook job",
		slog.String("event_id", job.Event.EventID.String()),
		slog.String("url", job.Request.URL),
		slog.Int("attempt", job.Attempts))

	// Create context with timeout
	jobCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Send webhook
	response, err := worker.client.SendWebhook(jobCtx, job.Request)
	duration := time.Since(start)

	// Update queue size
	w.updateStats(func(s *WorkerStats) {
		s.QueueSize--
		s.TotalProcessed++
	})

	if err != nil || !response.Success {
		// Handle failure
		worker.logger.Warn("webhook job failed",
			slog.String("event_id", job.Event.EventID.String()),
			slog.String("url", job.Request.URL),
			slog.Int("attempt", job.Attempts),
			slog.Int("max_retries", job.MaxRetries),
			slog.Duration("duration", duration),
			slog.String("error", func() string {
				if err != nil {
					return err.Error()
				}
				return response.Error
			}()))

		// Check if we should retry
		if job.Attempts < job.MaxRetries && time.Since(job.CreatedAt) < w.config.MaxRetryAge {
			// Schedule retry
			job.NextRetryAt = time.Now().Add(w.calculateRetryDelay(job.Attempts))

			select {
			case w.retryQueue <- job:
				w.updateStats(func(s *WorkerStats) {
					s.PendingRetries++
					s.TotalRetries++
				})

				worker.logger.Info("webhook job scheduled for retry",
					slog.String("event_id", job.Event.EventID.String()),
					slog.Time("next_retry_at", job.NextRetryAt))
			default:
				// Retry queue is full, abandon
				w.abandonWebhookJob(worker, job, "retry_queue_full")
			}
		} else {
			// Give up on this job
			w.abandonWebhookJob(worker, job, "max_retries_exceeded")
		}

		// Record failure metrics
		if worker.metrics != nil {
			worker.metrics.RecordWebhookFailure("incoming_email", job.Event.Domain.Name, "delivery_failed")
			worker.metrics.RecordWebhookDuration("incoming_email", job.Event.Domain.Name, duration, false)
		}
	} else {
		// Success
		worker.logger.Info("webhook job completed successfully",
			slog.String("event_id", job.Event.EventID.String()),
			slog.String("url", job.Request.URL),
			slog.Int("attempts", job.Attempts),
			slog.Duration("duration", duration))

		// Record success metrics
		if worker.metrics != nil {
			worker.metrics.RecordWebhookSuccess("incoming_email", job.Event.Domain.Name)
			worker.metrics.RecordWebhookDuration("incoming_email", job.Event.Domain.Name, duration, true)
			worker.metrics.RecordWebhookRetries("incoming_email", job.Event.Domain.Name, job.Attempts)
		}
	}
}

// runRetryProcessor processes retry jobs
func (w *AsyncWebhookWorker) runRetryProcessor(ctx context.Context) {
	defer w.wg.Done()

	w.retryTimer = time.NewTimer(w.config.RetryInterval)
	defer w.retryTimer.Stop()

	retryJobs := make([]WebhookJob, 0)

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case job := <-w.retryQueue:
			retryJobs = append(retryJobs, job)
		case <-w.retryTimer.C:
			// Process pending retries
			now := time.Now()
			remaining := make([]WebhookJob, 0, len(retryJobs))

			for _, job := range retryJobs {
				if job.NextRetryAt.Before(now) || job.NextRetryAt.Equal(now) {
					// Time to retry
					select {
					case w.webhookQueue <- job:
						w.updateStats(func(s *WorkerStats) {
							s.PendingRetries--
							s.QueueSize++
						})
					default:
						// Main queue is full, keep in retry queue
						remaining = append(remaining, job)
					}
				} else {
					// Not time yet
					remaining = append(remaining, job)
				}
			}

			retryJobs = remaining
			w.retryTimer.Reset(w.config.RetryInterval)
		}
	}
}

// abandonWebhookJob abandons a webhook job that can't be delivered
func (w *AsyncWebhookWorker) abandonWebhookJob(worker *worker, job WebhookJob, reason string) {
	worker.logger.Error("abandoning webhook job",
		slog.String("event_id", job.Event.EventID.String()),
		slog.String("url", job.Request.URL),
		slog.Int("attempts", job.Attempts),
		slog.String("reason", reason),
		slog.Duration("age", time.Since(job.CreatedAt)))

	w.updateStats(func(s *WorkerStats) {
		s.TotalAbandoned++
	})

	if worker.metrics != nil {
		worker.metrics.RecordWebhookFailure("incoming_email", job.Event.Domain.Name, reason)
	}
}

// calculateRetryDelay calculates exponential backoff delay for retries
func (w *AsyncWebhookWorker) calculateRetryDelay(attempt int) time.Duration {
	// Exponential backoff: 1s, 2s, 4s, 8s, 16s, then 30s
	delay := time.Duration(1<<(attempt-1)) * time.Second
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}
	return delay
}

// updateStats safely updates worker statistics
func (w *AsyncWebhookWorker) updateStats(updateFn func(*WorkerStats)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	updateFn(&w.stats)
}

// GetStats returns current worker statistics
func (w *AsyncWebhookWorker) GetStats() WorkerStats {
	w.mu.RLock()
	defer w.mu.RUnlock()

	stats := w.stats
	stats.ActiveWorkers = len(w.workers)
	return stats
}