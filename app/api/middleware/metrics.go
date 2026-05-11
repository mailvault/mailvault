package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsConfig holds metrics configuration
type MetricsConfig struct {
	// Namespace for metrics (e.g., "mailvault")
	Namespace string

	// Subsystem for API metrics (e.g., "api")
	Subsystem string

	// Enable detailed endpoint metrics
	EnableEndpointMetrics bool

	// Enable detailed status code metrics
	EnableStatusCodeMetrics bool

	// Enable request size metrics
	EnableRequestSizeMetrics bool

	// Enable response size metrics
	EnableResponseSizeMetrics bool

	// Buckets for histogram metrics
	DurationBuckets []float64
	SizeBuckets     []float64
}

// DefaultMetricsConfig returns production-ready metrics configuration
func DefaultMetricsConfig() MetricsConfig {
	return MetricsConfig{
		Namespace:                 "mailvault",
		Subsystem:                 "api",
		EnableEndpointMetrics:     true,
		EnableStatusCodeMetrics:   true,
		EnableRequestSizeMetrics:  true,
		EnableResponseSizeMetrics: true,
		DurationBuckets: []float64{
			0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0,
		},
		SizeBuckets: []float64{
			100, 1000, 10000, 100000, 1000000, 10000000,
		},
	}
}

// MetricsMiddleware provides comprehensive Prometheus metrics
type MetricsMiddleware struct {
	config MetricsConfig

	// Request metrics
	requestsTotal      *prometheus.CounterVec
	requestDuration    *prometheus.HistogramVec
	requestsInProgress prometheus.Gauge
	requestSize        *prometheus.HistogramVec
	responseSize       *prometheus.HistogramVec

	// Business metrics
	authAttemptsTotal   *prometheus.CounterVec
	emailsSentTotal     *prometheus.CounterVec
	domainsTotal        prometheus.Gauge
	emailAddressesTotal prometheus.Gauge
	receivedEmailsTotal *prometheus.CounterVec

	// Rate limiting metrics
	rateLimitHitsTotal *prometheus.CounterVec

	// Security metrics
	securityViolationsTotal *prometheus.CounterVec

	// Database metrics
	dbConnectionsActive prometheus.Gauge
	dbQueryDuration     *prometheus.HistogramVec
	dbQueryTotal        *prometheus.CounterVec

	// Registry for metrics
	registry *prometheus.Registry
}

// NewMetricsMiddleware creates a new metrics middleware
func NewMetricsMiddleware(config MetricsConfig) *MetricsMiddleware {
	m := &MetricsMiddleware{
		config:   config,
		registry: prometheus.NewRegistry(),
	}

	m.initializeMetrics()
	return m
}

// initializeMetrics sets up all Prometheus metrics
func (m *MetricsMiddleware) initializeMetrics() {
	// Request metrics
	m.requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status_code"},
	)

	m.requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   m.config.DurationBuckets,
		},
		[]string{"method", "endpoint"},
	)

	m.requestsInProgress = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "requests_in_progress",
			Help:      "Number of HTTP requests currently being processed",
		},
	)

	if m.config.EnableRequestSizeMetrics {
		m.requestSize = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: m.config.Namespace,
				Subsystem: m.config.Subsystem,
				Name:      "request_size_bytes",
				Help:      "HTTP request size in bytes",
				Buckets:   m.config.SizeBuckets,
			},
			[]string{"method", "endpoint"},
		)
	}

	if m.config.EnableResponseSizeMetrics {
		m.responseSize = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: m.config.Namespace,
				Subsystem: m.config.Subsystem,
				Name:      "response_size_bytes",
				Help:      "HTTP response size in bytes",
				Buckets:   m.config.SizeBuckets,
			},
			[]string{"method", "endpoint"},
		)
	}

	// Business metrics
	m.authAttemptsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: "auth",
			Name:      "attempts_total",
			Help:      "Total number of authentication attempts",
		},
		[]string{"type", "status"},
	)

	m.emailsSentTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: "email",
			Name:      "sent_total",
			Help:      "Total number of emails sent",
		},
		[]string{"domain", "status"},
	)

	m.domainsTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: "domains",
			Name:      "total",
			Help:      "Total number of domains registered",
		},
	)

	m.emailAddressesTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: "email_addresses",
			Name:      "total",
			Help:      "Total number of email addresses configured",
		},
	)

	m.receivedEmailsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: "email",
			Name:      "received_total",
			Help:      "Total number of emails received",
		},
		[]string{"domain"},
	)

	// Rate limiting metrics
	m.rateLimitHitsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: "rate_limit",
			Name:      "hits_total",
			Help:      "Total number of rate limit hits",
		},
		[]string{"type", "endpoint"},
	)

	// Security metrics
	m.securityViolationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: "security",
			Name:      "violations_total",
			Help:      "Total number of security violations detected",
		},
		[]string{"type", "severity"},
	)

	// Database metrics
	m.dbConnectionsActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: "database",
			Name:      "connections_active",
			Help:      "Number of active database connections",
		},
	)

	m.dbQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: m.config.Namespace,
			Subsystem: "database",
			Name:      "query_duration_seconds",
			Help:      "Database query duration in seconds",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0},
		},
		[]string{"operation"},
	)

	m.dbQueryTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: "database",
			Name:      "queries_total",
			Help:      "Total number of database queries",
		},
		[]string{"operation", "status"},
	)

	// Register all metrics
	m.registry.MustRegister(
		m.requestsTotal,
		m.requestDuration,
		m.requestsInProgress,
		m.authAttemptsTotal,
		m.emailsSentTotal,
		m.domainsTotal,
		m.emailAddressesTotal,
		m.receivedEmailsTotal,
		m.rateLimitHitsTotal,
		m.securityViolationsTotal,
		m.dbConnectionsActive,
		m.dbQueryDuration,
		m.dbQueryTotal,
	)

	if m.requestSize != nil {
		m.registry.MustRegister(m.requestSize)
	}

	if m.responseSize != nil {
		m.registry.MustRegister(m.responseSize)
	}
}

// MetricsHandler returns the Prometheus metrics middleware
func (m *MetricsMiddleware) MetricsHandler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Track requests in progress
			m.requestsInProgress.Inc()
			defer m.requestsInProgress.Dec()

			// Wrap response writer to capture status code and size
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// Track request size
			if m.requestSize != nil && r.ContentLength > 0 {
				endpoint := m.getEndpointPattern(r)
				m.requestSize.WithLabelValues(r.Method, endpoint).Observe(float64(r.ContentLength))
			}

			// Process request
			next.ServeHTTP(ww, r)

			// Calculate duration
			duration := time.Since(start).Seconds()

			// Get endpoint pattern and status code
			endpoint := m.getEndpointPattern(r)
			statusCode := strconv.Itoa(ww.Status())

			// Record metrics
			m.requestsTotal.WithLabelValues(r.Method, endpoint, statusCode).Inc()
			m.requestDuration.WithLabelValues(r.Method, endpoint).Observe(duration)

			// Track response size
			if m.responseSize != nil {
				m.responseSize.WithLabelValues(r.Method, endpoint).Observe(float64(ww.BytesWritten()))
			}
		})
	}
}

// getEndpointPattern extracts the route pattern from the request
func (m *MetricsMiddleware) getEndpointPattern(r *http.Request) string {
	// Try to get route pattern from chi context
	if rctx := r.Context().Value(chi.RouteCtxKey); rctx != nil {
		if routeCtx, ok := rctx.(*chi.Context); ok && routeCtx.RoutePattern() != "" {
			return routeCtx.RoutePattern()
		}
	}

	// Fallback to path normalization
	path := r.URL.Path

	// Basic path normalization for common patterns
	if m.config.EnableEndpointMetrics {
		// Replace UUIDs with placeholder
		if uuidRegex := `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`; containsPattern(path, uuidRegex) {
			path = replacePattern(path, uuidRegex, "{id}")
		}

		// Replace numeric IDs with placeholder
		if numberRegex := `/\d+`; containsPattern(path, numberRegex) {
			path = replacePattern(path, numberRegex, "/{id}")
		}
	}

	return path
}

// Business metric helper methods
func (m *MetricsMiddleware) RecordAuthAttempt(authType, status string) {
	m.authAttemptsTotal.WithLabelValues(authType, status).Inc()
}

func (m *MetricsMiddleware) RecordEmailSent(domain, status string) {
	m.emailsSentTotal.WithLabelValues(domain, status).Inc()
}

func (m *MetricsMiddleware) RecordEmailReceived(domain string) {
	m.receivedEmailsTotal.WithLabelValues(domain).Inc()
}

func (m *MetricsMiddleware) SetDomainsTotal(count float64) {
	m.domainsTotal.Set(count)
}

func (m *MetricsMiddleware) SetEmailAddressesTotal(count float64) {
	m.emailAddressesTotal.Set(count)
}

func (m *MetricsMiddleware) RecordRateLimitHit(limitType, endpoint string) {
	m.rateLimitHitsTotal.WithLabelValues(limitType, endpoint).Inc()
}

func (m *MetricsMiddleware) RecordSecurityViolation(violationType, severity string) {
	m.securityViolationsTotal.WithLabelValues(violationType, severity).Inc()
}

// Database metric helper methods
func (m *MetricsMiddleware) SetDBConnectionsActive(count float64) {
	m.dbConnectionsActive.Set(count)
}

func (m *MetricsMiddleware) RecordDBQuery(operation string, duration time.Duration, status string) {
	m.dbQueryDuration.WithLabelValues(operation).Observe(duration.Seconds())
	m.dbQueryTotal.WithLabelValues(operation, status).Inc()
}

// PrometheusHandler returns the HTTP handler for Prometheus metrics endpoint
func (m *MetricsMiddleware) PrometheusHandler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// GetRegistry returns the Prometheus registry for external use
func (m *MetricsMiddleware) GetRegistry() *prometheus.Registry {
	return m.registry
}

// Helper functions for pattern matching (simplified implementations)
func containsPattern(text, pattern string) bool {
	// Simplified implementation - in production, use regexp
	// This is a placeholder for UUID and numeric pattern detection
	return false
}

func replacePattern(text, pattern, replacement string) string {
	// Simplified implementation - in production, use regexp
	// This is a placeholder for pattern replacement
	return text
}

// Custom metrics collection function for periodic updates
func (m *MetricsMiddleware) CollectBusinessMetrics(
	domainsCount, emailAddressesCount float64,
	dbConnectionsActive float64,
) {
	m.SetDomainsTotal(domainsCount)
	m.SetEmailAddressesTotal(emailAddressesCount)
	m.SetDBConnectionsActive(dbConnectionsActive)
}

// MetricsInfo returns information about configured metrics
func (m *MetricsMiddleware) GetMetricsInfo() map[string]interface{} {
	return map[string]interface{}{
		"namespace":                     m.config.Namespace,
		"subsystem":                     m.config.Subsystem,
		"endpoint_metrics_enabled":      m.config.EnableEndpointMetrics,
		"status_code_metrics_enabled":   m.config.EnableStatusCodeMetrics,
		"request_size_metrics_enabled":  m.config.EnableRequestSizeMetrics,
		"response_size_metrics_enabled": m.config.EnableResponseSizeMetrics,
		"duration_buckets":              m.config.DurationBuckets,
		"size_buckets":                  m.config.SizeBuckets,
	}
}
