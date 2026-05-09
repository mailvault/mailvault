package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mailvault/app/smtp/verification"
	"mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookIntegration_EndToEnd(t *testing.T) {
	skipDeprecatedNotificationTest(t)
	// Track received webhooks
	receivedWebhooks := make([]IncomingEmailEvent, 0)
	webhookCalled := make(chan bool, 1)

	// Create test webhook server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request format
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "MailVault-Webhook/1.0", r.Header.Get("User-Agent"))
		assert.NotEmpty(t, r.Header.Get("X-MailVault-Signature"))
		assert.NotEmpty(t, r.Header.Get("X-MailVault-Timestamp"))
		assert.Equal(t, "v1", r.Header.Get("X-MailVault-Signature-Version"))

		// Read and parse payload
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var webhook IncomingEmailEvent
		err = json.Unmarshal(body, &webhook)
		require.NoError(t, err)

		// Verify webhook signature
		secret := "test-secret-123"
		signature := r.Header.Get("X-MailVault-Signature")
		assert.True(t, VerifyWebhookSignature(body, signature, secret))

		// Store received webhook
		receivedWebhooks = append(receivedWebhooks, webhook)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Webhook processed successfully"))

		webhookCalled <- true
	}))
	defer server.Close()

	// Create domain with webhook configuration
	domain := &entities.Domain{
		ID:              uuid.Must(uuid.NewV4()),
		UserID:          uuid.Must(uuid.NewV4()),
		Domain:          "example.com",
		PublicKey:       "test-public-key",
		APIKey:          "test-api-key",
		StorageEnabled:  true,
		AutoCreateAddress: true,
		VerificationStatus: entities.VerificationStatusVerified,
		WebhookConfig: &entities.WebhookConfig{
			URL:     server.URL,
			Secret:  "test-secret-123",
			Enabled: true,
			Headers: map[string]string{
				"X-Source":      "MailVault-Test",
				"Authorization": "Bearer test-token",
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create email address
	emailAddress := &entities.EmailAddress{
		ID:               uuid.Must(uuid.NewV4()),
		DomainID:         domain.ID,
		LocalPart:        "hello",
		ForwardAddresses: []string{},
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// Create received email
	subject := "Welcome to MailVault"
	receivedEmail := &entities.ReceivedEmail{
		ID:             uuid.Must(uuid.NewV4()),
		EmailAddressID: &emailAddress.ID,
		SequenceNumber: 1,
		FromAddress:    "noreply@company.com",
		Subject:        &subject,
		EncryptedBody:  "VGhpcyBpcyBhIHRlc3QgZW1haWwgYm9keQ==", // Base64 encoded test content
		DomainName:     domain.Domain,
		EmailAddress:   emailAddress.LocalPart + "@" + domain.Domain,
		ReceivedAt:     time.Now(),
	}

	// Create verification result
	verificationResult := &verification.VerificationResult{
		Action: verification.ActionAccept,
		Content: verification.ContentResult{
			SpamScore: 0.05,
		},
		Reputation: verification.ReputationResult{
			Score: 0.95,
		},
		SPF: verification.SPFResult{
			Result:    verification.SPFPass,
			Mechanism: "company.com",
			Error:     "",
		},
		DKIM: verification.DKIMResult{
			Valid: true,
			Results: []verification.DKIMSignatureResult{
				{
					Domain:   "company.com",
					Selector: "selector1",
					Status:   verification.DKIMPass,
				},
			},
		},
		DMARC: verification.DMARCResult{
			Result:    verification.DMARCPass,
			Policy:    "quarantine",
			SPFAlign:  true,
			DKIMAlign: true,
		},
	}

	// Create webhook notification system
	clientConfig := DefaultClientConfig()
	clientConfig.BaseDelay = 10 * time.Millisecond // Fast retries for testing
	client := NewHTTPClient(clientConfig)
	client.isTestMode = true // Allow localhost URLs for testing

	metricsCollector := NewMetricsCollector()

	service := NewIncomingEmailNotificationService(NotificationServiceConfig{
		HTTPClient:       client,
		EnableAsync:      false, // Use sync for predictable testing
		MetricsCollector: metricsCollector,
	})

	// Send webhook notification
	ctx := context.Background()
	err := service.NotifyIncomingEmail(
		ctx,
		receivedEmail,
		domain,
		emailAddress,
		verificationResult,
		false, // Not auto-created
	)

	require.NoError(t, err)

	// Wait for webhook to be called
	select {
	case <-webhookCalled:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Webhook was not called within timeout")
	}

	// Verify webhook was received
	require.Len(t, receivedWebhooks, 1)
	webhook := receivedWebhooks[0]

	// Verify webhook content
	assert.Equal(t, "email.received", webhook.EventType)
	assert.NotEqual(t, uuid.Nil, webhook.EventID)
	assert.WithinDuration(t, time.Now(), webhook.Timestamp, 10*time.Second)

	// Verify email metadata
	assert.Equal(t, receivedEmail.ID, webhook.Email.ID)
	assert.Equal(t, receivedEmail.SequenceNumber, webhook.Email.SequenceNumber)
	assert.Equal(t, receivedEmail.FromAddress, webhook.Email.FromAddress)
	assert.Equal(t, "Welcome to MailVault", webhook.Email.Subject)
	assert.Equal(t, receivedEmail.EncryptedBody, webhook.Email.EncryptedBody)
	assert.WithinDuration(t, receivedEmail.ReceivedAt, webhook.Email.ReceivedAt, time.Second)
	assert.False(t, webhook.Email.IsQuarantined)

	// Verify recipient info
	assert.Equal(t, "hello@example.com", webhook.Recipient.EmailAddress)
	assert.Equal(t, "hello", webhook.Recipient.LocalPart)
	assert.Equal(t, "example.com", webhook.Recipient.DomainName)
	assert.Equal(t, emailAddress.ID, webhook.Recipient.AddressID)
	assert.False(t, webhook.Recipient.AutoCreated)

	// Verify security info
	assert.Equal(t, "accept", webhook.Security.VerificationAction)
	assert.Equal(t, 0.05, webhook.Security.SpamScore)
	assert.Equal(t, 0.95, webhook.Security.ReputationScore)

	// SPF
	assert.Equal(t, "pass", webhook.Security.SPF.Result)
	assert.Equal(t, "company.com", webhook.Security.SPF.Domain)
	assert.Equal(t, "", webhook.Security.SPF.Explanation)

	// DKIM
	assert.True(t, webhook.Security.DKIM.Valid)
	assert.Equal(t, "company.com", webhook.Security.DKIM.Domain)
	assert.Equal(t, "selector1", webhook.Security.DKIM.Selector)

	// DMARC
	assert.Equal(t, "pass", webhook.Security.DMARC.Result)
	assert.Equal(t, "quarantine", webhook.Security.DMARC.Policy)
	assert.Equal(t, "", webhook.Security.DMARC.Disposition)
	assert.True(t, webhook.Security.DMARC.AlignmentSPF)
	assert.True(t, webhook.Security.DMARC.AlignmentDKIM)

	// Verify domain info
	assert.Equal(t, domain.ID, webhook.Domain.ID)
	assert.Equal(t, domain.Domain, webhook.Domain.Name)
	assert.Equal(t, domain.StorageEnabled, webhook.Domain.StorageEnabled)
	assert.Equal(t, domain.AutoCreateAddress, webhook.Domain.AutoCreateEmail)

	// Verify metrics
	metrics := metricsCollector.GetWebhookMetrics("incoming_email")
	assert.Equal(t, int64(1), metrics.TotalAttempts)
	assert.Equal(t, int64(1), metrics.Successful)
	assert.Equal(t, int64(0), metrics.Failed)

	domainMetrics := metricsCollector.GetDomainMetrics("incoming_email", "example.com")
	assert.Equal(t, int64(1), domainMetrics.TotalAttempts)
	assert.Equal(t, int64(1), domainMetrics.Successful)
	assert.Equal(t, int64(0), domainMetrics.Failed)
	assert.True(t, domainMetrics.LastSuccess.After(time.Time{}))
}

func TestWebhookIntegration_WithRetries(t *testing.T) {
	skipDeprecatedNotificationTest(t)
	attempts := 0
	webhookCalled := make(chan bool, 1)

	// Create server that fails first 2 attempts, then succeeds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Server Error"))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Success"))
		webhookCalled <- true
	}))
	defer server.Close()

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
		FromAddress:   "test@example.org",
		EncryptedBody: "test-content",
		ReceivedAt:    time.Now(),
		EmailAddress:  "test@example.com", // Add missing field
		DomainName:    "example.com",      // Add missing field
	}

	// Create service with fast retries
	clientConfig := DefaultClientConfig()
	clientConfig.BaseDelay = 10 * time.Millisecond
	client := NewHTTPClient(clientConfig)
	client.isTestMode = true // Allow localhost URLs for testing

	metricsCollector := NewMetricsCollector()
	service := NewIncomingEmailNotificationService(NotificationServiceConfig{
		HTTPClient:       client,
		MetricsCollector: metricsCollector,
	})

	// Send webhook
	err := service.NotifyIncomingEmail(
		context.Background(),
		receivedEmail,
		domain,
		emailAddress,
		nil,
		false,
	)

	require.NoError(t, err)

	// Wait for webhook success
	select {
	case <-webhookCalled:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Webhook was not called within timeout")
	}

	// Verify attempts
	assert.Equal(t, 3, attempts)

	// Verify metrics include retries
	metrics := metricsCollector.GetWebhookMetrics("incoming_email")
	assert.Equal(t, int64(1), metrics.TotalAttempts)
	assert.Equal(t, int64(1), metrics.Successful)
	assert.Equal(t, int64(0), metrics.Failed)
}

func TestWebhookIntegration_AsyncMode(t *testing.T) {
	skipDeprecatedNotificationTest(t)
	webhookCalled := make(chan bool, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		webhookCalled <- true
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

	// Create service in async mode
	client := NewHTTPClient(DefaultClientConfig())
	client.isTestMode = true // Allow localhost URLs for testing
	service := NewIncomingEmailNotificationService(NotificationServiceConfig{
		HTTPClient:  client,
		EnableAsync: true,
	})

	// Start async workers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := service.Start(ctx)
	require.NoError(t, err)
	defer service.Stop(context.Background())

	// Send webhook
	emailAddress := &entities.EmailAddress{
		ID:        uuid.Must(uuid.NewV4()),
		LocalPart: "test",
	}

	receivedEmail := &entities.ReceivedEmail{
		ID:            uuid.Must(uuid.NewV4()),
		FromAddress:   "test@example.org",
		EncryptedBody: "test-content",
		ReceivedAt:    time.Now(),
		EmailAddress:  "test@example.com", // Add missing field
		DomainName:    "example.com",      // Add missing field
	}

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
	case <-webhookCalled:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Async webhook was not delivered within timeout")
	}

	// Verify stats show async mode
	stats := service.GetStats()
	assert.Equal(t, "async", stats.Mode)
	assert.Greater(t, stats.WorkerCount, 0)
}

func TestWebhookIntegration_TestWebhook(t *testing.T) {
	skipDeprecatedNotificationTest(t)
	testWebhookReceived := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify it's a test webhook
		assert.Equal(t, "true", r.Header.Get("X-MailVault-Test"))

		// Parse payload to verify it's a test event
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var event IncomingEmailEvent
		err = json.Unmarshal(body, &event)
		require.NoError(t, err)

		// Verify test content
		assert.Equal(t, "email.received", event.EventType)
		assert.Equal(t, "test@example.com", event.Email.FromAddress)
		assert.Equal(t, "Test Webhook - MailVault", event.Email.Subject)
		assert.True(t, strings.Contains(event.Email.EncryptedBody, "VGhpcyBpcyBhIHRlc3QgZW1haWwgZm9yIHdlYmhvb2sgdmVyaWZpY2F0aW9u"))

		testWebhookReceived = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Test webhook received"))
	}))
	defer server.Close()

	domain := &entities.Domain{
		ID:     uuid.Must(uuid.NewV4()),
		Domain: "example.com",
		WebhookConfig: &entities.WebhookConfig{
			URL:     server.URL,
			Secret:  "test-secret",
			Enabled: true,
		},
	}

	client := NewHTTPClient(DefaultClientConfig())
	client.isTestMode = true // Allow localhost URLs for testing
	service := NewIncomingEmailNotificationService(NotificationServiceConfig{
		HTTPClient: client,
	})

	response, err := service.TestWebhook(context.Background(), domain)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.True(t, response.Success)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Contains(t, response.Body, "Test webhook received")
	assert.True(t, testWebhookReceived)
}