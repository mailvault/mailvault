package webhook

import (
	"sync"
	"time"
)

// MetricsCollector collects webhook delivery metrics
type MetricsCollector struct {
	mu      sync.RWMutex
	metrics map[string]*WebhookMetrics
}

// WebhookMetrics represents metrics for a specific webhook type
type WebhookMetrics struct {
	TotalAttempts   int64         `json:"total_attempts"`
	Successful      int64         `json:"successful"`
	Failed          int64         `json:"failed"`
	TotalRetries    int64         `json:"total_retries"`
	AverageLatency  time.Duration `json:"average_latency"`
	TotalLatency    time.Duration `json:"total_latency"`
	LatencyCount    int64         `json:"latency_count"`
	FailureReasons  map[string]int64 `json:"failure_reasons"`
	DomainMetrics   map[string]*DomainMetrics `json:"domain_metrics"`
	LastUpdated     time.Time     `json:"last_updated"`
}

// DomainMetrics represents metrics for a specific domain
type DomainMetrics struct {
	TotalAttempts  int64         `json:"total_attempts"`
	Successful     int64         `json:"successful"`
	Failed         int64         `json:"failed"`
	AverageLatency time.Duration `json:"average_latency"`
	TotalLatency   time.Duration `json:"total_latency"`
	LatencyCount   int64         `json:"latency_count"`
	LastSuccess    time.Time     `json:"last_success"`
	LastFailure    time.Time     `json:"last_failure"`
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics: make(map[string]*WebhookMetrics),
	}
}

// RecordWebhookAttempt records a webhook attempt
func (mc *MetricsCollector) RecordWebhookAttempt(webhookType, domain string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	metrics := mc.getOrCreateMetrics(webhookType)
	metrics.TotalAttempts++
	metrics.LastUpdated = time.Now()

	domainMetrics := mc.getOrCreateDomainMetrics(metrics, domain)
	domainMetrics.TotalAttempts++
}

// RecordWebhookSuccess records a successful webhook delivery
func (mc *MetricsCollector) RecordWebhookSuccess(webhookType, domain string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	metrics := mc.getOrCreateMetrics(webhookType)
	metrics.Successful++
	metrics.LastUpdated = time.Now()

	domainMetrics := mc.getOrCreateDomainMetrics(metrics, domain)
	domainMetrics.Successful++
	domainMetrics.LastSuccess = time.Now()
}

// RecordWebhookFailure records a failed webhook delivery
func (mc *MetricsCollector) RecordWebhookFailure(webhookType, domain, reason string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	metrics := mc.getOrCreateMetrics(webhookType)
	metrics.Failed++
	metrics.LastUpdated = time.Now()

	// Track failure reasons
	if metrics.FailureReasons == nil {
		metrics.FailureReasons = make(map[string]int64)
	}
	metrics.FailureReasons[reason]++

	domainMetrics := mc.getOrCreateDomainMetrics(metrics, domain)
	domainMetrics.Failed++
	domainMetrics.LastFailure = time.Now()
}

// RecordWebhookDuration records webhook request duration
func (mc *MetricsCollector) RecordWebhookDuration(webhookType, domain string, duration time.Duration, success bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	metrics := mc.getOrCreateMetrics(webhookType)
	metrics.TotalLatency += duration
	metrics.LatencyCount++
	metrics.AverageLatency = metrics.TotalLatency / time.Duration(metrics.LatencyCount)
	metrics.LastUpdated = time.Now()

	domainMetrics := mc.getOrCreateDomainMetrics(metrics, domain)
	domainMetrics.TotalLatency += duration
	domainMetrics.LatencyCount++
	domainMetrics.AverageLatency = domainMetrics.TotalLatency / time.Duration(domainMetrics.LatencyCount)
}

// RecordWebhookRetries records the number of retries for a webhook
func (mc *MetricsCollector) RecordWebhookRetries(webhookType, domain string, retries int) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	metrics := mc.getOrCreateMetrics(webhookType)
	metrics.TotalRetries += int64(retries - 1) // Subtract 1 because first attempt is not a retry
	metrics.LastUpdated = time.Now()
}

// GetWebhookMetrics returns metrics for a specific webhook type
func (mc *MetricsCollector) GetWebhookMetrics(webhookType string) *WebhookMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	metrics := mc.metrics[webhookType]
	if metrics == nil {
		return &WebhookMetrics{
			FailureReasons: make(map[string]int64),
			DomainMetrics:  make(map[string]*DomainMetrics),
		}
	}

	// Return a copy to avoid concurrent access issues
	return mc.copyMetrics(metrics)
}

// GetDomainMetrics returns metrics for a specific domain
func (mc *MetricsCollector) GetDomainMetrics(webhookType, domain string) *DomainMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	metrics := mc.metrics[webhookType]
	if metrics == nil || metrics.DomainMetrics == nil {
		return &DomainMetrics{}
	}

	domainMetrics := metrics.DomainMetrics[domain]
	if domainMetrics == nil {
		return &DomainMetrics{}
	}

	// Return a copy
	return mc.copyDomainMetrics(domainMetrics)
}

// GetAllMetrics returns all webhook metrics
func (mc *MetricsCollector) GetAllMetrics() map[string]*WebhookMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	result := make(map[string]*WebhookMetrics)
	for webhookType, metrics := range mc.metrics {
		result[webhookType] = mc.copyMetrics(metrics)
	}

	return result
}

// ResetMetrics resets all metrics
func (mc *MetricsCollector) ResetMetrics() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.metrics = make(map[string]*WebhookMetrics)
}

// ResetWebhookMetrics resets metrics for a specific webhook type
func (mc *MetricsCollector) ResetWebhookMetrics(webhookType string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	delete(mc.metrics, webhookType)
}

// getOrCreateMetrics gets or creates metrics for a webhook type
func (mc *MetricsCollector) getOrCreateMetrics(webhookType string) *WebhookMetrics {
	metrics := mc.metrics[webhookType]
	if metrics == nil {
		metrics = &WebhookMetrics{
			FailureReasons: make(map[string]int64),
			DomainMetrics:  make(map[string]*DomainMetrics),
		}
		mc.metrics[webhookType] = metrics
	}
	return metrics
}

// getOrCreateDomainMetrics gets or creates domain metrics
func (mc *MetricsCollector) getOrCreateDomainMetrics(metrics *WebhookMetrics, domain string) *DomainMetrics {
	if metrics.DomainMetrics == nil {
		metrics.DomainMetrics = make(map[string]*DomainMetrics)
	}

	domainMetrics := metrics.DomainMetrics[domain]
	if domainMetrics == nil {
		domainMetrics = &DomainMetrics{}
		metrics.DomainMetrics[domain] = domainMetrics
	}
	return domainMetrics
}

// copyMetrics creates a deep copy of webhook metrics
func (mc *MetricsCollector) copyMetrics(metrics *WebhookMetrics) *WebhookMetrics {
	copy := &WebhookMetrics{
		TotalAttempts:  metrics.TotalAttempts,
		Successful:     metrics.Successful,
		Failed:         metrics.Failed,
		TotalRetries:   metrics.TotalRetries,
		AverageLatency: metrics.AverageLatency,
		TotalLatency:   metrics.TotalLatency,
		LatencyCount:   metrics.LatencyCount,
		LastUpdated:    metrics.LastUpdated,
		FailureReasons: make(map[string]int64),
		DomainMetrics:  make(map[string]*DomainMetrics),
	}

	// Copy failure reasons
	for reason, count := range metrics.FailureReasons {
		copy.FailureReasons[reason] = count
	}

	// Copy domain metrics
	for domain, domainMetrics := range metrics.DomainMetrics {
		copy.DomainMetrics[domain] = mc.copyDomainMetrics(domainMetrics)
	}

	return copy
}

// copyDomainMetrics creates a deep copy of domain metrics
func (mc *MetricsCollector) copyDomainMetrics(metrics *DomainMetrics) *DomainMetrics {
	return &DomainMetrics{
		TotalAttempts:  metrics.TotalAttempts,
		Successful:     metrics.Successful,
		Failed:         metrics.Failed,
		AverageLatency: metrics.AverageLatency,
		TotalLatency:   metrics.TotalLatency,
		LatencyCount:   metrics.LatencyCount,
		LastSuccess:    metrics.LastSuccess,
		LastFailure:    metrics.LastFailure,
	}
}

// GetSuccessRate returns the success rate for a webhook type
func (mc *MetricsCollector) GetSuccessRate(webhookType string) float64 {
	metrics := mc.GetWebhookMetrics(webhookType)

	total := metrics.Successful + metrics.Failed
	if total == 0 {
		return 0.0
	}

	return float64(metrics.Successful) / float64(total)
}

// GetDomainSuccessRate returns the success rate for a specific domain
func (mc *MetricsCollector) GetDomainSuccessRate(webhookType, domain string) float64 {
	domainMetrics := mc.GetDomainMetrics(webhookType, domain)

	total := domainMetrics.Successful + domainMetrics.Failed
	if total == 0 {
		return 0.0
	}

	return float64(domainMetrics.Successful) / float64(total)
}

// IsHealthy returns true if webhook delivery is healthy (success rate > threshold)
func (mc *MetricsCollector) IsHealthy(webhookType string, threshold float64) bool {
	return mc.GetSuccessRate(webhookType) >= threshold
}

// IsDomainHealthy returns true if webhook delivery for a domain is healthy
func (mc *MetricsCollector) IsDomainHealthy(webhookType, domain string, threshold float64) bool {
	return mc.GetDomainSuccessRate(webhookType, domain) >= threshold
}