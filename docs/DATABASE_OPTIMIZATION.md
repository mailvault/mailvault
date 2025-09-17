# Database Optimization Guide

This guide covers the database connection pooling optimizations implemented in MailVault for production deployments.

## Overview

MailVault now includes comprehensive database connection pool optimization with the following features:

- **Optimized Connection Pooling**: CPU-aware connection pool sizing
- **Performance Metrics**: Prometheus metrics for monitoring
- **Query Instrumentation**: Detailed query performance tracking
- **Health Monitoring**: Automatic health checks and alerting
- **Production Configuration**: Environment-based tuning

## Configuration

### Environment Variables

```bash
# Database connection details
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=postgres
DATABASE_PASSWORD=postgres
DATABASE_NAME=mailvault
DATABASE_SSLMODE=disable

# Connection pooling optimization
DATABASE_POOL_MIN_SIZE=5                    # Minimum connections to maintain
DATABASE_POOL_MAX_SIZE=25                   # Maximum connections allowed
DATABASE_MAX_CONN_LIFETIME=3600             # Connection lifetime in seconds
DATABASE_MAX_CONN_IDLE_TIME=900             # Idle connection timeout in seconds
DATABASE_HEALTH_CHECK_PERIOD=60             # Health check interval in seconds
DATABASE_CONNECT_TIMEOUT=30                 # Connection timeout in seconds
DATABASE_STATEMENT_CACHE_CAPACITY=512       # Prepared statement cache size

# Monitoring
ENABLE_DATABASE_METRICS=true                # Enable Prometheus metrics
ENABLE_QUERY_INSTRUMENTATION=true           # Enable detailed query tracking
```

### Pool Sizing Guidelines

The optimal pool size depends on your deployment characteristics:

#### CPU-Based Sizing (Default)
- **Min Pool Size**: 20% of max pool size, minimum 2
- **Max Pool Size**: 4 × CPU cores, capped between 10-50
- **Recommendation**: Start with defaults and monitor utilization

#### Custom Sizing by Load Profile

**Low Traffic (< 100 req/min)**
```bash
DATABASE_POOL_MIN_SIZE=2
DATABASE_POOL_MAX_SIZE=10
```

**Medium Traffic (100-1000 req/min)**
```bash
DATABASE_POOL_MIN_SIZE=5
DATABASE_POOL_MAX_SIZE=25
```

**High Traffic (> 1000 req/min)**
```bash
DATABASE_POOL_MIN_SIZE=10
DATABASE_POOL_MAX_SIZE=50
```

**SMTP-Heavy Workload**
```bash
DATABASE_POOL_MIN_SIZE=8
DATABASE_POOL_MAX_SIZE=40
DATABASE_MAX_CONN_LIFETIME=1800  # Shorter lifetime for high turnover
```

## Monitoring

### Prometheus Metrics

The following metrics are exposed at `/metrics`:

#### Connection Pool Metrics
- `database_total_connections` - Total connections in pool
- `database_active_connections` - Currently active connections
- `database_idle_connections` - Currently idle connections
- `database_waiting_connections` - Connections waiting for availability

#### Connection Lifecycle
- `database_connections_created_total` - Total connections created
- `database_connections_destroyed_total` - Total connections destroyed
- `database_connections_failed_total` - Failed connection attempts

#### Query Performance
- `database_query_duration_seconds` - Query execution time histogram
- `database_queries_total` - Total queries executed by operation/table/status
- `database_transaction_duration_seconds` - Transaction duration histogram

#### Health Monitoring
- `database_health_checks_total` - Total health checks performed
- `database_health_check_failures_total` - Failed health checks

### Monitoring Script

Use the included monitoring script for real-time analysis:

```bash
# Show current status
./scripts/db-monitor.sh

# Continuous monitoring
./scripts/db-monitor.sh watch

# Export metrics to JSON
./scripts/db-monitor.sh export metrics.json

# View query performance
./scripts/db-monitor.sh queries
```

### Alerting Rules

Recommended Prometheus alerting rules:

```yaml
groups:
- name: mailvault-database
  rules:
  - alert: DatabasePoolHighUtilization
    expr: (database_active_connections / database_total_connections) > 0.8
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "Database pool utilization is high"
      description: "Pool utilization is {{ $value | humanizePercentage }}"

  - alert: DatabaseConnectionFailures
    expr: increase(database_connections_failed_total[5m]) > 5
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "Database connection failures detected"

  - alert: DatabaseHealthCheckFailures
    expr: increase(database_health_check_failures_total[5m]) > 0
    for: 1m
    labels:
      severity: warning
    annotations:
      summary: "Database health check failures"

  - alert: SlowDatabaseQueries
    expr: histogram_quantile(0.95, database_query_duration_seconds_bucket) > 1.0
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "95th percentile query time is high"
```

## Performance Tuning

### Connection Pool Tuning

Monitor these key indicators:

1. **Pool Utilization**: Keep between 60-80% for optimal performance
2. **Wait Time**: Should be minimal; increase max pool size if high
3. **Idle Connections**: Should be > 0; decrease min pool size if always high
4. **Connection Churn**: High create/destroy rates indicate pool tuning needed

### Query Optimization

Use query instrumentation to identify:

1. **Slow Queries**: Queries taking > 100ms consistently
2. **High-Frequency Queries**: Candidates for caching
3. **Failed Queries**: May indicate connection issues
4. **Transaction Duration**: Long transactions can block connections

### Common Optimization Scenarios

#### High Connection Wait Times
```bash
# Increase maximum pool size
DATABASE_POOL_MAX_SIZE=40

# Reduce connection lifetime to cycle faster
DATABASE_MAX_CONN_LIFETIME=1800
```

#### Excessive Idle Connections
```bash
# Reduce minimum pool size
DATABASE_POOL_MIN_SIZE=3

# Reduce idle timeout
DATABASE_MAX_CONN_IDLE_TIME=600
```

#### Connection Creation Overhead
```bash
# Increase minimum pool size to keep connections warm
DATABASE_POOL_MIN_SIZE=8

# Increase connection lifetime
DATABASE_MAX_CONN_LIFETIME=7200
```

## Production Deployment

### Docker Configuration

```dockerfile
ENV DATABASE_POOL_MIN_SIZE=8
ENV DATABASE_POOL_MAX_SIZE=32
ENV DATABASE_MAX_CONN_LIFETIME=3600
ENV DATABASE_MAX_CONN_IDLE_TIME=900
ENV ENABLE_DATABASE_METRICS=true
```

### Kubernetes Configuration

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mailvault-api
spec:
  template:
    spec:
      containers:
      - name: api
        env:
        - name: DATABASE_POOL_MIN_SIZE
          value: "8"
        - name: DATABASE_POOL_MAX_SIZE
          value: "32"
        - name: ENABLE_DATABASE_METRICS
          value: "true"
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
```

### PostgreSQL Configuration

Optimize PostgreSQL settings for connection pooling:

```sql
-- postgresql.conf
max_connections = 200                    # Should be > sum of all pool max sizes
shared_buffers = 256MB                   # 25% of available RAM
effective_cache_size = 1GB               # 75% of available RAM
work_mem = 4MB                          # Per connection working memory
maintenance_work_mem = 64MB              # For maintenance operations
wal_buffers = 16MB                      # Write-ahead log buffers
checkpoint_completion_target = 0.9       # Checkpoint spread
random_page_cost = 1.1                  # SSD optimization
effective_io_concurrency = 200          # SSD concurrency
```

## Troubleshooting

### Connection Pool Exhaustion

**Symptoms**: High wait times, connection failures, timeouts

**Solutions**:
1. Increase `DATABASE_POOL_MAX_SIZE`
2. Optimize slow queries to reduce connection hold time
3. Implement connection pooling at application level (PgBouncer)
4. Scale horizontally with read replicas

### Memory Usage

**Symptoms**: High memory consumption, OOM kills

**Solutions**:
1. Reduce `DATABASE_POOL_MAX_SIZE`
2. Tune `work_mem` in PostgreSQL
3. Monitor query complexity and data sizes
4. Implement query result streaming for large datasets

### Connection Leaks

**Symptoms**: Steadily increasing active connections, pool exhaustion

**Solutions**:
1. Review connection handling in repository layer
2. Ensure proper transaction rollback on errors
3. Monitor connection lifecycle metrics
4. Implement connection timeout safeguards

### Performance Degradation

**Symptoms**: Increasing query times, high CPU usage

**Solutions**:
1. Analyze slow query logs
2. Review database indexes
3. Monitor connection pool utilization
4. Consider read/write splitting

## Integration Examples

### Repository Pattern with Metrics

```go
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*entities.User, error) {
    start := time.Now()
    defer func() {
        duration := time.Since(start)
        r.pool.RecordQuery("SELECT", "users", duration, nil)
    }()

    // Query implementation
    return user, nil
}
```

### Caching Integration

```go
func (r *UserRepository) GetByIDCached(ctx context.Context, id uuid.UUID) (*entities.User, error) {
    cacheKey := fmt.Sprintf("user:id:%s", id.String())

    // Try cache first
    var user entities.User
    if err := r.cache.Get(ctx, cacheKey, &user); err == nil {
        return &user, nil
    }

    // Database fallback with metrics
    return r.GetByID(ctx, id)
}
```

## References

- [PostgreSQL Connection Pooling Best Practices](https://www.postgresql.org/docs/current/runtime-config-connection.html)
- [pgx Connection Pool Documentation](https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool)
- [Prometheus Monitoring Guidelines](https://prometheus.io/docs/practices/naming/)
- [Database Monitoring with Grafana](https://grafana.com/docs/grafana/latest/getting-started/)

## Support

For additional support with database optimization:

1. Review application logs for connection-related errors
2. Monitor Prometheus metrics dashboard
3. Use the `db-monitor.sh` script for real-time analysis
4. Consider professional database tuning for high-scale deployments