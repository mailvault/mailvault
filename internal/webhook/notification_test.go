package webhook

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"mailvault/app/smtp/verification"
	"mailvault/domain/entities"
	"mailvault/domain/webhook_config"
	"mailvault/domain/webhook_config/mocks"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// notificationTestFixture wires a notification service against a fake
// webhook_config repository so each test can declare exactly which
// configurations the loader returns.
type notificationTestFixture struct {
	t       *testing.T
	repo    *mocks.RepositoryMock
	loader  *ConfigLoader
	service *IncomingEmailNotificationService
	domain  *entities.Domain
}

func newNotificationFixture(t *testing.T, configs []*entities.WebhookConfiguration) *notificationTestFixture {
	t.Helper()

	repo := &mocks.RepositoryMock{
		GetActiveByDomainIDFunc: func(ctx context.Context, domainID uuid.UUID) ([]*entities.WebhookConfiguration, error) {
			return configs, nil
		},
		UpdateFunc: func(ctx context.Context, config *entities.WebhookConfiguration) error {
			return nil
		},
		CreateAuditFunc: func(ctx context.Context, audit *entities.WebhookConfigurationAudit) error {
			return nil
		},
		CreateHealthCheckFunc: func(ctx context.Context, check *entities.WebhookHealthCheck) error {
			return nil
		},
	}

	loader := NewConfigLoader(repo)

	client := NewHTTPClient(DefaultClientConfig())
	client.isTestMode = true // allow localhost URLs

	service := NewIncomingEmailNotificationService(NotificationServiceConfig{
		HTTPClient:   client,
		ConfigLoader: loader,
		EnableAsync:  false,
	})

	return &notificationTestFixture{
		t:       t,
		repo:    repo,
		loader:  loader,
		service: service,
		domain: &entities.Domain{
			ID:     uuid.Must(uuid.NewV4()),
			Domain: "example.com",
		},
	}
}

func (f *notificationTestFixture) sampleEvent() (*entities.ReceivedEmail, *entities.EmailAddress, *verification.VerificationResult) {
	subject := "Hello"
	received := &entities.ReceivedEmail{
		ID:            uuid.Must(uuid.NewV4()),
		FromAddress:   "sender@elsewhere.test",
		Subject:       &subject,
		EncryptedBody: "ciphertext",
		ReceivedAt:    time.Now(),
		EmailAddress:  "inbox@example.com",
		DomainName:    "example.com",
	}
	addr := &entities.EmailAddress{
		ID:        uuid.Must(uuid.NewV4()),
		LocalPart: "inbox",
	}
	verResult := &verification.VerificationResult{Action: verification.ActionAccept}
	return received, addr, verResult
}

func enabledWebhookConfig(url string) *entities.WebhookConfiguration {
	return &entities.WebhookConfiguration{
		ID:                    uuid.Must(uuid.NewV4()),
		DomainID:              uuid.Must(uuid.NewV4()),
		Name:                  "test",
		URL:                   url,
		Method:                "POST",
		Enabled:               true,
		EventTypes:            []string{"*"},
		TimeoutSeconds:        5,
		CircuitBreakerEnabled: false,
		CircuitBreakerState:   entities.CircuitBreakerStateClosed,
		HealthStatus:          entities.WebhookHealthStatusHealthy,
		MaxRetries:            0,
		AuthType:              entities.WebhookAuthTypeNone,
	}
}

func TestNotificationService_NoConfigLoader_ReturnsError(t *testing.T) {
	client := NewHTTPClient(DefaultClientConfig())
	client.isTestMode = true

	service := NewIncomingEmailNotificationService(NotificationServiceConfig{
		HTTPClient: client,
		// No ConfigLoader: must surface an error.
	})

	domain := &entities.Domain{ID: uuid.Must(uuid.NewV4()), Domain: "example.com"}
	subject := "Hi"
	received := &entities.ReceivedEmail{
		ID: uuid.Must(uuid.NewV4()), FromAddress: "a@b.test", Subject: &subject,
		EncryptedBody: "x", ReceivedAt: time.Now(),
		EmailAddress: "inbox@example.com", DomainName: "example.com",
	}
	addr := &entities.EmailAddress{ID: uuid.Must(uuid.NewV4()), LocalPart: "inbox"}

	err := service.NotifyIncomingEmail(context.Background(), received, domain, addr,
		&verification.VerificationResult{Action: verification.ActionAccept}, false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config loader")
}

func TestNotificationService_NoConfigsForDomain_ReturnsErrWebhookNotConfigured(t *testing.T) {
	fix := newNotificationFixture(t, nil) // empty config list

	received, addr, verResult := fix.sampleEvent()

	err := fix.service.NotifyIncomingEmail(context.Background(), received, fix.domain, addr, verResult, false)

	assert.ErrorIs(t, err, ErrWebhookNotConfigured)
}

func TestNotificationService_SuccessfulSyncDelivery(t *testing.T) {
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	fix := newNotificationFixture(t, []*entities.WebhookConfiguration{
		enabledWebhookConfig(server.URL),
	})

	received, addr, verResult := fix.sampleEvent()

	err := fix.service.NotifyIncomingEmail(context.Background(), received, fix.domain, addr, verResult, false)

	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&hits))
}

func TestNotificationService_AllDeliveriesFail_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	fix := newNotificationFixture(t, []*entities.WebhookConfiguration{
		enabledWebhookConfig(server.URL),
	})

	received, addr, verResult := fix.sampleEvent()

	err := fix.service.NotifyIncomingEmail(context.Background(), received, fix.domain, addr, verResult, false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all webhook deliveries failed")
}

func TestNotificationService_SkipsCircuitBreakerOpenConfig(t *testing.T) {
	// Server should NOT be called for the open-circuit config.
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	openedAt := time.Now().Add(-1 * time.Second) // Open recently — still within timeout window.
	openCfg := enabledWebhookConfig(server.URL)
	openCfg.CircuitBreakerEnabled = true
	openCfg.CircuitBreakerState = entities.CircuitBreakerStateOpen
	openCfg.CircuitBreakerOpenedAt = &openedAt
	openCfg.CircuitBreakerTimeoutSeconds = 600

	fix := newNotificationFixture(t, []*entities.WebhookConfiguration{openCfg})

	received, addr, verResult := fix.sampleEvent()

	// The loader filters out !ShouldSendEvent configs, so this returns
	// ErrWebhookNotConfigured because no configs survive filtering.
	err := fix.service.NotifyIncomingEmail(context.Background(), received, fix.domain, addr, verResult, false)
	assert.ErrorIs(t, err, ErrWebhookNotConfigured)
	assert.Equal(t, int32(0), atomic.LoadInt32(&hits))
}

func TestNotificationService_RepositoryErrorPropagates(t *testing.T) {
	repo := &mocks.RepositoryMock{
		GetActiveByDomainIDFunc: func(ctx context.Context, domainID uuid.UUID) ([]*entities.WebhookConfiguration, error) {
			return nil, errors.New("postgres down")
		},
	}
	loader := NewConfigLoader(repo)

	client := NewHTTPClient(DefaultClientConfig())
	client.isTestMode = true
	service := NewIncomingEmailNotificationService(NotificationServiceConfig{
		HTTPClient:   client,
		ConfigLoader: loader,
	})

	domain := &entities.Domain{ID: uuid.Must(uuid.NewV4()), Domain: "example.com"}
	subject := "Hi"
	received := &entities.ReceivedEmail{
		ID: uuid.Must(uuid.NewV4()), FromAddress: "a@b.test", Subject: &subject,
		EncryptedBody: "x", ReceivedAt: time.Now(),
		EmailAddress: "inbox@example.com", DomainName: "example.com",
	}
	addr := &entities.EmailAddress{ID: uuid.Must(uuid.NewV4()), LocalPart: "inbox"}

	err := service.NotifyIncomingEmail(context.Background(), received, domain, addr,
		&verification.VerificationResult{Action: verification.ActionAccept}, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "load webhook configurations")
}

// Compile-time assertion that the mock satisfies the Repository interface.
var _ webhook_config.Repository = (*mocks.RepositoryMock)(nil)
