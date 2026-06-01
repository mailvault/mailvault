package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	goxjwt "github.com/guilhermebr/gox/jwt"
)

// AuthMiddleware provides JWT authentication middleware
type AuthMiddleware struct {
	jwt goxjwt.Service
}

func NewAuthMiddleware(secret string) (*AuthMiddleware, error) {
	// Validate JWT secret strength
	if err := validateJWTSecret(secret); err != nil {
		return nil, fmt.Errorf("invalid JWT secret: %w", err)
	}

	return &AuthMiddleware{
		// "mailvault" issuer must match the one set by app/api/v1/auth handlers
		// so tokens issued at login parse here at request time.
		jwt: goxjwt.NewService(secret, "mailvault", "24h"),
	}, nil
}

// validateJWTSecret ensures the JWT secret meets security requirements
func validateJWTSecret(secret string) error {
	if secret == "" {
		return fmt.Errorf("JWT secret cannot be empty")
	}

	if len(secret) < 32 {
		return fmt.Errorf("JWT secret must be at least 32 characters long for security")
	}

	// Check for common weak secrets
	weakSecrets := []string{
		"secret",
		"password",
		"123456",
		"jwt-secret",
		"your-secret-key",
		"change-me",
		"development",
		"test",
	}

	lowercaseSecret := strings.ToLower(secret)
	for _, weak := range weakSecrets {
		if strings.Contains(lowercaseSecret, weak) {
			return fmt.Errorf("JWT secret contains common weak patterns, please use a strong random secret")
		}
	}

	return nil
}

// RequireAuth middleware validates JWT token and adds user info to context
func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			m.unauthorizedResponse(w, r, "missing authorization header")
			return
		}

		// Check Bearer prefix
		if !strings.HasPrefix(authHeader, "Bearer ") {
			m.unauthorizedResponse(w, r, "invalid authorization header format")
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			m.unauthorizedResponse(w, r, "missing token")
			return
		}

		// Validate our JWT and extract local user id
		userID, email, accountType, err := m.parseJWT(token)
		if err != nil {
			m.unauthorizedResponse(w, r, "invalid token")
			return
		}
		// Add user info to context
		ctx := context.WithValue(r.Context(), "user_id", userID)
		ctx = context.WithValue(ctx, "user_email", email)
		ctx = context.WithValue(ctx, "account_type", accountType)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalAuth middleware validates JWT token if present but doesn't require it
func (m *AuthMiddleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			next.ServeHTTP(w, r)
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			next.ServeHTTP(w, r)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Validate token if present
		userID, email, accountType, err := m.parseJWT(token)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		// Add user info to context if token is valid
		ctx := context.WithValue(r.Context(), "user_id", userID)
		ctx = context.WithValue(ctx, "user_email", email)
		ctx = context.WithValue(ctx, "account_type", accountType)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func (m *AuthMiddleware) unauthorizedResponse(w http.ResponseWriter, r *http.Request, message string) {
	render.Status(r, http.StatusUnauthorized)
	render.JSON(w, r, ErrorResponse{Error: message})
}

// parseJWT validates our HS256 token and extracts local user id, email, and account type.
func (m *AuthMiddleware) parseJWT(tokenString string) (string, string, string, error) {
	claims, err := m.jwt.ValidateToken(tokenString)
	if err != nil {
		return "", "", "", err
	}
	// The gox/jwt service sets RegisteredClaims.Subject = UserID at mint
	// time; either is fine, prefer Subject for downstream consumers that
	// already key off "sub".
	uid := claims.UserID
	if uid == "" {
		uid = claims.Subject
	}
	if uid == "" {
		return "", "", "", fmt.Errorf("invalid token claims: missing subject")
	}
	return uid, claims.Email, claims.AccountType, nil
}

// RequireAdmin middleware that requires admin authentication
func (m *AuthMiddleware) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			m.unauthorizedResponse(w, r, "missing authorization header")
			return
		}

		// Check Bearer prefix
		if !strings.HasPrefix(authHeader, "Bearer ") {
			m.unauthorizedResponse(w, r, "invalid authorization header format")
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			m.unauthorizedResponse(w, r, "missing token")
			return
		}

		// Validate our JWT and extract local user id
		userID, email, accountType, err := m.parseJWT(token)
		if err != nil {
			m.unauthorizedResponse(w, r, "invalid token")
			return
		}

		// Check if user has admin privileges
		if accountType != "admin" && accountType != "super_admin" {
			m.forbiddenResponse(w, r, "admin access required")
			return
		}

		// Add user info to context
		ctx := context.WithValue(r.Context(), "user_id", userID)
		ctx = context.WithValue(ctx, "user_email", email)
		ctx = context.WithValue(ctx, "account_type", accountType)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *AuthMiddleware) forbiddenResponse(w http.ResponseWriter, r *http.Request, message string) {
	render.Status(r, http.StatusForbidden)
	render.JSON(w, r, ErrorResponse{Error: message})
}

// UserClaims represents the JWT claims for a user (for context retrieval)
type UserClaims struct {
	UserID      string `json:"user_id"`
	Email       string `json:"email"`
	AccountType string `json:"account_type"`
}

// GetUserFromContext retrieves user claims from the request context
func GetUserFromContext(ctx context.Context) (*UserClaims, bool) {
	userID, ok := ctx.Value("user_id").(string)
	if !ok {
		return nil, false
	}

	email, _ := ctx.Value("user_email").(string)
	accountType, _ := ctx.Value("account_type").(string)

	return &UserClaims{
		UserID:      userID,
		Email:       email,
		AccountType: accountType,
	}, true
}
