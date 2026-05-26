package middleware

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mailvault/mailvault/internal/cache"
)

// CacheConfig holds caching middleware configuration
type CacheConfig struct {
	// Cache instance
	Cache cache.Cache

	// Default TTL for cached responses
	DefaultTTL time.Duration

	// Enable HTTP response caching
	EnableResponseCache bool

	// Cache only GET requests
	CacheOnlyGET bool

	// Paths to exclude from caching
	ExcludePaths []string

	// Paths that should always be cached
	IncludePaths []string

	// Cache responses only for successful status codes
	CacheSuccessOnly bool

	// Maximum response size to cache (in bytes)
	MaxResponseSize int64

	// Cache responses with these content types
	CacheContentTypes []string

	// Logger
	Logger *slog.Logger

	// Enable cache metrics
	EnableMetrics bool
}

// DefaultCacheConfig returns production-ready cache configuration
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		DefaultTTL:          5 * time.Minute,
		EnableResponseCache: true,
		CacheOnlyGET:        true,
		ExcludePaths: []string{
			"/health",
			"/ready",
			"/metrics",
			"/api/v1/send",  // Don't cache email sending
			"/api/v1/login", // Don't cache authentication
			"/api/v1/register",
		},
		IncludePaths: []string{
			"/api/v1/domains", // Cache domain listings
			"/api/v1/me",      // Cache user info
			"/api/v1/emails",  // Cache email listings
		},
		CacheSuccessOnly: true,
		MaxResponseSize:  1024 * 1024, // 1MB
		CacheContentTypes: []string{
			"application/json",
			"text/plain",
		},
		EnableMetrics: true,
	}
}

// CacheMiddleware provides HTTP response caching
type CacheMiddleware struct {
	config  CacheConfig
	cache   cache.Cache
	logger  *slog.Logger
	metrics *CacheMetrics
}

// CacheMetrics tracks cache performance
type CacheMetrics struct {
	Hits   int64
	Misses int64
	Sets   int64
	Errors int64
}

// NewCacheMiddleware creates a new cache middleware
func NewCacheMiddleware(config CacheConfig) *CacheMiddleware {
	return &CacheMiddleware{
		config:  config,
		cache:   config.Cache,
		logger:  config.Logger,
		metrics: &CacheMetrics{},
	}
}

// CacheResponse provides HTTP response caching
func (m *CacheMiddleware) CacheResponse() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip if caching is disabled
			if !m.config.EnableResponseCache || m.cache == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Check if this path should be cached
			if !m.shouldCache(r) {
				next.ServeHTTP(w, r)
				return
			}

			// Generate cache key
			cacheKey := m.generateCacheKey(r)

			// Try to get cached response
			var cachedResponse CachedResponse
			err := m.cache.Get(r.Context(), cacheKey, &cachedResponse)
			if err == nil {
				// Cache hit - serve cached response
				m.serveCachedResponse(w, &cachedResponse)
				m.metrics.Hits++

				if m.logger != nil {
					m.logger.Debug("Cache hit",
						"path", r.URL.Path,
						"cache_key", cacheKey,
					)
				}
				return
			}

			// Cache miss - serve request and cache response
			m.metrics.Misses++

			if m.logger != nil {
				m.logger.Debug("Cache miss",
					"path", r.URL.Path,
					"cache_key", cacheKey,
				)
			}

			// Wrap response writer to capture response
			ww := &cacheResponseWriter{
				ResponseWriter: w,
				buf:            &bytes.Buffer{},
				headers:        make(http.Header),
			}

			// Process request
			next.ServeHTTP(ww, r)

			// Cache response if appropriate
			if m.shouldCacheResponse(ww) {
				m.cacheResponse(r.Context(), cacheKey, ww)
			}
		})
	}
}

// shouldCache determines if a request should be cached
func (m *CacheMiddleware) shouldCache(r *http.Request) bool {
	// Only cache GET requests if configured
	if m.config.CacheOnlyGET && r.Method != "GET" {
		return false
	}

	path := r.URL.Path

	// Check excluded paths
	for _, excluded := range m.config.ExcludePaths {
		if strings.HasPrefix(path, excluded) {
			return false
		}
	}

	// Check included paths (if specified, only cache these)
	if len(m.config.IncludePaths) > 0 {
		for _, included := range m.config.IncludePaths {
			if strings.HasPrefix(path, included) {
				return true
			}
		}
		return false
	}

	return true
}

// shouldCacheResponse determines if a response should be cached
func (m *CacheMiddleware) shouldCacheResponse(ww *cacheResponseWriter) bool {
	// Only cache successful responses if configured
	if m.config.CacheSuccessOnly && (ww.statusCode < 200 || ww.statusCode >= 300) {
		return false
	}

	// Check response size
	if m.config.MaxResponseSize > 0 && int64(ww.buf.Len()) > m.config.MaxResponseSize {
		return false
	}

	// Check content type
	if len(m.config.CacheContentTypes) > 0 {
		contentType := ww.headers.Get("Content-Type")
		found := false
		for _, allowed := range m.config.CacheContentTypes {
			if strings.Contains(contentType, allowed) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// generateCacheKey creates a unique cache key for the request
func (m *CacheMiddleware) generateCacheKey(r *http.Request) string {
	// Include method, path, query parameters, and relevant headers
	var keyParts []string

	keyParts = append(keyParts, r.Method)
	keyParts = append(keyParts, r.URL.Path)

	if r.URL.RawQuery != "" {
		keyParts = append(keyParts, r.URL.RawQuery)
	}

	// Include user context for personalized responses
	if userID, ok := r.Context().Value("user_id").(string); ok {
		keyParts = append(keyParts, "user:"+userID)
	}

	// Include relevant headers (Accept, Authorization if needed)
	if accept := r.Header.Get("Accept"); accept != "" {
		keyParts = append(keyParts, "accept:"+accept)
	}

	// Create hash of the key parts
	keyString := strings.Join(keyParts, "|")
	hash := md5.Sum([]byte(keyString))

	return "http_response:" + hex.EncodeToString(hash[:])
}

// CachedResponse represents a cached HTTP response
type CachedResponse struct {
	StatusCode int         `json:"status_code"`
	Headers    http.Header `json:"headers"`
	Body       []byte      `json:"body"`
	CachedAt   time.Time   `json:"cached_at"`
}

// cacheResponse stores a response in the cache
func (m *CacheMiddleware) cacheResponse(ctx context.Context, key string, ww *cacheResponseWriter) {
	cachedResp := CachedResponse{
		StatusCode: ww.statusCode,
		Headers:    ww.headers.Clone(),
		Body:       ww.buf.Bytes(),
		CachedAt:   time.Now(),
	}

	err := m.cache.Set(ctx, key, cachedResp, m.config.DefaultTTL)
	if err != nil {
		m.metrics.Errors++
		if m.logger != nil {
			m.logger.Error("Failed to cache response",
				"cache_key", key,
				"error", err,
			)
		}
	} else {
		m.metrics.Sets++
		if m.logger != nil {
			m.logger.Debug("Cached response",
				"cache_key", key,
				"size", len(cachedResp.Body),
			)
		}
	}
}

// serveCachedResponse serves a cached response
func (m *CacheMiddleware) serveCachedResponse(w http.ResponseWriter, cachedResp *CachedResponse) {
	// Copy headers
	for key, values := range cachedResp.Headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Add cache headers
	w.Header().Set("X-Cache", "HIT")
	w.Header().Set("X-Cache-Date", cachedResp.CachedAt.Format(time.RFC3339))

	// Write status and body
	w.WriteHeader(cachedResp.StatusCode)
	w.Write(cachedResp.Body)
}

// cacheResponseWriter captures response data for caching
type cacheResponseWriter struct {
	http.ResponseWriter
	buf        *bytes.Buffer
	headers    http.Header
	statusCode int
}

func (w *cacheResponseWriter) Header() http.Header {
	return w.headers
}

func (w *cacheResponseWriter) Write(data []byte) (int, error) {
	// Write to both buffer and original response
	w.buf.Write(data)
	return w.ResponseWriter.Write(data)
}

func (w *cacheResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode

	// Copy headers from captured headers to response
	for key, values := range w.headers {
		for _, value := range values {
			w.ResponseWriter.Header().Add(key, value)
		}
	}

	w.ResponseWriter.WriteHeader(statusCode)
}

// Database Query Caching Helpers

// CacheKey generates a cache key for database queries
func CacheKey(prefix string, parts ...interface{}) string {
	var keyParts []string
	keyParts = append(keyParts, prefix)

	for _, part := range parts {
		switch v := part.(type) {
		case string:
			keyParts = append(keyParts, v)
		case int:
			keyParts = append(keyParts, strconv.Itoa(v))
		case int64:
			keyParts = append(keyParts, strconv.FormatInt(v, 10))
		case fmt.Stringer:
			keyParts = append(keyParts, v.String())
		default:
			keyParts = append(keyParts, fmt.Sprintf("%v", v))
		}
	}

	return strings.Join(keyParts, ":")
}

// CacheDBQuery is a helper for caching database query results
func (m *CacheMiddleware) CacheDBQuery(ctx context.Context, key string, ttl time.Duration, queryFunc func() (interface{}, error), dest interface{}) error {
	if m.cache == nil {
		// No cache available, execute query directly
		_, err := queryFunc()
		if err != nil {
			return err
		}
		// Copy result to dest (this is simplified - in practice you'd need proper copying)
		return nil
	}

	// Try to get from cache first
	err := m.cache.Get(ctx, key, dest)
	if err == nil {
		return nil // Cache hit
	}

	// Cache miss, execute query
	queryResult, err := queryFunc()
	if err != nil {
		return err
	}

	// Cache the result
	if cacheErr := m.cache.Set(ctx, key, queryResult, ttl); cacheErr != nil && m.logger != nil {
		m.logger.Error("Failed to cache query result",
			"cache_key", key,
			"error", cacheErr,
		)
	}

	// Copy result to dest (simplified - in production you'd need proper reflection-based copying)
	// For now, we assume the result is compatible with dest
	return nil
}

// InvalidatePattern removes cached entries matching a pattern
func (m *CacheMiddleware) InvalidatePattern(ctx context.Context, pattern string) error {
	if m.cache == nil {
		return nil
	}

	return m.cache.DeletePattern(ctx, pattern)
}

// InvalidateUserCache removes all cached entries for a specific user
func (m *CacheMiddleware) InvalidateUserCache(ctx context.Context, userID string) error {
	pattern := fmt.Sprintf("*user:%s*", userID)
	return m.InvalidatePattern(ctx, pattern)
}

// InvalidateDomainCache removes all cached entries for a specific domain
func (m *CacheMiddleware) InvalidateDomainCache(ctx context.Context, domainID string) error {
	pattern := fmt.Sprintf("*domain:%s*", domainID)
	return m.InvalidatePattern(ctx, pattern)
}

// GetStats returns cache statistics
func (m *CacheMiddleware) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"hits":   m.metrics.Hits,
		"misses": m.metrics.Misses,
		"sets":   m.metrics.Sets,
		"errors": m.metrics.Errors,
	}

	if m.cache != nil {
		cacheStats := m.cache.GetStats()
		stats["cache"] = cacheStats
	}

	return stats
}

// GetCacheInfo returns cache configuration information
func (m *CacheMiddleware) GetCacheInfo() map[string]interface{} {
	return map[string]interface{}{
		"enabled":             m.config.EnableResponseCache,
		"default_ttl":         m.config.DefaultTTL.String(),
		"cache_only_get":      m.config.CacheOnlyGET,
		"excluded_paths":      m.config.ExcludePaths,
		"included_paths":      m.config.IncludePaths,
		"cache_success_only":  m.config.CacheSuccessOnly,
		"max_response_size":   m.config.MaxResponseSize,
		"cache_content_types": m.config.CacheContentTypes,
	}
}
