# MailVault Monitoring with Prometheus

This guide covers the complete monitoring setup for MailVault using Prometheus, Grafana, and AlertManager.

## Overview

The monitoring infrastructure provides:

- **Prometheus**: Metrics collection and storage
- **Grafana**: Visualization dashboards
- **AlertManager**: Alert routing and notifications
- **Node Exporter**: System metrics
- **PostgreSQL Exporter**: Database metrics
- **Redis Exporter**: Cache metrics

## Quick Start

### 1. Setup Monitoring Infrastructure

```bash
# Start the monitoring stack
./monitoring/setup.sh

# Or manually with Docker Compose
docker-compose -f docker-compose.prometheus.yml up -d
```

### 2. Start MailVault Services

```bash
# Start API service (metrics on :8080/metrics)
./build/service

# Start SMTP daemon (metrics on :8081/metrics)
./build/smtpd
```

### 3. Access Dashboards

- **Grafana**: http://localhost:3000 (admin/admin123)
- **Prometheus**: http://localhost:9090
- **AlertManager**: http://localhost:9093

## Service Configuration

### API Service Metrics

The API service exposes metrics on `/metrics` endpoint (port 8080 by default):

```bash
# Check API metrics
curl http://localhost:8080/metrics
```

**Key Metrics:**
- `http_requests_total` - Total HTTP requests
- `http_request_duration_seconds` - Request duration
- `rate_limit_violations_total` - Rate limiting violations
- `security_violations_total` - Security violations
- `auth_attempts_total` - Authentication attempts
- `database_*` - Database connection pool metrics

### SMTP Service Metrics

The SMTP service exposes metrics on a separate port (8081 by default):

```bash
# Check SMTP metrics
curl http://localhost:8081/metrics
```

**Key Metrics:**
- `smtp_connections_total` - SMTP connections
- `smtp_emails_received_total` - Emails received
- `smtp_emails_processed_total` - Emails processed
- `smtp_verification_checks_total` - Verification checks (SPF, DKIM, DMARC)
- `smtp_processing_duration_seconds` - Email processing time

### Database Metrics

Database metrics are automatically collected:

**Connection Pool:**
- `database_total_connections` - Total connections
- `database_active_connections` - Active connections
- `database_idle_connections` - Idle connections
- `database_waiting_connections` - Waiting connections

**Performance:**
- `database_query_duration_seconds` - Query execution time
- `database_queries_total` - Total queries by operation/table
- `database_transaction_duration_seconds` - Transaction duration

**Health:**
- `database_health_checks_total` - Health checks performed
- `database_health_check_failures_total` - Failed health checks

## Pre-configured Dashboards

### 1. MailVault Overview
- API request rate and response time
- Database connection pool status
- SMTP connection and email processing rates
- System overview

### 2. MailVault Database Metrics
- Connection pool utilization gauge
- Query performance percentiles
- Connection health status
- Detailed pool metrics
- Query rates by operation
- Transaction duration

## Alert Rules

### Critical Alerts
- **Service Down**: API or SMTP service unavailable
- **High Error Rate**: >10% error rate for 5 minutes
- **Database Connection Failures**: Connection failures detected
- **Security Violations**: Security incidents detected
- **Disk Space Low**: >90% disk usage

### Warning Alerts
- **High Response Time**: 95th percentile >1 second
- **High Pool Utilization**: Database pool >80% utilized
- **Rate Limit Violations**: High rate limiting activity
- **Authentication Failures**: Suspicious login activity
- **Slow Queries**: Database queries >1 second

### System Alerts
- **High CPU Usage**: >80% CPU for 5 minutes
- **High Memory Usage**: >85% memory usage
- **Network Issues**: Connection timeouts

## AlertManager Configuration

### Email Notifications

Edit `monitoring/alertmanager/alertmanager.yml`:

```yaml
global:
  smtp_smarthost: 'smtp.gmail.com:587'
  smtp_from: 'alerts@yourdomain.com'
  smtp_auth_username: 'alerts@yourdomain.com'
  smtp_auth_password: 'your-app-password'
  smtp_require_tls: true
```

### Slack Notifications

Add Slack webhook configuration:

```yaml
slack_configs:
  - api_url: 'https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK'
    channel: '#alerts'
    title: 'MailVault Alert: {{ .GroupLabels.alertname }}'
```

### Alert Routing

Configure different receivers for different severity levels:

```yaml
routes:
  - match:
      severity: critical
    receiver: 'critical-alerts'
    group_wait: 5s
    repeat_interval: 5m
  - match:
      severity: warning
    receiver: 'warning-alerts'
    repeat_interval: 30m
```

## Grafana Configuration

### Datasource

Prometheus is automatically configured as the default datasource.

### Custom Dashboards

Create custom dashboards for specific metrics:

1. Navigate to Grafana (http://localhost:3000)
2. Login with admin/admin123
3. Create new dashboard
4. Add panels with PromQL queries

### Example Queries

**API Request Rate:**
```promql
rate(http_requests_total{job="mailvault-api"}[5m])
```

**Database Pool Utilization:**
```promql
(database_active_connections / database_total_connections) * 100
```

**95th Percentile Response Time:**
```promql
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))
```

**Error Rate:**
```promql
rate(http_requests_total{status_code=~"5.."}[5m]) / rate(http_requests_total[5m])
```

## Monitoring Best Practices

### 1. Metrics Collection

- Monitor both services and infrastructure
- Use appropriate scrape intervals (15s for services, 60s for system)
- Set up proper retention policies
- Monitor metric cardinality

### 2. Alerting Strategy

- Set up different severity levels
- Configure appropriate thresholds
- Avoid alert fatigue with proper grouping
- Test alert delivery regularly

### 3. Dashboard Design

- Create role-specific dashboards
- Use consistent time ranges
- Include SLA/SLO indicators
- Add annotations for deployments

### 4. Performance Optimization

- Monitor query performance
- Track resource utilization
- Set up capacity planning alerts
- Regular review of slow queries

## Troubleshooting

### Services Not Appearing in Prometheus

1. Check service endpoints:
   ```bash
   curl http://localhost:8080/metrics  # API service
   curl http://localhost:8081/metrics  # SMTP service
   ```

2. Verify Prometheus configuration:
   ```bash
   docker-compose -f docker-compose.prometheus.yml logs prometheus
   ```

3. Check Prometheus targets:
   - Navigate to http://localhost:9090/targets
   - Ensure targets are "UP"

### Database Metrics Missing

1. Check PostgreSQL Exporter:
   ```bash
   docker-compose -f docker-compose.prometheus.yml logs postgres-exporter
   ```

2. Verify database connection string in docker-compose.yml

3. Check database accessibility from container:
   ```bash
   docker exec mailvault-postgres-exporter pg_isready -h host.docker.internal
   ```

### Grafana Dashboard Issues

1. Verify Prometheus datasource connection
2. Check query syntax in panel editor
3. Ensure metrics exist in Prometheus
4. Review time range settings

### Alert Not Firing

1. Check alert rules in Prometheus:
   - Navigate to http://localhost:9090/alerts

2. Verify AlertManager configuration:
   ```bash
   docker-compose -f docker-compose.prometheus.yml logs alertmanager
   ```

3. Test alert delivery manually

## Advanced Configuration

### Custom Metrics

Add custom metrics to your application:

```go
// Define custom metrics
var (
    emailsSent = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mailvault_emails_sent_total",
            Help: "Total emails sent",
        },
        []string{"domain", "status"},
    )
)

// Register metrics
prometheus.MustRegister(emailsSent)

// Increment counter
emailsSent.WithLabelValues("example.com", "success").Inc()
```

### External Integrations

#### PagerDuty Integration

```yaml
# In alertmanager.yml
receivers:
  - name: 'pagerduty'
    pagerduty_configs:
      - service_key: 'YOUR_PAGERDUTY_SERVICE_KEY'
        severity: '{{ .GroupLabels.severity }}'
        description: '{{ .GroupLabels.alertname }}: {{ .GroupLabels.instance }}'
```

#### Webhook Integration

```yaml
# In alertmanager.yml
receivers:
  - name: 'webhook'
    webhook_configs:
      - url: 'http://your-webhook-endpoint/alerts'
        send_resolved: true
```

## Security Considerations

### 1. Access Control

- Change default Grafana admin password
- Set up proper user roles and permissions
- Use reverse proxy with authentication
- Restrict access to monitoring ports

### 2. Network Security

- Use private networks for monitoring components
- Enable TLS for external connections
- Configure firewall rules
- Use VPN for remote access

### 3. Data Privacy

- Avoid logging sensitive data in metrics
- Use metric labels carefully
- Implement data retention policies
- Secure alert notification channels

## Scaling Monitoring

### High Availability

- Run multiple Prometheus instances
- Use Prometheus federation
- Set up Grafana load balancing
- Configure AlertManager clustering

### Performance Optimization

- Optimize PromQL queries
- Use recording rules for complex calculations
- Implement metric filtering
- Configure appropriate retention policies

## Backup and Recovery

### Prometheus Data

```bash
# Backup Prometheus data
docker run --rm -v prometheus_data:/data -v $(pwd):/backup alpine tar czf /backup/prometheus-backup.tar.gz /data

# Restore Prometheus data
docker run --rm -v prometheus_data:/data -v $(pwd):/backup alpine tar xzf /backup/prometheus-backup.tar.gz -C /
```

### Grafana Configuration

```bash
# Backup Grafana
docker run --rm -v grafana_data:/data -v $(pwd):/backup alpine tar czf /backup/grafana-backup.tar.gz /data

# Restore Grafana
docker run --rm -v grafana_data:/data -v $(pwd):/backup alpine tar xzf /backup/grafana-backup.tar.gz -C /
```

## Support

For monitoring issues:

1. Check service logs:
   ```bash
   ./monitoring/setup.sh logs [service]
   ```

2. Verify service status:
   ```bash
   ./monitoring/setup.sh status
   ```

3. Review monitoring documentation
4. Check Prometheus and Grafana communities for help

## References

- [Prometheus Documentation](https://prometheus.io/docs/)
- [Grafana Documentation](https://grafana.com/docs/)
- [AlertManager Documentation](https://prometheus.io/docs/alerting/latest/alertmanager/)
- [PromQL Guide](https://prometheus.io/docs/prometheus/latest/querying/basics/)