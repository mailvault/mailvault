package middleware

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

// SecurityConfig holds security headers configuration
type SecurityConfig struct {
	// Content Security Policy configuration
	CSP SecurityCSPConfig

	// HSTS (HTTP Strict Transport Security) configuration
	HSTS SecurityHSTSConfig

	// Additional security headers
	XFrameOptions       string
	XContentTypeOptions string
	XSSProtection       string
	ReferrerPolicy      string
	PermissionsPolicy   string

	// CORS configuration
	CORS SecurityCORSConfig

	// Enable security logging
	LogSecurityViolations bool
	Logger                *slog.Logger

	// Development mode (less restrictive policies)
	DevelopmentMode bool
}

// SecurityCSPConfig holds Content Security Policy configuration
type SecurityCSPConfig struct {
	Enabled         bool
	DefaultSrc      []string
	ScriptSrc       []string
	StyleSrc        []string
	ImgSrc          []string
	ConnectSrc      []string
	FontSrc         []string
	ObjectSrc       []string
	MediaSrc        []string
	FrameSrc        []string
	ChildSrc        []string
	WorkerSrc       []string
	FrameAncestors  []string
	FormAction      []string
	BaseURI         []string
	ManifestSrc     []string
	ReportURI       string
	ReportTo        string
	UpgradeInsecure bool
}

// SecurityHSTSConfig holds HSTS configuration
type SecurityHSTSConfig struct {
	Enabled           bool
	MaxAge            int // seconds
	IncludeSubdomains bool
	Preload           bool
}

// SecurityCORSConfig holds CORS configuration
type SecurityCORSConfig struct {
	Enabled          bool
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int // seconds
}

// DefaultSecurityConfig returns production-ready security configuration
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		CSP: SecurityCSPConfig{
			Enabled:         true,
			DefaultSrc:      []string{"'self'"},
			ScriptSrc:       []string{"'self'", "'unsafe-inline'"},
			StyleSrc:        []string{"'self'", "'unsafe-inline'"},
			ImgSrc:          []string{"'self'", "data:", "https:"},
			ConnectSrc:      []string{"'self'"},
			FontSrc:         []string{"'self'"},
			ObjectSrc:       []string{"'none'"},
			MediaSrc:        []string{"'self'"},
			FrameSrc:        []string{"'none'"},
			ChildSrc:        []string{"'none'"},
			WorkerSrc:       []string{"'self'"},
			FrameAncestors:  []string{"'none'"},
			FormAction:      []string{"'self'"},
			BaseURI:         []string{"'self'"},
			ManifestSrc:     []string{"'self'"},
			UpgradeInsecure: true,
		},
		HSTS: SecurityHSTSConfig{
			Enabled:           true,
			MaxAge:            31536000, // 1 year
			IncludeSubdomains: true,
			Preload:           true,
		},
		XFrameOptions:       "DENY",
		XContentTypeOptions: "nosniff",
		XSSProtection:       "1; mode=block",
		ReferrerPolicy:      "strict-origin-when-cross-origin",
		PermissionsPolicy:   "camera=(), microphone=(), geolocation=(), interest-cohort=()",
		CORS: SecurityCORSConfig{
			Enabled:        true,
			AllowedOrigins: []string{"https://mailvault.sh"},
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-API-Key"},
			ExposedHeaders: []string{"X-RateLimit-Limit", "X-RateLimit-Remaining"},
			MaxAge:         86400, // 24 hours
		},
		LogSecurityViolations: true,
		DevelopmentMode:       false,
	}
}

// DevelopmentSecurityConfig returns security configuration suitable for development
func DevelopmentSecurityConfig() SecurityConfig {
	config := DefaultSecurityConfig()

	// More permissive policies for development
	config.DevelopmentMode = true
	config.CSP.ScriptSrc = append(config.CSP.ScriptSrc, "'unsafe-eval'")
	config.CORS.AllowedOrigins = []string{"http://localhost:3000", "http://127.0.0.1:3000"}
	config.HSTS.Enabled = false // Disable HSTS for development (HTTP)

	return config
}

// SecurityMiddleware provides comprehensive security headers
type SecurityMiddleware struct {
	config SecurityConfig
	logger *slog.Logger
}

// NewSecurityMiddleware creates a new security middleware
func NewSecurityMiddleware(config SecurityConfig) *SecurityMiddleware {
	return &SecurityMiddleware{
		config: config,
		logger: config.Logger,
	}
}

// SecurityHeaders applies security headers to all responses
func (m *SecurityMiddleware) SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Apply security headers
			m.applySecurityHeaders(w, r)

			next.ServeHTTP(w, r)
		})
	}
}

// CORS handles Cross-Origin Resource Sharing
func (m *SecurityMiddleware) CORS() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !m.config.CORS.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			if m.isOriginAllowed(origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}

			// Set CORS headers
			w.Header().Set("Access-Control-Allow-Methods", strings.Join(m.config.CORS.AllowedMethods, ", "))
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(m.config.CORS.AllowedHeaders, ", "))

			if len(m.config.CORS.ExposedHeaders) > 0 {
				w.Header().Set("Access-Control-Expose-Headers", strings.Join(m.config.CORS.ExposedHeaders, ", "))
			}

			if m.config.CORS.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if m.config.CORS.MaxAge > 0 {
				w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", m.config.CORS.MaxAge))
			}

			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// applySecurityHeaders sets various security headers
func (m *SecurityMiddleware) applySecurityHeaders(w http.ResponseWriter, r *http.Request) {
	// Content Security Policy
	if m.config.CSP.Enabled {
		csp := m.buildCSPHeader()
		if csp != "" {
			w.Header().Set("Content-Security-Policy", csp)
		}
	}

	// HTTP Strict Transport Security
	if m.config.HSTS.Enabled && r.TLS != nil {
		hsts := m.buildHSTSHeader()
		w.Header().Set("Strict-Transport-Security", hsts)
	}

	// X-Frame-Options
	if m.config.XFrameOptions != "" {
		w.Header().Set("X-Frame-Options", m.config.XFrameOptions)
	}

	// X-Content-Type-Options
	if m.config.XContentTypeOptions != "" {
		w.Header().Set("X-Content-Type-Options", m.config.XContentTypeOptions)
	}

	// X-XSS-Protection
	if m.config.XSSProtection != "" {
		w.Header().Set("X-XSS-Protection", m.config.XSSProtection)
	}

	// Referrer-Policy
	if m.config.ReferrerPolicy != "" {
		w.Header().Set("Referrer-Policy", m.config.ReferrerPolicy)
	}

	// Permissions-Policy
	if m.config.PermissionsPolicy != "" {
		w.Header().Set("Permissions-Policy", m.config.PermissionsPolicy)
	}

	// Additional security headers
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	// Remove server information
	w.Header().Del("Server")
	w.Header().Del("X-Powered-By")
}

// buildCSPHeader constructs the Content Security Policy header value
func (m *SecurityMiddleware) buildCSPHeader() string {
	var directives []string

	if len(m.config.CSP.DefaultSrc) > 0 {
		directives = append(directives, fmt.Sprintf("default-src %s", strings.Join(m.config.CSP.DefaultSrc, " ")))
	}

	if len(m.config.CSP.ScriptSrc) > 0 {
		directives = append(directives, fmt.Sprintf("script-src %s", strings.Join(m.config.CSP.ScriptSrc, " ")))
	}

	if len(m.config.CSP.StyleSrc) > 0 {
		directives = append(directives, fmt.Sprintf("style-src %s", strings.Join(m.config.CSP.StyleSrc, " ")))
	}

	if len(m.config.CSP.ImgSrc) > 0 {
		directives = append(directives, fmt.Sprintf("img-src %s", strings.Join(m.config.CSP.ImgSrc, " ")))
	}

	if len(m.config.CSP.ConnectSrc) > 0 {
		directives = append(directives, fmt.Sprintf("connect-src %s", strings.Join(m.config.CSP.ConnectSrc, " ")))
	}

	if len(m.config.CSP.FontSrc) > 0 {
		directives = append(directives, fmt.Sprintf("font-src %s", strings.Join(m.config.CSP.FontSrc, " ")))
	}

	if len(m.config.CSP.ObjectSrc) > 0 {
		directives = append(directives, fmt.Sprintf("object-src %s", strings.Join(m.config.CSP.ObjectSrc, " ")))
	}

	if len(m.config.CSP.MediaSrc) > 0 {
		directives = append(directives, fmt.Sprintf("media-src %s", strings.Join(m.config.CSP.MediaSrc, " ")))
	}

	if len(m.config.CSP.FrameSrc) > 0 {
		directives = append(directives, fmt.Sprintf("frame-src %s", strings.Join(m.config.CSP.FrameSrc, " ")))
	}

	if len(m.config.CSP.ChildSrc) > 0 {
		directives = append(directives, fmt.Sprintf("child-src %s", strings.Join(m.config.CSP.ChildSrc, " ")))
	}

	if len(m.config.CSP.WorkerSrc) > 0 {
		directives = append(directives, fmt.Sprintf("worker-src %s", strings.Join(m.config.CSP.WorkerSrc, " ")))
	}

	if len(m.config.CSP.FrameAncestors) > 0 {
		directives = append(directives, fmt.Sprintf("frame-ancestors %s", strings.Join(m.config.CSP.FrameAncestors, " ")))
	}

	if len(m.config.CSP.FormAction) > 0 {
		directives = append(directives, fmt.Sprintf("form-action %s", strings.Join(m.config.CSP.FormAction, " ")))
	}

	if len(m.config.CSP.BaseURI) > 0 {
		directives = append(directives, fmt.Sprintf("base-uri %s", strings.Join(m.config.CSP.BaseURI, " ")))
	}

	if len(m.config.CSP.ManifestSrc) > 0 {
		directives = append(directives, fmt.Sprintf("manifest-src %s", strings.Join(m.config.CSP.ManifestSrc, " ")))
	}

	if m.config.CSP.UpgradeInsecure {
		directives = append(directives, "upgrade-insecure-requests")
	}

	if m.config.CSP.ReportURI != "" {
		directives = append(directives, fmt.Sprintf("report-uri %s", m.config.CSP.ReportURI))
	}

	if m.config.CSP.ReportTo != "" {
		directives = append(directives, fmt.Sprintf("report-to %s", m.config.CSP.ReportTo))
	}

	return strings.Join(directives, "; ")
}

// buildHSTSHeader constructs the HSTS header value
func (m *SecurityMiddleware) buildHSTSHeader() string {
	hsts := fmt.Sprintf("max-age=%d", m.config.HSTS.MaxAge)

	if m.config.HSTS.IncludeSubdomains {
		hsts += "; includeSubDomains"
	}

	if m.config.HSTS.Preload {
		hsts += "; preload"
	}

	return hsts
}

// isOriginAllowed checks if an origin is in the allowed list
func (m *SecurityMiddleware) isOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}

	for _, allowed := range m.config.CORS.AllowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}

	return false
}

// CSPViolationReport represents a CSP violation report
type CSPViolationReport struct {
	DocumentURI        string `json:"document-uri"`
	Referrer           string `json:"referrer"`
	ViolatedDirective  string `json:"violated-directive"`
	EffectiveDirective string `json:"effective-directive"`
	OriginalPolicy     string `json:"original-policy"`
	Disposition        string `json:"disposition"`
	BlockedURI         string `json:"blocked-uri"`
	StatusCode         int    `json:"status-code"`
	ScriptSample       string `json:"script-sample"`
}

// CSPReportHandler handles CSP violation reports
func (m *SecurityMiddleware) CSPReportHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var report struct {
			CSPReport CSPViolationReport `json:"csp-report"`
		}

		if err := parseJSON(r, &report); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Log CSP violation
		if m.config.LogSecurityViolations && m.logger != nil {
			m.logger.Warn("CSP Violation detected",
				"document_uri", report.CSPReport.DocumentURI,
				"violated_directive", report.CSPReport.ViolatedDirective,
				"blocked_uri", report.CSPReport.BlockedURI,
				"user_agent", r.Header.Get("User-Agent"),
				"ip", getRealIP(r),
			)
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// parseJSON helper function to parse JSON requests
func parseJSON(r *http.Request, v interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}

// SecureHeaders returns a map of recommended security headers
func (m *SecurityMiddleware) GetSecurityInfo() map[string]interface{} {
	return map[string]interface{}{
		"csp_enabled":     m.config.CSP.Enabled,
		"hsts_enabled":    m.config.HSTS.Enabled,
		"cors_enabled":    m.config.CORS.Enabled,
		"dev_mode":        m.config.DevelopmentMode,
		"allowed_origins": m.config.CORS.AllowedOrigins,
	}
}
