package webhook

import (
	"context"
	"fmt"
	"github.com/mailvault/mailvault/domain/entities"
	"github.com/mailvault/mailvault/domain/webhook_config"
)

// ConfigLoader loads webhook configurations from both new and legacy sources
type ConfigLoader struct {
	webhookConfigRepo webhook_config.Repository
}

// NewConfigLoader creates a new webhook configuration loader
func NewConfigLoader(webhookConfigRepo webhook_config.Repository) *ConfigLoader {
	return &ConfigLoader{
		webhookConfigRepo: webhookConfigRepo,
	}
}

// LoadActiveWebhookConfigs loads all active webhook configurations for a domain
func (l *ConfigLoader) LoadActiveWebhookConfigs(ctx context.Context, domain *entities.Domain) ([]*entities.WebhookConfiguration, error) {
	configs, err := l.webhookConfigRepo.GetActiveByDomainID(ctx, domain.ID)
	if err != nil {
		return nil, fmt.Errorf("load active webhook configs: %w", err)
	}

	return configs, nil
}

// LoadWebhookConfigsForEvent loads webhook configurations that should receive a specific event type
func (l *ConfigLoader) LoadWebhookConfigsForEvent(ctx context.Context, domain *entities.Domain, eventType string) ([]*entities.WebhookConfiguration, error) {
	configs, err := l.LoadActiveWebhookConfigs(ctx, domain)
	if err != nil {
		return nil, err
	}

	// Filter configs that should receive this event type
	var filtered []*entities.WebhookConfiguration
	for _, config := range configs {
		if config.ShouldSendEvent(eventType) && config.CanAttemptDelivery() {
			filtered = append(filtered, config)
		}
	}

	return filtered, nil
}

// RecordWebhookResult records the result of a webhook delivery
// This updates metrics and circuit breaker state for the webhook configuration
func (l *ConfigLoader) RecordWebhookResult(ctx context.Context, config *entities.WebhookConfiguration, success bool, responseTimeMs int, errorMsg string) error {
	if success {
		config.RecordSuccess(responseTimeMs)
	} else {
		config.RecordFailure(errorMsg)
	}

	// Update in database
	if err := l.webhookConfigRepo.Update(ctx, config); err != nil {
		return fmt.Errorf("update webhook config after delivery: %w", err)
	}

	return nil
}

// CreateHealthCheck creates a health check record for a webhook configuration
func (l *ConfigLoader) CreateHealthCheck(ctx context.Context, check *entities.WebhookHealthCheck) error {
	return l.webhookConfigRepo.CreateHealthCheck(ctx, check)
}
