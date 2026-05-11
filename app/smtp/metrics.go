package smtp

import (
	"log/slog"
	"net"
	"time"

	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// SMTPMetricsConfig holds SMTP metrics configuration
type SMTPMetricsConfig struct {
	// Namespace for metrics
	Namespace string

	// Subsystem for SMTP metrics
	Subsystem string

	// Enable detailed domain metrics
	EnableDomainMetrics bool

	// Enable detailed verification metrics
	EnableVerificationMetrics bool

	// Enable connection tracking
	EnableConnectionMetrics bool

	// Buckets for duration metrics
	DurationBuckets []float64

	// Buckets for size metrics
	SizeBuckets []float64

	// Logger for metrics errors
	Logger *slog.Logger
}

// DefaultSMTPMetricsConfig returns production-ready SMTP metrics configuration
func DefaultSMTPMetricsConfig() SMTPMetricsConfig {
	return SMTPMetricsConfig{
		Namespace:                 "mailvault",
		Subsystem:                 "smtp",
		EnableDomainMetrics:       true,
		EnableVerificationMetrics: true,
		EnableConnectionMetrics:   true,
		DurationBuckets: []float64{
			0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0, 60.0,
		},
		SizeBuckets: []float64{
			1024, 10240, 102400, 1048576, 10485760, 104857600, // 1KB to 100MB
		},
	}
}

// SMTPMetrics provides comprehensive SMTP server metrics
type SMTPMetrics struct {
	config   SMTPMetricsConfig
	logger   *slog.Logger
	registry *prometheus.Registry

	// Connection metrics
	connectionsTotal   *prometheus.CounterVec
	connectionsActive  prometheus.Gauge
	connectionDuration *prometheus.HistogramVec
	connectionErrors   *prometheus.CounterVec

	// Session metrics
	sessionsTotal   *prometheus.CounterVec
	sessionDuration *prometheus.HistogramVec
	sessionErrors   *prometheus.CounterVec

	// Email processing metrics
	emailsReceived      *prometheus.CounterVec
	emailsProcessed     *prometheus.CounterVec
	emailProcessingTime *prometheus.HistogramVec
	emailSize           *prometheus.HistogramVec
	emailsRejected      *prometheus.CounterVec
	emailsDeferred      *prometheus.CounterVec

	// Verification metrics
	verificationChecks   *prometheus.CounterVec
	verificationDuration *prometheus.HistogramVec
	spfChecks            *prometheus.CounterVec
	dkimChecks           *prometheus.CounterVec
	dmarcChecks          *prometheus.CounterVec

	// Domain metrics
	domainEmails *prometheus.CounterVec
	domainErrors *prometheus.CounterVec

	// Queue metrics (if applicable)
	queueSize           prometheus.Gauge
	queueProcessingTime *prometheus.HistogramVec

	// Resource metrics
	memoryUsage    prometheus.Gauge
	goroutineCount prometheus.Gauge
}

// NewSMTPMetrics creates a new SMTP metrics collector
func NewSMTPMetrics(config SMTPMetricsConfig) *SMTPMetrics {
	m := &SMTPMetrics{
		config:   config,
		logger:   config.Logger,
		registry: prometheus.NewRegistry(),
	}

	m.initializeMetrics()
	return m
}

// initializeMetrics sets up all SMTP Prometheus metrics
func (m *SMTPMetrics) initializeMetrics() {
	// Connection metrics
	m.connectionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "connections_total",
			Help:      "Total number of SMTP connections",
		},
		[]string{"status", "remote_ip"},
	)

	m.connectionsActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "connections_active",
			Help:      "Number of active SMTP connections",
		},
	)

	m.connectionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "connection_duration_seconds",
			Help:      "SMTP connection duration in seconds",
			Buckets:   m.config.DurationBuckets,
		},
		[]string{"status"},
	)

	m.connectionErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "connection_errors_total",
			Help:      "Total number of SMTP connection errors",
		},
		[]string{"error_type", "remote_ip"},
	)

	// Session metrics
	m.sessionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "sessions_total",
			Help:      "Total number of SMTP sessions",
		},
		[]string{"status", "command"},
	)

	m.sessionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "session_duration_seconds",
			Help:      "SMTP session duration in seconds",
			Buckets:   m.config.DurationBuckets,
		},
		[]string{"status"},
	)

	m.sessionErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "session_errors_total",
			Help:      "Total number of SMTP session errors",
		},
		[]string{"error_type", "command"},
	)

	// Email processing metrics
	m.emailsReceived = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "emails_received_total",
			Help:      "Total number of emails received",
		},
		[]string{"domain", "status"},
	)

	m.emailsProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "emails_processed_total",
			Help:      "Total number of emails processed",
		},
		[]string{"domain", "result"},
	)

	m.emailProcessingTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "email_processing_duration_seconds",
			Help:      "Email processing duration in seconds",
			Buckets:   m.config.DurationBuckets,
		},
		[]string{"domain", "stage"},
	)

	m.emailSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "email_size_bytes",
			Help:      "Email size in bytes",
			Buckets:   m.config.SizeBuckets,
		},
		[]string{"domain"},
	)

	m.emailsRejected = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "emails_rejected_total",
			Help:      "Total number of emails rejected",
		},
		[]string{"domain", "reason"},
	)

	m.emailsDeferred = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "emails_deferred_total",
			Help:      "Total number of emails deferred",
		},
		[]string{"domain", "reason"},
	)

	// Verification metrics
	if m.config.EnableVerificationMetrics {
		m.verificationChecks = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: m.config.Namespace,
				Subsystem: m.config.Subsystem,
				Name:      "verification_checks_total",
				Help:      "Total number of verification checks performed",
			},
			[]string{"type", "result"},
		)

		m.verificationDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: m.config.Namespace,
				Subsystem: m.config.Subsystem,
				Name:      "verification_duration_seconds",
				Help:      "Email verification duration in seconds",
				Buckets:   []float64{0.1, 0.25, 0.5, 1.0, 2.0, 5.0},
			},
			[]string{"type"},
		)

		m.spfChecks = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: m.config.Namespace,
				Subsystem: m.config.Subsystem,
				Name:      "spf_checks_total",
				Help:      "Total number of SPF checks",
			},
			[]string{"result"},
		)

		m.dkimChecks = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: m.config.Namespace,
				Subsystem: m.config.Subsystem,
				Name:      "dkim_checks_total",
				Help:      "Total number of DKIM checks",
			},
			[]string{"result"},
		)

		m.dmarcChecks = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: m.config.Namespace,
				Subsystem: m.config.Subsystem,
				Name:      "dmarc_checks_total",
				Help:      "Total number of DMARC checks",
			},
			[]string{"result", "policy"},
		)
	}

	// Domain metrics
	if m.config.EnableDomainMetrics {
		m.domainEmails = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: m.config.Namespace,
				Subsystem: m.config.Subsystem,
				Name:      "domain_emails_total",
				Help:      "Total number of emails per domain",
			},
			[]string{"domain", "status"},
		)

		m.domainErrors = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: m.config.Namespace,
				Subsystem: m.config.Subsystem,
				Name:      "domain_errors_total",
				Help:      "Total number of domain-related errors",
			},
			[]string{"domain", "error_type"},
		)
	}

	// Queue metrics
	m.queueSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "queue_size",
			Help:      "Number of emails in processing queue",
		},
	)

	m.queueProcessingTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "queue_processing_duration_seconds",
			Help:      "Queue processing duration in seconds",
			Buckets:   m.config.DurationBuckets,
		},
		[]string{"operation"},
	)

	// Resource metrics
	m.memoryUsage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "memory_usage_bytes",
			Help:      "SMTP server memory usage in bytes",
		},
	)

	m.goroutineCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "goroutines",
			Help:      "Number of goroutines in SMTP server",
		},
	)

	// Register all metrics
	m.registerMetrics()
}

// registerMetrics registers all metrics with the registry
func (m *SMTPMetrics) registerMetrics() {
	// Connection metrics
	m.registry.MustRegister(
		m.connectionsTotal,
		m.connectionsActive,
		m.connectionDuration,
		m.connectionErrors,
	)

	// Session metrics
	m.registry.MustRegister(
		m.sessionsTotal,
		m.sessionDuration,
		m.sessionErrors,
	)

	// Email metrics
	m.registry.MustRegister(
		m.emailsReceived,
		m.emailsProcessed,
		m.emailProcessingTime,
		m.emailSize,
		m.emailsRejected,
		m.emailsDeferred,
	)

	// Verification metrics
	if m.config.EnableVerificationMetrics {
		m.registry.MustRegister(
			m.verificationChecks,
			m.verificationDuration,
			m.spfChecks,
			m.dkimChecks,
			m.dmarcChecks,
		)
	}

	// Domain metrics
	if m.config.EnableDomainMetrics {
		m.registry.MustRegister(
			m.domainEmails,
			m.domainErrors,
		)
	}

	// Queue and resource metrics
	m.registry.MustRegister(
		m.queueSize,
		m.queueProcessingTime,
		m.memoryUsage,
		m.goroutineCount,
	)
}

// Connection tracking methods
func (m *SMTPMetrics) RecordConnection(remoteIP string, status string) {
	m.connectionsTotal.WithLabelValues(status, remoteIP).Inc()
}

func (m *SMTPMetrics) RecordConnectionStart() {
	m.connectionsActive.Inc()
}

func (m *SMTPMetrics) RecordConnectionEnd(duration time.Duration, status string) {
	m.connectionsActive.Dec()
	m.connectionDuration.WithLabelValues(status).Observe(duration.Seconds())
}

func (m *SMTPMetrics) RecordConnectionError(errorType, remoteIP string) {
	m.connectionErrors.WithLabelValues(errorType, remoteIP).Inc()
}

// Session tracking methods
func (m *SMTPMetrics) RecordSession(status, command string) {
	m.sessionsTotal.WithLabelValues(status, command).Inc()
}

func (m *SMTPMetrics) RecordSessionDuration(duration time.Duration, status string) {
	m.sessionDuration.WithLabelValues(status).Observe(duration.Seconds())
}

func (m *SMTPMetrics) RecordSessionError(errorType, command string) {
	m.sessionErrors.WithLabelValues(errorType, command).Inc()
}

// Email processing methods
func (m *SMTPMetrics) RecordEmailReceived(domain, status string) {
	m.emailsReceived.WithLabelValues(domain, status).Inc()
}

func (m *SMTPMetrics) RecordEmailProcessed(domain, result string) {
	m.emailsProcessed.WithLabelValues(domain, result).Inc()
}

func (m *SMTPMetrics) RecordEmailProcessingTime(domain, stage string, duration time.Duration) {
	m.emailProcessingTime.WithLabelValues(domain, stage).Observe(duration.Seconds())
}

func (m *SMTPMetrics) RecordEmailSize(domain string, size int) {
	m.emailSize.WithLabelValues(domain).Observe(float64(size))
}

func (m *SMTPMetrics) RecordEmailRejected(domain, reason string) {
	m.emailsRejected.WithLabelValues(domain, reason).Inc()
}

func (m *SMTPMetrics) RecordEmailDeferred(domain, reason string) {
	m.emailsDeferred.WithLabelValues(domain, reason).Inc()
}

// Verification methods
func (m *SMTPMetrics) RecordVerificationCheck(checkType, result string) {
	if m.verificationChecks != nil {
		m.verificationChecks.WithLabelValues(checkType, result).Inc()
	}
}

func (m *SMTPMetrics) RecordVerificationDuration(checkType string, duration time.Duration) {
	if m.verificationDuration != nil {
		m.verificationDuration.WithLabelValues(checkType).Observe(duration.Seconds())
	}
}

func (m *SMTPMetrics) RecordSPFCheck(result string) {
	if m.spfChecks != nil {
		m.spfChecks.WithLabelValues(result).Inc()
	}
}

func (m *SMTPMetrics) RecordDKIMCheck(result string) {
	if m.dkimChecks != nil {
		m.dkimChecks.WithLabelValues(result).Inc()
	}
}

func (m *SMTPMetrics) RecordDMARCCheck(result, policy string) {
	if m.dmarcChecks != nil {
		m.dmarcChecks.WithLabelValues(result, policy).Inc()
	}
}

// Domain methods
func (m *SMTPMetrics) RecordDomainEmail(domain, status string) {
	if m.domainEmails != nil {
		m.domainEmails.WithLabelValues(domain, status).Inc()
	}
}

func (m *SMTPMetrics) RecordDomainError(domain, errorType string) {
	if m.domainErrors != nil {
		m.domainErrors.WithLabelValues(domain, errorType).Inc()
	}
}

// Queue methods
func (m *SMTPMetrics) SetQueueSize(size float64) {
	m.queueSize.Set(size)
}

func (m *SMTPMetrics) RecordQueueProcessingTime(operation string, duration time.Duration) {
	m.queueProcessingTime.WithLabelValues(operation).Observe(duration.Seconds())
}

// Resource methods
func (m *SMTPMetrics) SetMemoryUsage(bytes float64) {
	m.memoryUsage.Set(bytes)
}

func (m *SMTPMetrics) SetGoroutineCount(count float64) {
	m.goroutineCount.Set(count)
}

// Helper methods
func (m *SMTPMetrics) GetRemoteIP(conn net.Conn) string {
	if conn == nil {
		return "unknown"
	}
	addr := conn.RemoteAddr()
	if addr == nil {
		return "unknown"
	}

	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		return tcpAddr.IP.String()
	}

	return addr.String()
}

// PrometheusHandler returns the HTTP handler for SMTP metrics
func (m *SMTPMetrics) PrometheusHandler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// GetRegistry returns the Prometheus registry
func (m *SMTPMetrics) GetRegistry() *prometheus.Registry {
	return m.registry
}

// GetMetricsInfo returns information about configured SMTP metrics
func (m *SMTPMetrics) GetMetricsInfo() map[string]interface{} {
	return map[string]interface{}{
		"namespace":                    m.config.Namespace,
		"subsystem":                    m.config.Subsystem,
		"domain_metrics_enabled":       m.config.EnableDomainMetrics,
		"verification_metrics_enabled": m.config.EnableVerificationMetrics,
		"connection_metrics_enabled":   m.config.EnableConnectionMetrics,
		"duration_buckets":             m.config.DurationBuckets,
		"size_buckets":                 m.config.SizeBuckets,
	}
}
