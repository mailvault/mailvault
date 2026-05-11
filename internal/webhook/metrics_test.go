package webhook

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMetricsCollector_BasicOperations(t *testing.T) {
	mc := NewMetricsCollector()

	// Test initial state
	metrics := mc.GetWebhookMetrics("incoming_email")
	assert.Equal(t, int64(0), metrics.TotalAttempts)
	assert.Equal(t, int64(0), metrics.Successful)
	assert.Equal(t, int64(0), metrics.Failed)
	assert.Equal(t, time.Duration(0), metrics.AverageLatency)
	assert.NotNil(t, metrics.FailureReasons)
	assert.NotNil(t, metrics.DomainMetrics)

	// Record some metrics
	mc.RecordWebhookAttempt("incoming_email", "example.com")
	mc.RecordWebhookSuccess("incoming_email", "example.com")
	mc.RecordWebhookDuration("incoming_email", "example.com", 100*time.Millisecond, true)

	// Check updated metrics
	metrics = mc.GetWebhookMetrics("incoming_email")
	assert.Equal(t, int64(1), metrics.TotalAttempts)
	assert.Equal(t, int64(1), metrics.Successful)
	assert.Equal(t, int64(0), metrics.Failed)
	assert.Equal(t, 100*time.Millisecond, metrics.AverageLatency)
	assert.True(t, metrics.LastUpdated.After(time.Time{}))
}

func TestMetricsCollector_Failures(t *testing.T) {
	mc := NewMetricsCollector()

	// Record failures with different reasons
	mc.RecordWebhookAttempt("incoming_email", "example.com")
	mc.RecordWebhookFailure("incoming_email", "example.com", "timeout")
	mc.RecordWebhookFailure("incoming_email", "example.com", "timeout")
	mc.RecordWebhookFailure("incoming_email", "example.com", "network_error")

	metrics := mc.GetWebhookMetrics("incoming_email")
	assert.Equal(t, int64(1), metrics.TotalAttempts)
	assert.Equal(t, int64(0), metrics.Successful)
	assert.Equal(t, int64(3), metrics.Failed)

	// Check failure reasons
	assert.Equal(t, int64(2), metrics.FailureReasons["timeout"])
	assert.Equal(t, int64(1), metrics.FailureReasons["network_error"])
}

func TestMetricsCollector_DurationCalculation(t *testing.T) {
	mc := NewMetricsCollector()

	// Record multiple durations
	mc.RecordWebhookDuration("incoming_email", "example.com", 100*time.Millisecond, true)
	mc.RecordWebhookDuration("incoming_email", "example.com", 200*time.Millisecond, true)
	mc.RecordWebhookDuration("incoming_email", "example.com", 300*time.Millisecond, false)

	metrics := mc.GetWebhookMetrics("incoming_email")
	assert.Equal(t, int64(3), metrics.LatencyCount)
	assert.Equal(t, 600*time.Millisecond, metrics.TotalLatency)
	assert.Equal(t, 200*time.Millisecond, metrics.AverageLatency) // (100+200+300)/3 = 200
}

func TestMetricsCollector_DomainMetrics(t *testing.T) {
	mc := NewMetricsCollector()

	// Record metrics for different domains
	mc.RecordWebhookAttempt("incoming_email", "example.com")
	mc.RecordWebhookSuccess("incoming_email", "example.com")
	mc.RecordWebhookDuration("incoming_email", "example.com", 150*time.Millisecond, true)

	mc.RecordWebhookAttempt("incoming_email", "test.org")
	mc.RecordWebhookFailure("incoming_email", "test.org", "timeout")

	// Check domain-specific metrics
	exampleMetrics := mc.GetDomainMetrics("incoming_email", "example.com")
	assert.Equal(t, int64(1), exampleMetrics.TotalAttempts)
	assert.Equal(t, int64(1), exampleMetrics.Successful)
	assert.Equal(t, int64(0), exampleMetrics.Failed)
	assert.Equal(t, 150*time.Millisecond, exampleMetrics.AverageLatency)
	assert.True(t, exampleMetrics.LastSuccess.After(time.Time{}))
	assert.True(t, exampleMetrics.LastFailure.IsZero())

	testMetrics := mc.GetDomainMetrics("incoming_email", "test.org")
	assert.Equal(t, int64(1), testMetrics.TotalAttempts)
	assert.Equal(t, int64(0), testMetrics.Successful)
	assert.Equal(t, int64(1), testMetrics.Failed)
	assert.True(t, testMetrics.LastSuccess.IsZero())
	assert.True(t, testMetrics.LastFailure.After(time.Time{}))
}

func TestMetricsCollector_Retries(t *testing.T) {
	mc := NewMetricsCollector()

	// Record retries
	mc.RecordWebhookRetries("incoming_email", "example.com", 1) // No retries (first attempt)
	mc.RecordWebhookRetries("incoming_email", "example.com", 3) // 2 retries
	mc.RecordWebhookRetries("incoming_email", "example.com", 5) // 4 retries

	metrics := mc.GetWebhookMetrics("incoming_email")
	assert.Equal(t, int64(6), metrics.TotalRetries) // 0 + 2 + 4 = 6
}

func TestMetricsCollector_MultipleWebhookTypes(t *testing.T) {
	mc := NewMetricsCollector()

	// Record metrics for different webhook types
	mc.RecordWebhookAttempt("incoming_email", "example.com")
	mc.RecordWebhookSuccess("incoming_email", "example.com")

	mc.RecordWebhookAttempt("outgoing_email", "example.com")
	mc.RecordWebhookFailure("outgoing_email", "example.com", "timeout")

	// Check metrics are separate
	incomingMetrics := mc.GetWebhookMetrics("incoming_email")
	assert.Equal(t, int64(1), incomingMetrics.TotalAttempts)
	assert.Equal(t, int64(1), incomingMetrics.Successful)
	assert.Equal(t, int64(0), incomingMetrics.Failed)

	outgoingMetrics := mc.GetWebhookMetrics("outgoing_email")
	assert.Equal(t, int64(1), outgoingMetrics.TotalAttempts)
	assert.Equal(t, int64(0), outgoingMetrics.Successful)
	assert.Equal(t, int64(1), outgoingMetrics.Failed)
}

func TestMetricsCollector_GetAllMetrics(t *testing.T) {
	mc := NewMetricsCollector()

	mc.RecordWebhookAttempt("incoming_email", "example.com")
	mc.RecordWebhookAttempt("outgoing_email", "example.com")

	allMetrics := mc.GetAllMetrics()
	assert.Len(t, allMetrics, 2)
	assert.Contains(t, allMetrics, "incoming_email")
	assert.Contains(t, allMetrics, "outgoing_email")
}

func TestMetricsCollector_SuccessRate(t *testing.T) {
	mc := NewMetricsCollector()

	// No data initially
	rate := mc.GetSuccessRate("incoming_email")
	assert.Equal(t, 0.0, rate)

	// Record some successful and failed attempts
	mc.RecordWebhookSuccess("incoming_email", "example.com")
	mc.RecordWebhookSuccess("incoming_email", "example.com")
	mc.RecordWebhookFailure("incoming_email", "example.com", "timeout")

	rate = mc.GetSuccessRate("incoming_email")
	assert.InDelta(t, 0.6667, rate, 0.001) // 2 successes out of 3 total

	// Domain-specific success rate
	domainRate := mc.GetDomainSuccessRate("incoming_email", "example.com")
	assert.InDelta(t, 0.6667, domainRate, 0.001)
}

func TestMetricsCollector_HealthChecks(t *testing.T) {
	mc := NewMetricsCollector()

	// No data - should not be healthy
	assert.False(t, mc.IsHealthy("incoming_email", 0.8))

	// Add successful webhooks
	mc.RecordWebhookSuccess("incoming_email", "example.com")
	mc.RecordWebhookSuccess("incoming_email", "example.com")
	mc.RecordWebhookSuccess("incoming_email", "example.com")
	mc.RecordWebhookSuccess("incoming_email", "example.com")
	mc.RecordWebhookFailure("incoming_email", "example.com", "timeout")

	// Success rate = 4/5 = 0.8
	assert.True(t, mc.IsHealthy("incoming_email", 0.8))
	assert.False(t, mc.IsHealthy("incoming_email", 0.9))

	// Domain-specific health
	assert.True(t, mc.IsDomainHealthy("incoming_email", "example.com", 0.8))
	assert.False(t, mc.IsDomainHealthy("incoming_email", "example.com", 0.9))
}

func TestMetricsCollector_Reset(t *testing.T) {
	mc := NewMetricsCollector()

	// Add some data
	mc.RecordWebhookAttempt("incoming_email", "example.com")
	mc.RecordWebhookSuccess("incoming_email", "example.com")
	mc.RecordWebhookAttempt("outgoing_email", "example.com")

	// Verify data exists
	assert.Equal(t, int64(1), mc.GetWebhookMetrics("incoming_email").TotalAttempts)
	assert.Equal(t, int64(1), mc.GetWebhookMetrics("outgoing_email").TotalAttempts)

	// Reset specific webhook type
	mc.ResetWebhookMetrics("incoming_email")
	assert.Equal(t, int64(0), mc.GetWebhookMetrics("incoming_email").TotalAttempts)
	assert.Equal(t, int64(1), mc.GetWebhookMetrics("outgoing_email").TotalAttempts)

	// Reset all metrics
	mc.ResetMetrics()
	assert.Equal(t, int64(0), mc.GetWebhookMetrics("outgoing_email").TotalAttempts)
}

func TestMetricsCollector_ConcurrentAccess(t *testing.T) {
	mc := NewMetricsCollector()

	// Test concurrent access doesn't panic
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()

			for j := 0; j < 100; j++ {
				mc.RecordWebhookAttempt("incoming_email", "example.com")
				mc.RecordWebhookSuccess("incoming_email", "example.com")
				mc.GetWebhookMetrics("incoming_email")
				mc.GetDomainMetrics("incoming_email", "example.com")
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify final counts
	metrics := mc.GetWebhookMetrics("incoming_email")
	assert.Equal(t, int64(1000), metrics.TotalAttempts) // 10 goroutines * 100 iterations
	assert.Equal(t, int64(1000), metrics.Successful)
}

func TestMetricsCollector_NonExistentData(t *testing.T) {
	mc := NewMetricsCollector()

	// Getting metrics for non-existent webhook type should return empty metrics
	metrics := mc.GetWebhookMetrics("non_existent")
	assert.Equal(t, int64(0), metrics.TotalAttempts)
	assert.NotNil(t, metrics.FailureReasons)
	assert.NotNil(t, metrics.DomainMetrics)

	// Getting domain metrics for non-existent data should return empty metrics
	domainMetrics := mc.GetDomainMetrics("non_existent", "example.com")
	assert.Equal(t, int64(0), domainMetrics.TotalAttempts)

	// Success rate for non-existent data should be 0
	rate := mc.GetSuccessRate("non_existent")
	assert.Equal(t, 0.0, rate)

	domainRate := mc.GetDomainSuccessRate("non_existent", "example.com")
	assert.Equal(t, 0.0, domainRate)
}
