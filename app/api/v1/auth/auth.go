package auth

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/mailvault/mailvault/app/api"
	"github.com/mailvault/mailvault/domain/auth"
	"github.com/mailvault/mailvault/domain/entities"

	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"
	"github.com/gofrs/uuid/v5"
	goxjwt "github.com/guilhermebr/gox/jwt"
)

//go:generate moq -skip-ensure -stub -pkg mocks -out mocks/usecase.go . UseCase
type UseCase interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (*entities.User, error)
	GetUserByEmail(ctx context.Context, email string) (*entities.User, error)
	GetOrCreateUserByAuthProvider(ctx context.Context, provider, providerID, email string) (*entities.User, error)
}

// AuthHandlers contains authentication-related endpoints
type AuthHandlers struct {
	authProvider auth.Provider
	userUseCase  UseCase
	validator    *validator.Validate
	jwt          goxjwt.Service
}

func NewAuthHandlers(authProvider auth.Provider, userUseCase UseCase, jwtSecret []byte, authTokenTTL string) *AuthHandlers {
	return &AuthHandlers{
		authProvider: authProvider,
		userUseCase:  userUseCase,
		validator:    validator.New(),
		jwt:          goxjwt.NewService(string(jwtSecret), "mailvault", authTokenTTL),
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
// @Failure 400 {object} models.ErrorResponseBody "Bad request"
// @Failure 500 {object} models.ErrorResponseBody "Internal server error"
// @Router /auth/register [post]
func (h *AuthHandlers) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Validate request
	if err := h.validator.Struct(req); err != nil {
		slog.Error("validation error", "error", err)
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Create user via auth provider
	createResp, err := h.authProvider.CreateUser(r.Context(), req.Email, req.Password)
	if err != nil {
		slog.Error("failed to create user at auth provider", "error", err)
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Check if email confirmation is required
	if createResp.RequiresConfirm {
		// Email confirmation required - return success without token
		response := map[string]interface{}{
			"message":          "Registration successful. Please check your email to confirm your account.",
			"email":            req.Email,
			"requires_confirm": true,
		}
		render.Status(r, http.StatusCreated)
		render.JSON(w, r, response)
		return
	}

	// Auto-confirmed - proceed with normal flow
	// Create user in our database
	user, err := h.userUseCase.GetOrCreateUserByAuthProvider(
		r.Context(),
		h.authProvider.Provider(),
		createResp.UserID,
		req.Email,
	)
	if err != nil {
		slog.Error("failed to get or create user in our database", "error", err)
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	// Mint our JWT with local user ID
	token, err := h.generateJWT(user.ID.String(), user.Email)
	if err != nil {
		slog.Error("failed to mint jwt", "error", err)
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
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
// @Failure 400 {object} models.ErrorResponseBody "Bad request"
// @Failure 401 {object} models.ErrorResponseBody "Unauthorized"
// @Failure 500 {object} models.ErrorResponseBody "Internal server error"
// @Router /auth/login [post]
func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("failed to decode request", "error", err)
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Authenticate with auth provider (validate credentials)
	accessToken, err := h.authProvider.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		slog.Error("failed to login", "error", err)
		api.ErrorResponse(w, r, http.StatusUnauthorized, err)
		return
	}

	// Resolve the provider's user record so we have the stable AuthProviderID.
	authUser, err := h.authProvider.ValidateToken(r.Context(), accessToken)
	if err != nil {
		slog.Error("failed to validate token after login", "error", err)
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	// Get or create the local user record. Supabase is the source of truth
	// for identity; the local DB just mirrors the user. Auto-creating here
	// lets pre-existing Supabase users sign in after a fresh DB or migration.
	user, err := h.userUseCase.GetOrCreateUserByAuthProvider(
		r.Context(),
		h.authProvider.Provider(),
		authUser.AuthProviderID,
		authUser.Email,
	)
	if err != nil {
		slog.Error("failed to get or create user in our database", "error", err)
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	// Mint our JWT with local user ID
	jwtToken, err := h.generateJWT(user.ID.String(), user.Email)
	if err != nil {
		slog.Error("failed to mint jwt", "error", err)
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
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

// ConfirmEmailRequest represents email confirmation request
type ConfirmEmailRequest struct {
	Email string `json:"email" validate:"required,email"`
	Token string `json:"token" validate:"required"`
}

// ConfirmEmail verifies user email with token/OTP
// @Summary Confirm email
// @Description Confirm user email with verification token or OTP
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body ConfirmEmailRequest true "Confirmation details"
// @Success 200 {object} AuthResponse "Email confirmed successfully"
// @Failure 400 {object} models.ErrorResponseBody "Bad request"
// @Failure 500 {object} models.ErrorResponseBody "Internal server error"
// @Router /auth/confirm [post]
func (h *AuthHandlers) ConfirmEmail(w http.ResponseWriter, r *http.Request) {
	var req ConfirmEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("failed to decode request", "error", err)
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Validate request
	if err := h.validator.Struct(req); err != nil {
		slog.Error("validation error", "error", err)
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Confirm email with auth provider
	authToken, err := h.authProvider.ConfirmEmail(r.Context(), req.Token, req.Email)
	if err != nil {
		slog.Error("failed to confirm email", "error", err)
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Get auth provider user info using the token
	authUser, err := h.authProvider.ValidateToken(r.Context(), authToken)
	if err != nil {
		slog.Error("failed to validate token after confirmation", "error", err)
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	// Create user in our database
	user, err := h.userUseCase.GetOrCreateUserByAuthProvider(
		r.Context(),
		h.authProvider.Provider(),
		authUser.AuthProviderID,
		authUser.Email,
	)
	if err != nil {
		slog.Error("failed to get or create user in our database", "error", err)
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	// Mint our JWT with local user ID
	jwtToken, err := h.generateJWT(user.ID.String(), user.Email)
	if err != nil {
		slog.Error("failed to mint jwt", "error", err)
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
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

// ResendConfirmationRequest represents resend confirmation request
type ResendConfirmationRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// ResendConfirmation resends the confirmation email
// @Summary Resend confirmation email
// @Description Resend confirmation email to user
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body ResendConfirmationRequest true "Email address"
// @Success 200 {object} map[string]string "Confirmation email sent"
// @Failure 400 {object} models.ErrorResponseBody "Bad request"
// @Failure 500 {object} models.ErrorResponseBody "Internal server error"
// @Router /auth/resend-confirmation [post]
func (h *AuthHandlers) ResendConfirmation(w http.ResponseWriter, r *http.Request) {
	var req ResendConfirmationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("failed to decode request", "error", err)
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Validate request
	if err := h.validator.Struct(req); err != nil {
		slog.Error("validation error", "error", err)
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Resend confirmation via auth provider
	if err := h.authProvider.ResendConfirmation(r.Context(), req.Email); err != nil {
		slog.Error("failed to resend confirmation", "error", err)
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	response := map[string]string{
		"message": "Confirmation email has been resent. Please check your inbox.",
	}

	render.JSON(w, r, response)
}

// ConfirmEmailWithTokenRequest represents token-based confirmation request
type ConfirmEmailWithTokenRequest struct {
	AccessToken string `json:"access_token" validate:"required"`
}

// ConfirmEmailWithToken verifies user email using Supabase access token
// @Summary Confirm email with access token
// @Description Confirm user email using Supabase access token from callback URL fragment
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body ConfirmEmailWithTokenRequest true "Access token from Supabase"
// @Success 200 {object} AuthResponse "Email confirmed successfully"
// @Failure 400 {object} models.ErrorResponseBody "Bad request"
// @Failure 500 {object} models.ErrorResponseBody "Internal server error"
// @Router /auth/confirm-token [post]
func (h *AuthHandlers) ConfirmEmailWithToken(w http.ResponseWriter, r *http.Request) {
	var req ConfirmEmailWithTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("failed to decode request", "error", err)
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Validate request
	if err := h.validator.Struct(req); err != nil {
		slog.Error("validation error", "error", err)
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Validate the Supabase access token to get user info
	authUser, err := h.authProvider.ValidateToken(r.Context(), req.AccessToken)
	if err != nil {
		slog.Error("failed to validate Supabase access token", "error", err)
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Create or get user in our database
	user, err := h.userUseCase.GetOrCreateUserByAuthProvider(
		r.Context(),
		h.authProvider.Provider(),
		authUser.AuthProviderID,
		authUser.Email,
	)
	if err != nil {
		slog.Error("failed to get or create user in our database", "error", err)
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
		return
	}

	// Mint our JWT with local user ID
	jwtToken, err := h.generateJWT(user.ID.String(), user.Email)
	if err != nil {
		slog.Error("failed to mint jwt", "error", err)
		api.ErrorResponse(w, r, http.StatusInternalServerError, err)
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

// generateJWT mints an HS256 token via the gox/jwt service for the given local user.
func (h *AuthHandlers) generateJWT(userID, email string) (string, error) {
	return h.jwt.GenerateToken(userID, email, "")
}
