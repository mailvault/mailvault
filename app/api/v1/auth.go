package v1

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"mailvault/domain/auth"
	"mailvault/domain/entities"

	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"
	"github.com/gofrs/uuid/v5"
	"github.com/golang-jwt/jwt/v5"
)

//go:generate moq -skip-ensure -stub -pkg mocks -out mocks/user_usecase.go . UserUseCase
type UserUseCase interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (*entities.User, error)
	GetUserByEmail(ctx context.Context, email string) (*entities.User, error)
	GetOrCreateUserByAuthProvider(ctx context.Context, provider, providerID, email string) (*entities.User, error)
}

// AuthHandlers contains authentication-related endpoints
type AuthHandlers struct {
	authProvider auth.Provider
	userUseCase  UserUseCase
	validator    *validator.Validate
	jwtSecret    []byte
	jwtTTL       time.Duration
}

func NewAuthHandlers(authProvider auth.Provider, userUseCase UserUseCase, jwtSecret []byte, jwtTTL time.Duration) *AuthHandlers {
	return &AuthHandlers{
		authProvider: authProvider,
		userUseCase:  userUseCase,
		validator:    validator.New(),
		jwtSecret:    jwtSecret,
		jwtTTL:       jwtTTL,
	}
}

// RegisterRequest represents user registration request
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

// LoginRequest represents user login request
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// AuthResponse represents authentication response
type AuthResponse struct {
	Token string      `json:"token"`
	User  *UserResult `json:"user"`
}

// UserResult represents user data in responses
type UserResult struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	AuthProvider string `json:"auth_provider"`
	CreatedAt    string `json:"created_at"`
}

// Register creates a new user account
// @Summary Register a new user
// @Description Create a new user account with email and password
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Registration details"
// @Success 201 {object} AuthResponse "User created successfully"
// @Failure 400 {object} ErrorResponseBody "Bad request"
// @Failure 500 {object} ErrorResponseBody "Internal server error"
// @Router /auth/register [post]
func (h *AuthHandlers) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Validate request
	if err := h.validator.Struct(req); err != nil {
		slog.Error("validation error", "error", err)
		errorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Create user via auth provider
	authProviderID, err := h.authProvider.CreateUser(r.Context(), req.Email, req.Password)
	if err != nil {
		slog.Error("failed to create user at auth provider", "error", err)
		errorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Create user in our database
	user, err := h.userUseCase.GetOrCreateUserByAuthProvider(
		r.Context(),
		h.authProvider.Provider(),
		authProviderID,
		req.Email,
	)
	if err != nil {
		slog.Error("failed to get or create user in our database", "error", err)
		errorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	// Mint our JWT with local user ID
	token, err := h.generateJWT(user.ID.String(), user.Email)
	if err != nil {
		slog.Error("failed to mint jwt", "error", err)
		errorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	response := AuthResponse{
		Token: token,
		User: &UserResult{
			ID:           user.ID.String(),
			Email:        user.Email,
			AuthProvider: user.AuthProvider,
			CreatedAt:    user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		},
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, response)
}

// Login authenticates a user
// @Summary Login user
// @Description Authenticate user with email and password
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login credentials"
// @Success 200 {object} AuthResponse "Login successful"
// @Failure 400 {object} ErrorResponseBody "Bad request"
// @Failure 401 {object} ErrorResponseBody "Unauthorized"
// @Failure 500 {object} ErrorResponseBody "Internal server error"
// @Router /auth/login [post]
func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("failed to decode request", "error", err)
		errorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Authenticate with auth provider (validate credentials)
	_, err := h.authProvider.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		slog.Error("failed to login", "error", err)
		errorResponse(w, r, http.StatusUnauthorized, err)
		return
	}

	// Get user from database
	user, err := h.userUseCase.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		slog.Error("failed to get user from database", "error", err)
		errorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	// Mint our JWT with local user ID
	jwtToken, err := h.generateJWT(user.ID.String(), user.Email)
	if err != nil {
		slog.Error("failed to mint jwt", "error", err)
		errorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	response := AuthResponse{
		Token: jwtToken,
		User: &UserResult{
			ID:           user.ID.String(),
			Email:        user.Email,
			AuthProvider: user.AuthProvider,
			CreatedAt:    user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		},
	}

	render.JSON(w, r, response)
}

// Me returns current user information
// @Summary Get current user
// @Description Get information about the currently authenticated user
// @Tags Authentication
// @Produce json
// @Security BearerAuth
// @Success 200 {object} UserResult "Current user information"
// @Failure 401 {object} ErrorResponseBody "Unauthorized"
// @Failure 404 {object} ErrorResponseBody "User not found"
// @Router /auth/me [get]
func (h *AuthHandlers) Me(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userID, ok := r.Context().Value("user_id").(string)
	if !ok {
		slog.Error("failed to get user from context", "error", ErrUnauthorized)
		errorResponse(w, r, http.StatusUnauthorized, ErrUnauthorized)
		return
	}

	// Parse UUID
	id, err := parseUUID(userID)
	if err != nil {
		slog.Error("failed to parse user id", "error", err, "userID", userID)
		errorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Get user
	user, err := h.userUseCase.GetUserByID(r.Context(), id)
	if err != nil {
		slog.Error("failed to get user from database", "error", err, "userID", id)
		errorResponse(w, r, http.StatusNotFound, err)
		return
	}

	userResult := &UserResult{
		ID:           user.ID.String(),
		Email:        user.Email,
		AuthProvider: user.AuthProvider,
		CreatedAt:    user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	render.JSON(w, r, userResult)
}

// generateJWT creates an HS256 token with local user id
func (h *AuthHandlers) generateJWT(userID, email string) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(h.jwtTTL)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   claims.Subject,
		"email": email,
		"exp":   claims.ExpiresAt.Unix(),
		"iat":   claims.IssuedAt.Unix(),
	})
	return token.SignedString(h.jwtSecret)
}

// parseJWTTTL converts duration string to time.Duration with fallback
func parseJWTTTL(s string) time.Duration {
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	return 24 * time.Hour
}
