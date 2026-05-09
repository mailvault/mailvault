package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPClient_SendWebhook(t *testing.T) {
	tests := []struct {
		name           string
		serverHandler  func(w http.ResponseWriter, r *http.Request)
		request        WebhookRequest
		expectSuccess  bool
		expectAttempts int
		expectError    bool
	}{
		{
			name: "successful webhook delivery",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			},
			request: WebhookRequest{
				Payload: map[string]interface{}{
					"event_type": "email.received",
					"data":       "test",
				},
				Secret: "test-secret",
			},
			expectSuccess:  true,
			expectAttempts: 1,
			expectError:    false,
		},
		{
			name: "webhook with custom headers",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "Bearer token123", r.Header.Get("Authorization"))
				assert.Equal(t, "MailVault", r.Header.Get("X-Source"))
				w.WriteHeader(http.StatusOK)
			},
			request: WebhookRequest{
				Payload: map[string]interface{}{"test": "data"},
				Headers: map[string]string{
					"Authorization": "Bearer token123",
					"X-Source":      "MailVault",
				},
			},
			expectSuccess:  true,
			expectAttempts: 1,
		},
		{
			name: "server error with retries",
			serverHandler: func() func(w http.ResponseWriter, r *http.Request) {
				attempts := 0
				return func(w http.ResponseWriter, r *http.Request) {
					attempts++
					if attempts < 3 {
						w.WriteHeader(http.StatusInternalServerError)
						w.Write([]byte("Internal Server Error"))
					} else {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte("OK"))
					}
				}
			}(),
			request: WebhookRequest{
				Payload: map[string]interface{}{"test": "data"},
			},
			expectSuccess:  true,
			expectAttempts: 3,
		},
		{
			name: "permanent failure after max retries",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Persistent Error"))
			},
			request: WebhookRequest{
				Payload: map[string]interface{}{"test": "data"},
			},
			expectSuccess:  false,
			expectAttempts: 5, // Max retries
			expectError:    true,
		},
		{
			name: "client error no retry",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Bad Request"))
			},
			request: WebhookRequest{
				Payload: map[string]interface{}{"test": "data"},
			},
			expectSuccess:  false,
			expectAttempts: 1, // No retries for 4xx errors
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			// Set webhook URL to test server
			tt.request.URL = server.URL

			// Create client with fast retries for testing
			config := DefaultClientConfig()
			config.BaseDelay = 10 * time.Millisecond
			config.Timeout = 1 * time.Second
			client := NewHTTPClient(config)

			// Override URL validation for testing to allow localhost
			client.isTestMode = true

			// Send webhook
			ctx := context.Background()
			response, err := client.SendWebhook(ctx, tt.request)

			// Verify results
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			require.NotNil(t, response)
			assert.Equal(t, tt.expectSuccess, response.Success)
			assert.Equal(t, tt.expectAttempts, response.Attempt)

			if tt.expectSuccess {
				assert.Equal(t, http.StatusOK, response.StatusCode)
			}
		})
	}
}

func TestHTTPClient_ValidateWebhookURL(t *testing.T) {
	client := NewHTTPClient(DefaultClientConfig())

	tests := []struct {
		name        string
		url         string
		expectError bool
	}{
		{
			name:        "valid https url",
			url:         "https://example.com/webhook",
			expectError: false,
		},
		{
			name:        "valid http url",
			url:         "http://example.com/webhook",
			expectError: false,
		},
		{
			name:        "invalid scheme",
			url:         "ftp://example.com/webhook",
			expectError: true,
		},
		{
			name:        "localhost blocked",
			url:         "http://localhost:8080/webhook",
			expectError: true,
		},
		{
			name:        "127.0.0.1 blocked",
			url:         "http://127.0.0.1:8080/webhook",
			expectError: true,
		},
		{
			name:        "private IP blocked",
			url:         "http://192.168.1.1/webhook",
			expectError: true,
		},
		{
			name:        "invalid url format",
			url:         "not-a-url",
			expectError: true,
		},
		{
			name:        "missing host",
			url:         "http:///webhook",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.validateWebhookURL(tt.url)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHTTPClient_HMACSignatureGeneration(t *testing.T) {
	client := NewHTTPClient(DefaultClientConfig())
	payload := []byte(`{"event_type":"email.received","data":"test"}`)
	secret := "test-secret"
	timestamp := "1640995200" // Fixed timestamp for testing

	signature := client.generateHMACSignature(payload, secret, timestamp)

	// Verify signature format
	assert.True(t, strings.HasPrefix(signature, "t="))
	assert.Contains(t, signature, ",v1=")

	// Extract parts
	parts := strings.Split(signature, ",")
	require.Len(t, parts, 2)

	timestampPart := strings.TrimPrefix(parts[0], "t=")
	signaturePart := strings.TrimPrefix(parts[1], "v1=")

	assert.Equal(t, timestamp, timestampPart)

	// Verify signature
	expectedPayload := timestamp + "." + string(payload)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(expectedPayload))
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	assert.Equal(t, expectedSignature, signaturePart)
}

func TestVerifyWebhookSignature(t *testing.T) {
	payload := []byte(`{"event_type":"email.received"}`)
	secret := "test-secret"

	tests := []struct {
		name      string
		payload   []byte
		signature string
		secret    string
		expect    bool
	}{
		{
			name:      "valid signature",
			payload:   payload,
			signature: generateTestSignature(payload, secret, time.Now().Unix()),
			secret:    secret,
			expect:    true,
		},
		{
			name:      "invalid signature",
			payload:   payload,
			signature: "t=1640995200,v1=invalid-signature",
			secret:    secret,
			expect:    false,
		},
		{
			name:      "wrong secret",
			payload:   payload,
			signature: generateTestSignature(payload, "wrong-secret", time.Now().Unix()),
			secret:    secret,
			expect:    false,
		},
		{
			name:      "expired timestamp",
			payload:   payload,
			signature: generateTestSignature(payload, secret, time.Now().Unix()-400), // 400 seconds ago
			secret:    secret,
			expect:    false,
		},
		{
			name:      "malformed signature",
			payload:   payload,
			signature: "invalid-format",
			secret:    secret,
			expect:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VerifyWebhookSignature(tt.payload, tt.signature, tt.secret)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestHTTPClient_ContextCancellation(t *testing.T) {
	// Create a server that never responds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second) // Will be cancelled before this
	}))
	defer server.Close()

	client := NewHTTPClient(DefaultClientConfig())
	client.isTestMode = true // Allow localhost URLs for testing

	// Create context that cancels quickly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	request := WebhookRequest{
		URL:     server.URL,
		Payload: map[string]interface{}{"test": "data"},
	}

	_, err := client.SendWebhook(ctx, request)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestHTTPClient_RetryDelay(t *testing.T) {
	client := NewHTTPClient(DefaultClientConfig())

	tests := []struct {
		attempt int
		expect  time.Duration
	}{
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 16 * time.Second},
		{6, 30 * time.Second}, // Capped at 30s
		{10, 30 * time.Second}, // Still capped
	}

	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.attempt), func(t *testing.T) {
			delay := client.calculateDelay(tt.attempt)
			assert.Equal(t, tt.expect, delay)
		})
	}
}

func TestHTTPClient_ShouldRetry(t *testing.T) {
	client := NewHTTPClient(DefaultClientConfig())

	tests := []struct {
		statusCode int
		attempt    int
		expect     bool
	}{
		// Should retry
		{0, 1, true},                                    // Network error
		{http.StatusRequestTimeout, 1, true},           // 408
		{http.StatusTooManyRequests, 1, true},          // 429
		{http.StatusInternalServerError, 1, true},      // 500
		{http.StatusBadGateway, 1, true},               // 502
		{http.StatusServiceUnavailable, 1, true},       // 503
		{http.StatusGatewayTimeout, 1, true},           // 504

		// Should not retry
		{http.StatusBadRequest, 1, false},              // 400
		{http.StatusUnauthorized, 1, false},            // 401
		{http.StatusForbidden, 1, false},               // 403
		{http.StatusNotFound, 1, false},                // 404
		{http.StatusOK, 1, false},                      // 200 (success)

		// Max attempts reached
		{http.StatusInternalServerError, 5, false},     // At max attempts
	}

	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.statusCode), func(t *testing.T) {
			result := client.shouldRetry(tt.statusCode, tt.attempt)
			assert.Equal(t, tt.expect, result)
		})
	}
}

// Helper function to generate test signatures
func generateTestSignature(payload []byte, secret string, timestamp int64) string {
	timestampStr := strconv.FormatInt(timestamp, 10)
	signaturePayload := timestampStr + "." + string(payload)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(signaturePayload))
	signature := hex.EncodeToString(h.Sum(nil))
	return "t=" + timestampStr + ",v1=" + signature
}