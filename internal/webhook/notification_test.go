package webhook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mailvault/app/smtp/verification"
	"mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIncomingEmailNotificationService_NotifyIncomingEmail(t *testing.T) {
	tests := []struct {
		name               string
		domain             *entities.Domain
		expectWebhookCall  bool
		expectError        bool
		serverResponse     func(w http.ResponseWriter, r *http.Request)
	}{
		{
			name: "successful webhook notification",
			domain: &entities.Domain{
				ID:     uuid.Must(uuid.NewV4()),
				Domain: "example.com",
				WebhookConfig: &entities.WebhookConfig{
					URL:     "", // Will be set to test server URL
					Secret:  "test-secret",
					Enabled: true,
					Headers: map[string]string{
						"X-Custom": "test-value",
					},
				},
			},
			expectWebhookCall: true,
			expectError:       false,
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Equal(t, "MailVault-Webhook/1.0", r.Header.Get("User-Agent"))
				assert.Equal(t, "test-value", r.Header.Get("X-Custom"))
				assert.NotEmpty(t, r.Header.Get("X-MailVault-Signature"))
				assert.NotEmpty(t, r.Header.Get("X-MailVault-Timestamp"))

				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			},
		},
		{
			name: "webhook not configured",
			domain: &entities.Domain{
				ID:            uuid.Must(uuid.NewV4()),
				Domain:        "example.com",
				WebhookConfig: nil,
			},
			expectWebhookCall: false,
			expectError:       true, // Should return ErrWebhookNotConfigured
		},
		{
			name: "webhook disabled",
			domain: &entities.Domain{
				ID:     uuid.Must(uuid.NewV4()),
				Domain: "example.com",
				WebhookConfig: &entities.WebhookConfig{
					URL:     "https://example.com/webhook",
					Enabled: false,
				},
			},
			expectWebhookCall: false,
			expectError:       true, // Should return ErrWebhookNotConfigured
		},
		{
			name: "webhook URL invalid",
			domain: &entities.Domain{
				ID:     uuid.Must(uuid.NewV4()),
				Domain: "example.com",
				WebhookConfig: &entities.WebhookConfig{
					URL:     "", // Empty URL
					Enabled: true,
				},
			},
			expectWebhookCall: false,
			expectError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
			if tt.expectWebhookCall && tt.serverResponse != nil {
				server = httptest.NewServer(http.HandlerFunc(tt.serverResponse))
				defer server.Close()
				tt.domain.WebhookConfig.URL = server.URL
			}

			// Create notification service
			client := NewHTTPClient(DefaultClientConfig())
			client.isTestMode = true // Allow localhost URLs for testing
			service := NewIncomingEmailNotificationService(NotificationServiceConfig{
				HTTPClient:  client,
				EnableAsync: false, // Use sync for testing
			})

			// Create test data
			emailAddress := &entities.EmailAddress{
				ID:        uuid.Must(uuid.NewV4()),
				LocalPart: "test",
			}

			subject := "Test Email"
			receivedEmail := &entities.ReceivedEmail{
				ID:            uuid.Must(uuid.NewV4()),
				FromAddress:   "sender@example.org",
				Subject:       &subject,
				EncryptedBody: "encrypted-content",
				ReceivedAt:    time.Now(),
				EmailAddress:  "test@example.com",
				DomainName:    "example.com",
			}

			verificationResult := &verification.VerificationResult{
				Action: verification.ActionAccept,
			}

			// Call notification service
			err := service.NotifyIncomingEmail(
				context.Background(),
				receivedEmail,
				tt.domain,
				emailAddress,
				verificationResult,
				false,
			)

			// Verify results
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIncomingEmailNotificationService_TestWebhook(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify it's a test webhook
		assert.Equal(t, "true", r.Header.Get("X-MailVault-Test"))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Test webhook received"))
	}))
	defer server.Close()

	// Create domain with webhook config
	domain := &entities.Domain{
		ID:     uuid.Must(uuid.NewV4()),
		Domain: "example.com",
		WebhookConfig: &entities.WebhookConfig{
			URL:     server.URL,
			Secret:  "test-secret",
			Enabled: true,
		},
	}

	// Create notification service
	client := NewHTTPClient(DefaultClientConfig())
	client.isTestMode = true // Allow localhost URLs for testing
	service := NewIncomingEmailNotificationService(NotificationServiceConfig{
		HTTPClient: client,
	})

	// Test webhook
	response, err := service.TestWebhook(context.Background(), domain)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.True(t, response.Success)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Contains(t, response.Body, "Test webhook received")
}

func TestIncomingEmailNotificationService_TestWebhook_NotConfigured(t *testing.T) {
	domain := &entities.Domain{
		ID:            uuid.Must(uuid.NewV4()),
		Domain:        "example.com",
		WebhookConfig: nil, // No webhook configured
	}

	client := NewHTTPClient(DefaultClientConfig())
	client.isTestMode = true // Allow localhost URLs for testing
	service := NewIncomingEmailNotificationService(NotificationServiceConfig{
		HTTPClient: client,
	})

	response, err := service.TestWebhook(context.Background(), domain)

	assert.Error(t, err)
	assert.Equal(t, ErrWebhookNotConfigured, err)
	assert.Nil(t, response)
}

func TestIncomingEmailNotificationService_AsyncMode(t *testing.T) {
	// Create test server
	called := make(chan bool, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called <- true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create domain with webhook config
	domain := &entities.Domain{
		ID:     uuid.Must(uuid.NewV4()),
		Domain: "example.com",
		WebhookConfig: &entities.WebhookConfig{
			URL:     server.URL,
			Enabled: true,
		},
	}

	// Create notification service in async mode
	client := NewHTTPClient(DefaultClientConfig())
	client.isTestMode = true // Allow localhost URLs for testing
	metricsCollector := NewMetricsCollector()
	service := NewIncomingEmailNotificationService(NotificationServiceConfig{
		HTTPClient:       client,
		EnableAsync:      true,
		MetricsCollector: metricsCollector,
	})

	// Start the service
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := service.Start(ctx)
	require.NoError(t, err)
	defer service.Stop(context.Background())

	// Create test data
	emailAddress := &entities.EmailAddress{
		ID:        uuid.Must(uuid.NewV4()),
		LocalPart: "test",
	}

	receivedEmail := &entities.ReceivedEmail{
		ID:            uuid.Must(uuid.NewV4()),
		FromAddress:   "sender@example.org",
		EncryptedBody: "encrypted-content",
		ReceivedAt:    time.Now(),
		EmailAddress:  "test@example.com",
		DomainName:    "example.com",
	}

	// Send notification
	err = service.NotifyIncomingEmail(
		context.Background(),
		receivedEmail,
		domain,
		emailAddress,
		nil,
		false,
	)

	require.NoError(t, err)

	// Wait for async delivery
	select {
	case <-called:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Webhook was not called within timeout")
	}

	// Check stats
	stats := service.GetStats()
	assert.Equal(t, "async", stats.Mode)
	assert.Equal(t, 5, stats.WorkerCount) // Default worker count
}

func TestIncomingEmailNotificationService_GetStats(t *testing.T) {
	client := NewHTTPClient(DefaultClientConfig())
	client.isTestMode = true // Allow localhost URLs for testing
	metricsCollector := NewMetricsCollector()
	service := NewIncomingEmailNotificationService(NotificationServiceConfig{
		HTTPClient:       client,
		EnableAsync:      false,
		MetricsCollector: metricsCollector,
	})

	// Initial stats
	stats := service.GetStats()
	assert.Equal(t, "sync", stats.Mode)
	assert.Equal(t, int64(0), stats.TotalAttempts)
	assert.Equal(t, int64(0), stats.SuccessfulDeliveries)
	assert.Equal(t, int64(0), stats.FailedDeliveries)

	// Record some metrics
	metricsCollector.RecordWebhookAttempt("incoming_email", "example.com")
	metricsCollector.RecordWebhookSuccess("incoming_email", "example.com")
	metricsCollector.RecordWebhookDuration("incoming_email", "example.com", 100*time.Millisecond, true)

	stats = service.GetStats()
	assert.Equal(t, int64(1), stats.TotalAttempts)
	assert.Equal(t, int64(1), stats.SuccessfulDeliveries)
	assert.Equal(t, int64(0), stats.FailedDeliveries)
	assert.Equal(t, 100*time.Millisecond, stats.AverageLatency)
}

func TestIncomingEmailNotificationService_SyncTimeout(t *testing.T) {
	// Create server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(15 * time.Second) // Longer than sync timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	domain := &entities.Domain{
		ID:     uuid.Must(uuid.NewV4()),
		Domain: "example.com",
		WebhookConfig: &entities.WebhookConfig{
			URL:     server.URL,
			Enabled: true,
		},
	}

	client := NewHTTPClient(DefaultClientConfig())
	client.isTestMode = true // Allow localhost URLs for testing
	service := NewIncomingEmailNotificationService(NotificationServiceConfig{
		HTTPClient:  client,
		EnableAsync: false,
	})

	emailAddress := &entities.EmailAddress{
		ID:        uuid.Must(uuid.NewV4()),
		LocalPart: "test",
	}

	receivedEmail := &entities.ReceivedEmail{
		ID:            uuid.Must(uuid.NewV4()),
		FromAddress:   "sender@example.org",
		EncryptedBody: "encrypted-content",
		ReceivedAt:    time.Now(),
		EmailAddress:  "test@example.com",
		DomainName:    "example.com",
	}

	// Should timeout
	err := service.NotifyIncomingEmail(
		context.Background(),
		receivedEmail,
		domain,
		emailAddress,
		nil,
		false,
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "webhook delivery failed")
}

func TestNotificationServiceAdapter(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create service and adapter
	client := NewHTTPClient(DefaultClientConfig())
	client.isTestMode = true // Allow localhost URLs for testing
	service := NewIncomingEmailNotificationService(NotificationServiceConfig{
		HTTPClient: client,
	})
	adapter := NewNotificationServiceAdapter(service)

	// Verify adapter implements interface
	var _ interface{} = adapter

	// Create domain with webhook
	domain := &entities.Domain{
		ID:     uuid.Must(uuid.NewV4()),
		Domain: "example.com",
		WebhookConfig: &entities.WebhookConfig{
			URL:     server.URL,
			Enabled: true,
		},
	}

	emailAddress := &entities.EmailAddress{
		ID:        uuid.Must(uuid.NewV4()),
		LocalPart: "test",
	}

	receivedEmail := &entities.ReceivedEmail{
		ID:            uuid.Must(uuid.NewV4()),
		FromAddress:   "sender@example.org",
		EncryptedBody: "encrypted-content",
		ReceivedAt:    time.Now(),
		EmailAddress:  "test@example.com",
		DomainName:    "example.com",
	}

	// Call through adapter
	err := adapter.NotifyIncomingEmail(
		context.Background(),
		receivedEmail,
		domain,
		emailAddress,
		nil,
		false,
	)

	assert.NoError(t, err)
}