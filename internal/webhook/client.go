package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// HTTPClient represents a production-ready webhook HTTP client
type HTTPClient struct {
	httpClient *http.Client
	logger     *slog.Logger
	maxRetries int
	baseDelay  time.Duration
	isTestMode bool // For testing - allows localhost URLs
}

// ClientConfig configures the webhook HTTP client
type ClientConfig struct {
	Timeout    time.Duration
	MaxRetries int
	BaseDelay  time.Duration
	Logger     *slog.Logger
}

// DefaultClientConfig returns sensible defaults for the webhook client
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		Timeout:    30 * time.Second,
		MaxRetries: 5,
		BaseDelay:  1 * time.Second,
		Logger:     slog.Default(),
	}
}

// NewHTTPClient creates a new webhook HTTP client
func NewHTTPClient(config ClientConfig) *HTTPClient {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	httpClient := &http.Client{
		Timeout: config.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Don't follow redirects for webhook calls - they should be direct
			return http.ErrUseLastResponse
		},
	}

	return &HTTPClient{
		httpClient: httpClient,
		logger:     config.Logger,
		maxRetries: config.MaxRetries,
		baseDelay:  config.BaseDelay,
	}
}

// WebhookRequest represents a webhook HTTP request
type WebhookRequest struct {
	URL     string            `json:"url"`
	Payload interface{}       `json:"payload"`
	Secret  string            `json:"secret,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// WebhookResponse represents the response from a webhook call
type WebhookResponse struct {
	StatusCode    int           `json:"status_code"`
	Body          string        `json:"body"`
	Headers       http.Header   `json:"headers"`
	Duration      time.Duration `json:"duration"`
	Attempt       int           `json:"attempt"`
	Success       bool          `json:"success"`
	Error         string        `json:"error,omitempty"`
	LastAttemptAt time.Time     `json:"last_attempt_at"`
}

// SendWebhook sends a webhook with retry logic and security features
func (c *HTTPClient) SendWebhook(ctx context.Context, req WebhookRequest) (*WebhookResponse, error) {
	// Validate webhook URL
	if err := c.validateWebhookURL(req.URL); err != nil {
		return nil, fmt.Errorf("invalid webhook URL: %w", err)
	}

	// Serialize payload
	payloadBytes, err := json.Marshal(req.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	var lastResponse *WebhookResponse
	var lastErr error

	// Retry loop with exponential backoff
	for attempt := 1; attempt <= c.maxRetries; attempt++ {
		response, err := c.attemptWebhook(ctx, req, payloadBytes, attempt)
		lastResponse = response
		lastErr = err

		// Log attempt
		c.logger.Info("webhook attempt",
			slog.String("url", req.URL),
			slog.Int("attempt", attempt),
			slog.Int("status_code", response.StatusCode),
			slog.Duration("duration", response.Duration),
			slog.Bool("success", response.Success))

		// Check if we should retry
		if response.Success || !c.shouldRetry(response.StatusCode, attempt) {
			break
		}

		// Calculate delay for next attempt (exponential backoff)
		if attempt < c.maxRetries {
			delay := c.calculateDelay(attempt)
			c.logger.Warn("webhook failed, retrying",
				slog.String("url", req.URL),
				slog.Int("attempt", attempt),
				slog.Int("status_code", response.StatusCode),
				slog.Duration("retry_delay", delay),
				slog.String("error", response.Error))

			// Wait before retry (with context cancellation)
			select {
			case <-ctx.Done():
				return lastResponse, ctx.Err()
			case <-time.After(delay):
				// Continue to next attempt
			}
		}
	}

	if !lastResponse.Success {
		return lastResponse, fmt.Errorf("webhook failed after %d attempts: %s", c.maxRetries, lastResponse.Error)
	}

	return lastResponse, lastErr
}

// attemptWebhook performs a single webhook attempt
func (c *HTTPClient) attemptWebhook(ctx context.Context, req WebhookRequest, payloadBytes []byte, attempt int) (*WebhookResponse, error) {
	start := time.Now()

	response := &WebhookResponse{
		Attempt:       attempt,
		LastAttemptAt: start,
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, req.URL, bytes.NewReader(payloadBytes))
	if err != nil {
		response.Error = fmt.Sprintf("failed to create request: %v", err)
		response.Duration = time.Since(start)
		return response, err
	}

	// Set content type
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "MailVault-Webhook/1.0")

	// Add custom headers
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Generate and add HMAC signature if secret is provided
	if req.Secret != "" {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		signature := c.generateHMACSignature(payloadBytes, req.Secret, timestamp)

		httpReq.Header.Set("X-MailVault-Signature", signature)
		httpReq.Header.Set("X-MailVault-Timestamp", timestamp)
		httpReq.Header.Set("X-MailVault-Signature-Version", "v1")
	}

	// Perform HTTP request
	httpResp, err := c.httpClient.Do(httpReq)
	response.Duration = time.Since(start)

	if err != nil {
		response.Error = fmt.Sprintf("HTTP request failed: %v", err)
		return response, err
	}
	defer httpResp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		response.Error = fmt.Sprintf("failed to read response body: %v", err)
		response.StatusCode = httpResp.StatusCode
		response.Headers = httpResp.Header
		return response, err
	}

	// Populate response
	response.StatusCode = httpResp.StatusCode
	response.Body = string(bodyBytes)
	response.Headers = httpResp.Header
	response.Success = httpResp.StatusCode >= 200 && httpResp.StatusCode < 300

	if !response.Success {
		response.Error = fmt.Sprintf("HTTP %d: %s", httpResp.StatusCode, string(bodyBytes))
	}

	return response, nil
}

// generateHMACSignature generates HMAC-SHA256 signature for webhook security
func (c *HTTPClient) generateHMACSignature(payload []byte, secret, timestamp string) string {
	// Create signature payload: timestamp.payload
	signaturePayload := timestamp + "." + string(payload)

	// Generate HMAC-SHA256
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(signaturePayload))
	signature := hex.EncodeToString(h.Sum(nil))

	// Return in format expected by webhook receivers
	return fmt.Sprintf("t=%s,v1=%s", timestamp, signature)
}

// validateWebhookURL validates the webhook URL for security
func (c *HTTPClient) validateWebhookURL(webhookURL string) error {
	parsed, err := url.Parse(webhookURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Must be HTTP or HTTPS
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported scheme: %s (only http and https allowed)", parsed.Scheme)
	}

	// Must have a host
	if parsed.Host == "" {
		return fmt.Errorf("missing host in URL")
	}

	// Prevent SSRF attacks - block internal/private networks (except in test mode)
	if !c.isTestMode && c.isInternalOrPrivateURL(parsed) {
		return fmt.Errorf("webhook URL points to internal/private network")
	}

	return nil
}

// isInternalOrPrivateURL checks if URL points to internal/private networks
func (c *HTTPClient) isInternalOrPrivateURL(parsed *url.URL) bool {
	host := strings.ToLower(parsed.Hostname())

	// Check for localhost variants
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}

	// Check for private IP ranges (simplified check)
	if strings.HasPrefix(host, "192.168.") ||
		strings.HasPrefix(host, "10.") ||
		strings.HasPrefix(host, "172.16.") ||
		strings.HasPrefix(host, "172.17.") ||
		strings.HasPrefix(host, "172.18.") ||
		strings.HasPrefix(host, "172.19.") ||
		strings.HasPrefix(host, "172.20.") ||
		strings.HasPrefix(host, "172.21.") ||
		strings.HasPrefix(host, "172.22.") ||
		strings.HasPrefix(host, "172.23.") ||
		strings.HasPrefix(host, "172.24.") ||
		strings.HasPrefix(host, "172.25.") ||
		strings.HasPrefix(host, "172.26.") ||
		strings.HasPrefix(host, "172.27.") ||
		strings.HasPrefix(host, "172.28.") ||
		strings.HasPrefix(host, "172.29.") ||
		strings.HasPrefix(host, "172.30.") ||
		strings.HasPrefix(host, "172.31.") {
		return true
	}

	// Check for link-local addresses
	if strings.HasPrefix(host, "169.254.") {
		return true
	}

	return false
}

// shouldRetry determines if a webhook should be retried based on status code and attempt
func (c *HTTPClient) shouldRetry(statusCode, attempt int) bool {
	if attempt >= c.maxRetries {
		return false
	}

	// Retry on 5xx server errors and specific 4xx errors
	switch statusCode {
	case 0: // Network error
		return true
	case http.StatusRequestTimeout,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// calculateDelay calculates exponential backoff delay
func (c *HTTPClient) calculateDelay(attempt int) time.Duration {
	// Exponential backoff: baseDelay * 2^(attempt-1)
	// attempt 1: 1s, attempt 2: 2s, attempt 3: 4s, attempt 4: 8s, attempt 5: 16s
	multiplier := 1 << (attempt - 1)
	delay := c.baseDelay * time.Duration(multiplier)

	// Cap at 30 seconds
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}

	return delay
}

// VerifyWebhookSignature verifies HMAC signature from incoming webhook (for testing purposes)
func VerifyWebhookSignature(payload []byte, signature, secret string) bool {
	// Parse signature header: t=timestamp,v1=signature
	parts := strings.Split(signature, ",")
	if len(parts) != 2 {
		return false
	}

	var timestamp, sig string
	for _, part := range parts {
		if strings.HasPrefix(part, "t=") {
			timestamp = strings.TrimPrefix(part, "t=")
		} else if strings.HasPrefix(part, "v1=") {
			sig = strings.TrimPrefix(part, "v1=")
		}
	}

	if timestamp == "" || sig == "" {
		return false
	}

	// Verify timestamp is not too old (5 minutes tolerance)
	timestampInt, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}

	now := time.Now().Unix()
	if now-timestampInt > 300 { // 5 minutes
		return false
	}

	// Generate expected signature
	signaturePayload := timestamp + "." + string(payload)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(signaturePayload))
	expectedSig := hex.EncodeToString(h.Sum(nil))

	// Compare signatures
	return hmac.Equal([]byte(sig), []byte(expectedSig))
}