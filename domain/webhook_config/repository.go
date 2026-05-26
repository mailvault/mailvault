package webhook_config

import (
	"context"
	"github.com/mailvault/mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
)

//go:generate moq -skip-ensure -stub -pkg mocks -out mocks/repository.go . Repository

// Repository defines the interface for webhook configuration persistence
type Repository interface {
	// Webhook Configuration CRUD
	Create(ctx context.Context, config *entities.WebhookConfiguration) error
	GetByID(ctx context.Context, id uuid.UUID) (*entities.WebhookConfiguration, error)
	GetByDomainID(ctx context.Context, domainID uuid.UUID) ([]*entities.WebhookConfiguration, error)
	GetActiveByDomainID(ctx context.Context, domainID uuid.UUID) ([]*entities.WebhookConfiguration, error)
	Update(ctx context.Context, config *entities.WebhookConfiguration) error
	Delete(ctx context.Context, id uuid.UUID) error

	// Webhook Configuration Queries
	GetByDomainIDAndName(ctx context.Context, domainID uuid.UUID, name string) (*entities.WebhookConfiguration, error)
	ListUnhealthyWebhooks(ctx context.Context, limit int) ([]*entities.WebhookConfiguration, error)
	ListWebhooksForHealthCheck(ctx context.Context, olderThan int) ([]*entities.WebhookConfiguration, error)

	// Audit Trail
	CreateAudit(ctx context.Context, audit *entities.WebhookConfigurationAudit) error
	GetAuditByConfigID(ctx context.Context, configID uuid.UUID, limit, offset int) ([]*entities.WebhookConfigurationAudit, error)

	// Health Checks
	CreateHealthCheck(ctx context.Context, check *entities.WebhookHealthCheck) error
	GetHealthChecksByConfigID(ctx context.Context, configID uuid.UUID, limit, offset int) ([]*entities.WebhookHealthCheck, error)
	GetLatestHealthCheck(ctx context.Context, configID uuid.UUID) (*entities.WebhookHealthCheck, error)

	// Templates
	GetTemplates(ctx context.Context) ([]*entities.WebhookConfigurationTemplate, error)
	GetTemplateByID(ctx context.Context, id uuid.UUID) (*entities.WebhookConfigurationTemplate, error)
	GetTemplateByName(ctx context.Context, name string) (*entities.WebhookConfigurationTemplate, error)
	CreateTemplate(ctx context.Context, template *entities.WebhookConfigurationTemplate) error
	UpdateTemplate(ctx context.Context, template *entities.WebhookConfigurationTemplate) error
	IncrementTemplateUsage(ctx context.Context, templateID uuid.UUID) error
}
