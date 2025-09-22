package worker

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// WorkerMetricsConfig holds worker metrics configuration
type WorkerMetricsConfig struct {
	// Namespace for metrics (e.g., "mailvault")
	Namespace string

	// Subsystem for worker metrics (e.g., "worker")
	Subsystem string

	// Enable detailed job metrics
	EnableJobMetrics bool

	// Enable queue metrics
	EnableQueueMetrics bool

	// Enable DNS metrics
	EnableDNSMetrics bool

	// Buckets for histogram metrics
	DurationBuckets []float64
}

// DefaultWorkerMetricsConfig returns production-ready worker metrics configuration
func DefaultWorkerMetricsConfig() WorkerMetricsConfig {
	return WorkerMetricsConfig{
		Namespace:          "mailvault",
		Subsystem:          "worker",
		EnableJobMetrics:   true,
		EnableQueueMetrics: true,
		EnableDNSMetrics:   true,
		DurationBuckets: []float64{
			0.1, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0, 60.0, 120.0, 300.0,
		},
	}
}

// WorkerMetrics provides comprehensive Prometheus metrics for worker operations
type WorkerMetrics struct {
	config WorkerMetricsConfig

	// Job processing metrics
	jobsProcessedTotal     *prometheus.CounterVec
	jobsSuccessfulTotal    *prometheus.CounterVec
	jobsFailedTotal        *prometheus.CounterVec
	jobsRetriedTotal       *prometheus.CounterVec
	jobProcessingDuration  *prometheus.HistogramVec
	jobsInProgress         prometheus.Gauge

	// Queue metrics
	queueSize              prometheus.Gauge
	queuePushTotal         *prometheus.CounterVec
	queuePopTotal          *prometheus.CounterVec
	scheduledJobsTotal     prometheus.Gauge

	// Worker pool metrics
	workersActive          prometheus.Gauge
	workersTotal           prometheus.Gauge

	// DNS validation metrics
	dnsQueriesTotal        *prometheus.CounterVec
	dnsQueryDuration       *prometheus.HistogramVec
	validationAttemptsTotal *prometheus.CounterVec
	domainsValidatedTotal  *prometheus.CounterVec

	// Manager metrics
	managerUptime          prometheus.Gauge
	managerStatsUpdates    prometheus.Counter

	// Registry for metrics
	registry *prometheus.Registry
}

// NewWorkerMetrics creates a new worker metrics instance
func NewWorkerMetrics(config WorkerMetricsConfig) *WorkerMetrics {
	m := &WorkerMetrics{
		config:   config,
		registry: prometheus.NewRegistry(),
	}

	m.initializeMetrics()
	return m
}

// initializeMetrics sets up all Prometheus metrics
func (m *WorkerMetrics) initializeMetrics() {
	// Job processing metrics
	m.jobsProcessedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "jobs_processed_total",
			Help:      "Total number of validation jobs processed",
		},
		[]string{"job_type", "status"},
	)

	m.jobsSuccessfulTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "jobs_successful_total",
			Help:      "Total number of successful validation jobs",
		},
		[]string{"job_type"},
	)

	m.jobsFailedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "jobs_failed_total",
			Help:      "Total number of failed validation jobs",
		},
		[]string{"job_type", "error_type"},
	)

	m.jobsRetriedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "jobs_retried_total",
			Help:      "Total number of retried validation jobs",
		},
		[]string{"job_type"},
	)

	m.jobProcessingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "job_processing_duration_seconds",
			Help:      "Time spent processing validation jobs",
			Buckets:   m.config.DurationBuckets,
		},
		[]string{"job_type"},
	)

	m.jobsInProgress = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "jobs_in_progress",
			Help:      "Number of validation jobs currently being processed",
		},
	)

	// Queue metrics
	m.queueSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "queue_size",
			Help:      "Current number of jobs in the queue",
		},
	)

	m.queuePushTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "queue_push_total",
			Help:      "Total number of jobs pushed to queue",
		},
		[]string{"status"},
	)

	m.queuePopTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "queue_pop_total",
			Help:      "Total number of jobs popped from queue",
		},
		[]string{"status"},
	)

	m.scheduledJobsTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "scheduled_jobs_total",
			Help:      "Number of jobs scheduled for future execution",
		},
	)

	// Worker pool metrics
	m.workersActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "workers_active",
			Help:      "Number of active workers",
		},
	)

	m.workersTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "workers_total",
			Help:      "Total number of workers in pool",
		},
	)

	// DNS validation metrics
	m.dnsQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: "dns",
			Name:      "queries_total",
			Help:      "Total number of DNS queries performed",
		},
		[]string{"query_type", "status"},
	)

	m.dnsQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: m.config.Namespace,
			Subsystem: "dns",
			Name:      "query_duration_seconds",
			Help:      "Time spent performing DNS queries",
			Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
		},
		[]string{"query_type"},
	)

	m.validationAttemptsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: "validation",
			Name:      "attempts_total",
			Help:      "Total number of domain validation attempts",
		},
		[]string{"validation_type", "status"},
	)

	m.domainsValidatedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: "validation",
			Name:      "domains_validated_total",
			Help:      "Total number of domains validated",
		},
		[]string{"status"},
	)

	// Manager metrics
	m.managerUptime = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "manager_uptime_seconds",
			Help:      "Worker manager uptime in seconds",
		},
	)

	m.managerStatsUpdates = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "manager_stats_updates_total",
			Help:      "Total number of manager stats updates",
		},
	)

	// Register all metrics
	m.registry.MustRegister(
		m.jobsProcessedTotal,
		m.jobsSuccessfulTotal,
		m.jobsFailedTotal,
		m.jobsRetriedTotal,
		m.jobProcessingDuration,
		m.jobsInProgress,
		m.queueSize,
		m.queuePushTotal,
		m.queuePopTotal,
		m.scheduledJobsTotal,
		m.workersActive,
		m.workersTotal,
		m.dnsQueriesTotal,
		m.dnsQueryDuration,
		m.validationAttemptsTotal,
		m.domainsValidatedTotal,
		m.managerUptime,
		m.managerStatsUpdates,
	)
}

// Job processing metric methods
func (m *WorkerMetrics) RecordJobProcessed(jobType, status string) {
	m.jobsProcessedTotal.WithLabelValues(jobType, status).Inc()
}

func (m *WorkerMetrics) RecordJobSuccess(jobType string) {
	m.jobsSuccessfulTotal.WithLabelValues(jobType).Inc()
}

func (m *WorkerMetrics) RecordJobFailure(jobType, errorType string) {
	m.jobsFailedTotal.WithLabelValues(jobType, errorType).Inc()
}

func (m *WorkerMetrics) RecordJobRetry(jobType string) {
	m.jobsRetriedTotal.WithLabelValues(jobType).Inc()
}

func (m *WorkerMetrics) RecordJobDuration(jobType string, duration time.Duration) {
	m.jobProcessingDuration.WithLabelValues(jobType).Observe(duration.Seconds())
}

func (m *WorkerMetrics) SetJobsInProgress(count float64) {
	m.jobsInProgress.Set(count)
}

// Queue metric methods
func (m *WorkerMetrics) SetQueueSize(size float64) {
	m.queueSize.Set(size)
}

func (m *WorkerMetrics) RecordQueuePush(status string) {
	m.queuePushTotal.WithLabelValues(status).Inc()
}

func (m *WorkerMetrics) RecordQueuePop(status string) {
	m.queuePopTotal.WithLabelValues(status).Inc()
}

func (m *WorkerMetrics) SetScheduledJobs(count float64) {
	m.scheduledJobsTotal.Set(count)
}

// Worker pool metric methods
func (m *WorkerMetrics) SetWorkersActive(count float64) {
	m.workersActive.Set(count)
}

func (m *WorkerMetrics) SetWorkersTotal(count float64) {
	m.workersTotal.Set(count)
}

// DNS validation metric methods
func (m *WorkerMetrics) RecordDNSQuery(queryType, status string, duration time.Duration) {
	m.dnsQueriesTotal.WithLabelValues(queryType, status).Inc()
	m.dnsQueryDuration.WithLabelValues(queryType).Observe(duration.Seconds())
}

func (m *WorkerMetrics) RecordValidationAttempt(validationType, status string) {
	m.validationAttemptsTotal.WithLabelValues(validationType, status).Inc()
}

func (m *WorkerMetrics) RecordDomainValidated(status string) {
	m.domainsValidatedTotal.WithLabelValues(status).Inc()
}

// Manager metric methods
func (m *WorkerMetrics) SetManagerUptime(uptime time.Duration) {
	m.managerUptime.Set(uptime.Seconds())
}

func (m *WorkerMetrics) RecordManagerStatsUpdate() {
	m.managerStatsUpdates.Inc()
}

// PrometheusHandler returns the HTTP handler for Prometheus metrics endpoint
func (m *WorkerMetrics) PrometheusHandler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// GetRegistry returns the Prometheus registry for external use
func (m *WorkerMetrics) GetRegistry() *prometheus.Registry {
	return m.registry
}

// CollectManagerStats updates manager-related metrics
func (m *WorkerMetrics) CollectManagerStats(stats DetailedStats, managerStartTime time.Time) {
	// Update job metrics
	m.SetJobsInProgress(float64(stats.WorkerPool.RunningWorkers))
	m.SetQueueSize(float64(stats.QueueSize))
	m.SetScheduledJobs(float64(stats.ScheduledJobs))

	// Update worker metrics
	m.SetWorkersActive(float64(stats.WorkerPool.RunningWorkers))
	m.SetWorkersTotal(float64(stats.WorkerPool.TotalWorkers))

	// Update manager uptime
	uptime := time.Since(managerStartTime)
	m.SetManagerUptime(uptime)

	// Record stats update
	m.RecordManagerStatsUpdate()
}

// GetMetricsInfo returns information about configured metrics
func (m *WorkerMetrics) GetMetricsInfo() map[string]interface{} {
	return map[string]interface{}{
		"namespace":            m.config.Namespace,
		"subsystem":            m.config.Subsystem,
		"job_metrics_enabled":  m.config.EnableJobMetrics,
		"queue_metrics_enabled": m.config.EnableQueueMetrics,
		"dns_metrics_enabled":  m.config.EnableDNSMetrics,
		"duration_buckets":     m.config.DurationBuckets,
	}
}