package users

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/mailvault/mailvault/app/api"
	"github.com/mailvault/mailvault/domain/entities"

	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"
	"github.com/gofrs/uuid/v5"
)

//go:generate moq -skip-ensure -stub -pkg mocks -out mocks/usecase.go . UseCase
type UseCase interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (*entities.User, error)
}

// UsersHandlers contains user-related endpoints
type UsersHandlers struct {
	userUseCase UseCase
	validator   *validator.Validate
}

func NewUsersHandlers(userUseCase UseCase) *UsersHandlers {
	return &UsersHandlers{
		userUseCase: userUseCase,
		validator:   validator.New(),
	}
}

// UserResult represents user data in responses
type UserResult struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

// Me returns current user information
// @Summary Get current user
// @Description Get information about the currently authenticated user
// @Tags Authentication
// @Produce json
// @Security BearerAuth
// @Success 200 {object} UserResult "Current user information"
// @Failure 401 {object} models.ErrorResponseBody "Unauthorized"
// @Failure 404 {object} models.ErrorResponseBody "User not found"
// @Router /auth/me [get]
func (h *UsersHandlers) Me(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userID, ok := r.Context().Value("user_id").(string)
	if !ok {
		slog.Error("failed to get user from context", "error", api.ErrUnauthorized)
		api.ErrorResponse(w, r, http.StatusUnauthorized, api.ErrUnauthorized)
		return
	}

	// Parse UUID
	id, err := api.ParseUUID(userID)
	if err != nil {
		slog.Error("failed to parse user id", "error", err, "userID", userID)
		api.ErrorResponse(w, r, http.StatusBadRequest, err)
		return
	}

	// Get user
	user, err := h.userUseCase.GetUserByID(r.Context(), id)
	if err != nil {
		slog.Error("failed to get user from database", "error", err, "userID", id)
		api.ErrorResponse(w, r, http.StatusNotFound, err)
		return
	}

	userResult := &UserResult{
		ID:        user.ID.String(),
		Email:     user.Email,
		CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	render.JSON(w, r, userResult)
}
