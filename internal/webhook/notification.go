package webhook

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"mailvault/app/smtp/verification"
	"mailvault/domain/entities"
)

// IncomingEmailNotificationService handles webhook notifications for incoming emails
type IncomingEmailNotificationService struct {
	client           *HTTPClient
	logger           *slog.Logger
	asyncWorker      *AsyncWebhookWorker
	metricsCollector *MetricsCollector
	configLoader     *ConfigLoader
}

// NotificationServiceConfig configures the notification service
type NotificationServiceConfig struct {
	HTTPClient       *HTTPClient
	Logger           *slog.Logger
	EnableAsync      bool
	MetricsCollector *MetricsCollector
	ConfigLoader     *ConfigLoader
}

// NewIncomingEmailNotificationService creates a new notification service
func NewIncomingEmailNotificationService(config NotificationServiceConfig) *IncomingEmailNotificationService {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	service := &IncomingEmailNotificationService{
		client:           config.HTTPClient,
		logger:           config.Logger,
		metricsCollector: config.MetricsCollector,
		configLoader:     config.ConfigLoader,
	}

	// Initialize async worker if enabled
	if config.EnableAsync {
		workerConfig := AsyncWorkerConfig{
			BufferSize:       1000,
			WorkerCount:      5,
			RetryInterval:    30 * time.Second,
			MaxRetryAge:      24 * time.Hour,
			Logger:           config.Logger,
			HTTPClient:       config.HTTPClient,
			MetricsCollector: config.MetricsCollector,
		}
		service.asyncWorker = NewAsyncWebhookWorker(workerConfig)
	}

	return service
}

// NotifyIncomingEmail sends a webhook notification for an incoming email
func (s *IncomingEmailNotificationService) NotifyIncomingEmail(
	ctx context.Context,
	receivedEmail *entities.ReceivedEmail,
	domain *entities.Domain,
	emailAddress *entities.EmailAddress,
	verificationResult *verification.VerificationResult,
	autoCreated bool,
) error {
	// ConfigLoader is required
	if s.configLoader == nil {
		return fmt.Errorf("webhook config loader not initialized")
	}

	// Load active webhook configurations for this domain and event type
	webhookConfigs, err := s.configLoader.LoadWebhookConfigsForEvent(ctx, domain, "email.received")
	if err != nil {
		s.logger.Error("failed to load webhook configurations",
			slog.String("domain", domain.Domain),
			slog.String("error", err.Error()))
		return fmt.Errorf("load webhook configurations: %w", err)
	}

	// Check if any webhooks are configured
	if len(webhookConfigs) == 0 {
		s.logger.Debug("no active webhook configurations for domain",
			slog.String("domain", domain.Domain),
			slog.String("email_id", receivedEmail.ID.String()))
		return ErrWebhookNotConfigured
	}

	// Create webhook event (shared across all webhooks)
	event := NewIncomingEmailEvent(receivedEmail, domain, emailAddress, verificationResult, autoCreated)

	// Validate event
	if err := event.Validate(); err != nil {
		s.logger.Error("invalid webhook event",
			slog.String("domain", domain.Domain),
			slog.String("email_id", receivedEmail.ID.String()),
			slog.String("error", err.Error()))
		return fmt.Errorf("invalid webhook event: %w", err)
	}

	// Send to all configured webhooks
	var lastErr error
	successCount := 0

	for _, config := range webhookConfigs {
		// Skip if circuit breaker is open
		if !config.CanAttemptDelivery() {
			s.logger.Warn("skipping webhook due to circuit breaker",
				slog.String("domain", domain.Domain),
				slog.String("webhook_id", config.ID.String()),
				slog.String("webhook_url", config.URL))
			continue
		}

		// Create webhook request
		webhookReq := WebhookRequest{
			URL:     config.URL,
			Payload: event,
			Secret:  config.AuthSecret,
			Headers: config.CustomHeaders,
		}

		// Log webhook attempt
		s.logger.Info("sending incoming email webhook",
			slog.String("domain", domain.Domain),
			slog.String("email_id", receivedEmail.ID.String()),
			slog.String("event_id", event.EventID.String()),
			slog.String("webhook_id", config.ID.String()),
			slog.String("webhook_url", config.URL),
			slog.String("from", receivedEmail.FromAddress),
			slog.String("to", receivedEmail.EmailAddress))

		// Record webhook attempt metrics
		if s.metricsCollector != nil {
			s.metricsCollector.RecordWebhookAttempt("incoming_email", domain.Domain)
		}

		// Send webhook (async or sync based on configuration)
		var deliveryErr error
		if s.asyncWorker != nil {
			// Async delivery
			deliveryErr = s.asyncWorker.EnqueueWebhook(ctx, webhookReq, event)
		} else {
			// Sync delivery
			deliveryErr = s.sendWebhookSyncWithConfig(ctx, webhookReq, event, domain.Domain, config)
		}

		if deliveryErr != nil {
			lastErr = deliveryErr
			s.logger.Error("webhook delivery failed",
				slog.String("domain", domain.Domain),
				slog.String("webhook_id", config.ID.String()),
				slog.String("error", deliveryErr.Error()))
		} else {
			successCount++
		}
	}

	// Return error only if all webhooks failed
	if successCount == 0 && lastErr != nil {
		return fmt.Errorf("all webhook deliveries failed: %w", lastErr)
	}

	return nil
}

// sendWebhookSyncWithConfig sends webhook synchronously and updates webhook configuration metrics
func (s *IncomingEmailNotificationService) sendWebhookSyncWithConfig(
	ctx context.Context,
	webhookReq WebhookRequest,
	event *IncomingEmailEvent,
	domainName string,
	config *entities.WebhookConfiguration,
) error {
	start := time.Now()

	// Use config timeout if available, otherwise default to 10 seconds
	timeout := 10 * time.Second
	if config != nil && config.TimeoutSeconds > 0 {
		timeout = time.Duration(config.TimeoutSeconds) * time.Second
	}

	// Set a reasonable timeout for sync webhook calls (don't block email processing)
	webhookCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Send webhook
	response, err := s.client.SendWebhook(webhookCtx, webhookReq)
	duration := time.Since(start)
	responseTimeMs := int(duration.Milliseconds())

	// Log result
	if err != nil {
		s.logger.Error("webhook delivery failed",
			slog.String("domain", domainName),
			slog.String("event_id", event.EventID.String()),
			slog.String("error", err.Error()),
			slog.Duration("duration", duration))

		// Record failure metrics
		if s.metricsCollector != nil {
			s.metricsCollector.RecordWebhookFailure("incoming_email", domainName, "delivery_failed")
			s.metricsCollector.RecordWebhookDuration("incoming_email", domainName, duration, false)
		}

		// Update webhook configuration metrics
		if config != nil && s.configLoader != nil {
			if recordErr := s.configLoader.RecordWebhookResult(ctx, config, false, responseTimeMs, err.Error()); recordErr != nil {
				s.logger.Error("failed to record webhook failure",
					slog.String("domain", domainName),
					slog.String("webhook_id", config.ID.String()),
					slog.String("error", recordErr.Error()))
			}
		}

		return fmt.Errorf("webhook delivery failed: %w", err)
	}

	// Log successful delivery
	s.logger.Info("webhook delivered successfully",
		slog.String("domain", domainName),
		slog.String("event_id", event.EventID.String()),
		slog.Int("status_code", response.StatusCode),
		slog.Duration("duration", duration),
		slog.Int("attempts", response.Attempt))

	// Record success metrics
	if s.metricsCollector != nil {
		s.metricsCollector.RecordWebhookSuccess("incoming_email", domainName)
		s.metricsCollector.RecordWebhookDuration("incoming_email", domainName, duration, true)
		s.metricsCollector.RecordWebhookRetries("incoming_email", domainName, response.Attempt)
	}

	// Update webhook configuration metrics
	if config != nil && s.configLoader != nil {
		if recordErr := s.configLoader.RecordWebhookResult(ctx, config, true, responseTimeMs, ""); recordErr != nil {
			s.logger.Error("failed to record webhook success",
				slog.String("domain", domainName),
				slog.String("webhook_id", config.ID.String()),
				slog.String("error", recordErr.Error()))
		}
	}

	return nil
}

// Start starts the notification service (starts async worker if configured)
func (s *IncomingEmailNotificationService) Start(ctx context.Context) error {
	if s.asyncWorker != nil {
		return s.asyncWorker.Start(ctx)
	}
	return nil
}

// Stop stops the notification service
func (s *IncomingEmailNotificationService) Stop(ctx context.Context) error {
	if s.asyncWorker != nil {
		return s.asyncWorker.Stop(ctx)
	}
	return nil
}

// GetStats returns notification service statistics
func (s *IncomingEmailNotificationService) GetStats() NotificationStats {
	stats := NotificationStats{
		Mode: "sync",
	}

	if s.asyncWorker != nil {
		stats.Mode = "async"
		workerStats := s.asyncWorker.GetStats()
		stats.QueueSize = workerStats.QueueSize
		stats.PendingRetries = workerStats.PendingRetries
		stats.WorkerCount = workerStats.ActiveWorkers
	}

	if s.metricsCollector != nil {
		metrics := s.metricsCollector.GetWebhookMetrics("incoming_email")
		stats.TotalAttempts = metrics.TotalAttempts
		stats.SuccessfulDeliveries = metrics.Successful
		stats.FailedDeliveries = metrics.Failed
		stats.AverageLatency = metrics.AverageLatency
	}

	return stats
}

// NotificationStats represents statistics for the notification service
type NotificationStats struct {
	Mode                 string        `json:"mode"`                  // "sync" or "async"
	TotalAttempts        int64         `json:"total_attempts"`        // Total webhook attempts
	SuccessfulDeliveries int64         `json:"successful_deliveries"` // Successful deliveries
	FailedDeliveries     int64         `json:"failed_deliveries"`     // Failed deliveries
	QueueSize            int           `json:"queue_size"`            // Current queue size (async only)
	PendingRetries       int           `json:"pending_retries"`       // Pending retries (async only)
	WorkerCount          int           `json:"worker_count"`          // Active workers (async only)
	AverageLatency       time.Duration `json:"average_latency"`       // Average webhook latency
}

// TestWebhook has been deprecated. Use webhook_config.UseCase.TestWebhookConfiguration instead.
// This method is kept for backward compatibility but will be removed in a future version.
func (s *IncomingEmailNotificationService) TestWebhook(
	ctx context.Context,
	domain *entities.Domain,
) (*WebhookResponse, error) {
	return nil, fmt.Errorf("TestWebhook is deprecated, use webhook_config.UseCase.TestWebhookConfiguration instead")
}
