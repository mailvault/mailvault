package middleware

import (
	"context"
	"net/http"
	"strings"

	"mailsafe/internal/config"

	"github.com/go-chi/render"
	"github.com/golang-jwt/jwt/v5"
)

// AuthMiddleware provides JWT authentication middleware
type AuthMiddleware struct {
	secret []byte
}

func NewAuthMiddleware(cfg config.Config) *AuthMiddleware {
	return &AuthMiddleware{secret: []byte(cfg.AuthSecretKey)}
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
		userID, email, err := m.parseJWT(token)
		if err != nil {
			m.unauthorizedResponse(w, r, "invalid token")
			return
		}
		// Add user info to context
		ctx := context.WithValue(r.Context(), "user_id", userID)
		ctx = context.WithValue(ctx, "user_email", email)

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
		userID, email, err := m.parseJWT(token)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		// Add user info to context if token is valid
		ctx := context.WithValue(r.Context(), "user_id", userID)
		ctx = context.WithValue(ctx, "user_email", email)

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

// parseJWT validates our HS256 token and extracts local user id and email
func (m *AuthMiddleware) parseJWT(tokenString string) (string, string, error) {
	type claims struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
		jwt.RegisteredClaims
	}
	parsed, err := jwt.ParseWithClaims(tokenString, &claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return m.secret, nil
	})
	if err != nil || !parsed.Valid {
		return "", "", err
	}
	c, ok := parsed.Claims.(*claims)
	if !ok || c.Sub == "" {
		return "", "", jwt.ErrTokenInvalidClaims
	}
	return c.Sub, c.Email, nil
}
