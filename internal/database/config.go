package database

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"github.com/ardanlabs/conf/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Config represents the database configuration with optimized pooling settings
type Config struct {
	// Connection details
	Host     string `conf:"env:DATABASE_HOST,default:localhost"`
	Port     string `conf:"env:DATABASE_PORT,default:5432"`
	User     string `conf:"env:DATABASE_USER,required"`
	Password string `conf:"env:DATABASE_PASSWORD,required,mask"`
	Name     string `conf:"env:DATABASE_NAME,required"`
	SSLMode  string `conf:"env:DATABASE_SSLMODE,default:disable"`

	// Connection pooling settings
	PoolMinSize       int32         `conf:"env:DATABASE_POOL_MIN_SIZE,default:5"`
	PoolMaxSize       int32         `conf:"env:DATABASE_POOL_MAX_SIZE,default:25"`
	MaxConnLifetime   time.Duration `conf:"env:DATABASE_MAX_CONN_LIFETIME,default:1h"`
	MaxConnIdleTime   time.Duration `conf:"env:DATABASE_MAX_CONN_IDLE_TIME,default:15m"`
	HealthCheckPeriod time.Duration `conf:"env:DATABASE_HEALTH_CHECK_PERIOD,default:1m"`
	ConnectTimeout    time.Duration `conf:"env:DATABASE_CONNECT_TIMEOUT,default:30s"`

	// Performance settings
	StatementCacheCapacity int32 `conf:"env:DATABASE_STATEMENT_CACHE_CAPACITY,default:512"`

	// Monitoring settings
	EnableMetrics bool `conf:"env:DATABASE_ENABLE_METRICS,default:true"`
}

// DefaultConfig returns a production-optimized database configuration
func DefaultConfig() Config {
	// Calculate optimal pool size based on CPU cores
	maxCores := runtime.NumCPU()
	maxPoolSize := int32(maxCores * 4) // #nosec G115 -- NumCPU*4 fits int32 on any realistic machine
	if maxPoolSize < 10 {
		maxPoolSize = 10 // Minimum reasonable pool size
	}
	if maxPoolSize > 50 {
		maxPoolSize = 50 // Maximum to prevent resource exhaustion
	}

	minPoolSize := maxPoolSize / 5 // 20% of max pool size
	if minPoolSize < 2 {
		minPoolSize = 2 // Minimum to ensure availability
	}

	return Config{
		Host:                   "localhost",
		Port:                   "5432",
		SSLMode:                "disable",
		PoolMinSize:            minPoolSize,
		PoolMaxSize:            maxPoolSize,
		MaxConnLifetime:        time.Hour,
		MaxConnIdleTime:        15 * time.Minute,
		HealthCheckPeriod:      time.Minute,
		ConnectTimeout:         30 * time.Second,
		StatementCacheCapacity: 512,
		EnableMetrics:          true,
	}
}

// ConnectionString returns the optimized PostgreSQL connection string
func (c *Config) ConnectionString() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s&pool_min_conns=%d&pool_max_conns=%d&pool_max_conn_lifetime=%s&pool_max_conn_idle_time=%s&pool_health_check_period=%s&connect_timeout=%g&default_query_exec_mode=cache_statement",
		c.User, c.Password, c.Host, c.Port, c.Name,
		c.SSLMode, c.PoolMinSize, c.PoolMaxSize,
		c.MaxConnLifetime, c.MaxConnIdleTime, c.HealthCheckPeriod, c.ConnectTimeout.Seconds(),
	)
}

// DatabasePool represents an enhanced PostgreSQL connection pool with monitoring
type DatabasePool struct {
	*pgxpool.Pool
	config  Config
	metrics *DatabaseMetrics
	logger  *slog.Logger
}

// DatabaseMetrics tracks database connection pool performance
type DatabaseMetrics struct {
	// Pool metrics
	activeConnections  prometheus.Gauge
	idleConnections    prometheus.Gauge
	waitingConnections prometheus.Gauge
	totalConnections   prometheus.Gauge

	// Connection lifecycle metrics
	connectionsCreated   prometheus.Counter
	connectionsDestroyed prometheus.Counter
	connectionsFailed    prometheus.Counter

	// Query performance metrics
	queryDuration       prometheus.HistogramVec
	queryTotal          prometheus.CounterVec
	transactionDuration prometheus.Histogram

	// Health metrics
	healthCheckTotal    prometheus.Counter
	healthCheckFailures prometheus.Counter
}

// NewDatabaseMetrics creates database performance metrics
func NewDatabaseMetrics() *DatabaseMetrics {
	return &DatabaseMetrics{
		activeConnections: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "database_active_connections",
			Help: "Number of active database connections",
		}),
		idleConnections: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "database_idle_connections",
			Help: "Number of idle database connections",
		}),
		waitingConnections: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "database_waiting_connections",
			Help: "Number of connections waiting for availability",
		}),
		totalConnections: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "database_total_connections",
			Help: "Total number of database connections",
		}),
		connectionsCreated: promauto.NewCounter(prometheus.CounterOpts{
			Name: "database_connections_created_total",
			Help: "Total number of database connections created",
		}),
		connectionsDestroyed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "database_connections_destroyed_total",
			Help: "Total number of database connections destroyed",
		}),
		connectionsFailed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "database_connections_failed_total",
			Help: "Total number of failed database connection attempts",
		}),
		queryDuration: *promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "database_query_duration_seconds",
			Help:    "Database query execution duration",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 15), // 1ms to ~32s
		}, []string{"operation", "table"}),
		queryTotal: *promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "database_queries_total",
			Help: "Total number of database queries executed",
		}, []string{"operation", "table", "status"}),
		transactionDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "database_transaction_duration_seconds",
			Help:    "Database transaction duration",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 15),
		}),
		healthCheckTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "database_health_checks_total",
			Help: "Total number of database health checks performed",
		}),
		healthCheckFailures: promauto.NewCounter(prometheus.CounterOpts{
			Name: "database_health_check_failures_total",
			Help: "Total number of failed database health checks",
		}),
	}
}

// NewOptimizedPool creates a new optimized PostgreSQL connection pool
func NewOptimizedPool(ctx context.Context, prefix string, logger *slog.Logger) (*DatabasePool, error) {
	var cfg Config

	// Parse configuration with defaults
	if _, err := conf.Parse(prefix, &cfg); err != nil {
		return nil, fmt.Errorf("parsing database config: %w", err)
	}

	// Apply defaults if not set
	if cfg.PoolMinSize == 0 || cfg.PoolMaxSize == 0 {
		defaults := DefaultConfig()
		if cfg.PoolMinSize == 0 {
			cfg.PoolMinSize = defaults.PoolMinSize
		}
		if cfg.PoolMaxSize == 0 {
			cfg.PoolMaxSize = defaults.PoolMaxSize
		}
	}

	// Parse pool configuration for pgxpool
	poolConfig, err := pgxpool.ParseConfig(cfg.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("parsing pool config: %w", err)
	}

	// Apply additional optimizations
	poolConfig.MinConns = cfg.PoolMinSize
	poolConfig.MaxConns = cfg.PoolMaxSize
	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolConfig.HealthCheckPeriod = cfg.HealthCheckPeriod

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	// Initialize metrics
	var metrics *DatabaseMetrics
	if cfg.EnableMetrics {
		metrics = NewDatabaseMetrics()
	}

	dbPool := &DatabasePool{
		Pool:    pool,
		config:  cfg,
		metrics: metrics,
		logger:  logger,
	}

	// Start metrics collection if enabled
	if cfg.EnableMetrics {
		go dbPool.startMetricsCollection(ctx)
	}

	// Log configuration
	if logger != nil {
		logger.Info("Database pool initialized",
			slog.String("host", cfg.Host),
			slog.String("port", cfg.Port),
			slog.String("database", cfg.Name),
			slog.Int("min_connections", int(cfg.PoolMinSize)),
			slog.Int("max_connections", int(cfg.PoolMaxSize)),
			slog.Duration("max_conn_lifetime", cfg.MaxConnLifetime),
			slog.Duration("max_conn_idle_time", cfg.MaxConnIdleTime),
			slog.Duration("health_check_period", cfg.HealthCheckPeriod),
		)
	}

	return dbPool, nil
}

// startMetricsCollection collects pool statistics periodically
func (db *DatabasePool) startMetricsCollection(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second) // Collect metrics every 10 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			db.updatePoolMetrics()
		}
	}
}

// updatePoolMetrics updates Prometheus metrics with current pool statistics
func (db *DatabasePool) updatePoolMetrics() {
	if db.metrics == nil {
		return
	}

	stats := db.Pool.Stat()

	// Update pool connection metrics
	db.metrics.activeConnections.Set(float64(stats.AcquiredConns()))
	db.metrics.idleConnections.Set(float64(stats.IdleConns()))
	db.metrics.totalConnections.Set(float64(stats.TotalConns()))

	// Note: MaxConns() returns the maximum, not current waiting
	// For actual waiting connections, we'd need to track this separately
	maxConns := float64(stats.MaxConns())
	totalConns := float64(stats.TotalConns())

	// Estimate waiting connections (this is an approximation)
	estimatedWaiting := maxConns - totalConns
	if estimatedWaiting < 0 {
		estimatedWaiting = 0
	}
	db.metrics.waitingConnections.Set(estimatedWaiting)

	// Update lifecycle counters
	db.metrics.connectionsCreated.Add(float64(stats.NewConnsCount()))
	db.metrics.connectionsDestroyed.Add(float64(stats.MaxConns() - stats.TotalConns()))
}

// Ping performs a health check and updates metrics
func (db *DatabasePool) Ping(ctx context.Context) error {
	if db.metrics != nil {
		db.metrics.healthCheckTotal.Inc()
	}

	err := db.Pool.Ping(ctx)
	if err != nil && db.metrics != nil {
		db.metrics.healthCheckFailures.Inc()
	}

	return err
}

// GetMetrics returns the database metrics instance
func (db *DatabasePool) GetMetrics() *DatabaseMetrics {
	return db.metrics
}

// GetStats returns detailed connection pool statistics
func (db *DatabasePool) GetStats() map[string]interface{} {
	stats := db.Pool.Stat()

	return map[string]interface{}{
		"total_conns":                stats.TotalConns(),
		"acquired_conns":             stats.AcquiredConns(),
		"idle_conns":                 stats.IdleConns(),
		"max_conns":                  stats.MaxConns(),
		"new_conns_count":            stats.NewConnsCount(),
		"acquire_count":              stats.AcquireCount(),
		"acquire_duration":           stats.AcquireDuration(),
		"canceled_acquire_count":     stats.CanceledAcquireCount(),
		"constructing_conns":         stats.ConstructingConns(),
		"empty_acquire_count":        stats.EmptyAcquireCount(),
		"max_idle_destroy_count":     stats.MaxIdleDestroyCount(),
		"max_lifetime_destroy_count": stats.MaxLifetimeDestroyCount(),

		// Configuration
		"config": map[string]interface{}{
			"min_pool_size":       db.config.PoolMinSize,
			"max_pool_size":       db.config.PoolMaxSize,
			"max_conn_lifetime":   db.config.MaxConnLifetime.String(),
			"max_conn_idle_time":  db.config.MaxConnIdleTime.String(),
			"health_check_period": db.config.HealthCheckPeriod.String(),
		},
	}
}

// RecordQuery records query metrics for monitoring
func (db *DatabasePool) RecordQuery(operation, table string, duration time.Duration, err error) {
	if db.metrics == nil {
		return
	}

	status := "success"
	if err != nil {
		status = "error"
	}

	db.metrics.queryDuration.WithLabelValues(operation, table).Observe(duration.Seconds())
	db.metrics.queryTotal.WithLabelValues(operation, table, status).Inc()
}

// RecordTransaction records transaction metrics
func (db *DatabasePool) RecordTransaction(duration time.Duration) {
	if db.metrics != nil {
		db.metrics.transactionDuration.Observe(duration.Seconds())
	}
}

// Close closes the database pool and cleans up resources
func (db *DatabasePool) Close() {
	if db.logger != nil {
		db.logger.Info("Closing database connection pool")
	}

	db.Pool.Close()
}
