package admin

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"mailvault/domain/entities"
	"mailvault/internal/errors"
	httpPkg "mailvault/internal/http"
	"mailvault/internal/http/middleware"
	"mailvault/internal/jwt"
	"mailvault/internal/validation"

	"github.com/go-chi/chi/v5"
	"github.com/gofrs/uuid/v5"
)

//go:generate moq -skip-ensure -stub -pkg mocks -out mocks/smtp_stats_uc.go . SMTPStatsUseCase
type SMTPStatsUseCase interface {
	GetOverview(ctx context.Context, filter entities.SMTPStatsFilter) (*entities.SMTPStatsOverview, error)
	GetDomainStats(ctx context.Context, domainID uuid.UUID, filter entities.SMTPStatsFilter, page, pageSize int) ([]entities.SMTPVerificationStat, int64, error)
	GetTimeSeriesData(ctx context.Context, filter entities.SMTPStatsFilter, granularity string) ([]entities.TimeSeriesPoint, error)
	GetDistributions(ctx context.Context, filter entities.SMTPStatsFilter) (map[string]interface{}, error)
	GetTopSenders(ctx context.Context, filter entities.SMTPStatsFilter, limit int) (map[string]interface{}, error)
}

//go:generate moq -skip-ensure -stub -pkg mocks -out mocks/user_uc.go . UserUseCase
type UserUseCase interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (*entities.User, error)
	ListUsers(ctx context.Context, page, pageSize int) ([]entities.User, int64, error)
	UpdateUser(ctx context.Context, user entities.User) error
	DeleteUser(ctx context.Context, userID uuid.UUID) error
	SearchUsers(ctx context.Context, page, pageSize int, search, accountType string) ([]entities.User, int64, error)
}

type AdminHandler struct {
	smtpStatsUC SMTPStatsUseCase
	userUC      UserUseCase
	jwtService  jwt.Service
	authMw      *middleware.AuthMiddleware
	validator   *validation.Validator
	respWriter  *httpPkg.ResponseWriter
	logger      *slog.Logger
}

func NewAdminHandler(
	smtpStatsUC SMTPStatsUseCase,
	userUC UserUseCase,
	jwtService jwt.Service,
	authMw *middleware.AuthMiddleware,
	logger *slog.Logger,
) *AdminHandler {
	return &AdminHandler{
		smtpStatsUC: smtpStatsUC,
		userUC:      userUC,
		jwtService:  jwtService,
		authMw:      authMw,
		validator:   validation.NewValidator(),
		respWriter:  httpPkg.NewResponseWriter(logger),
		logger:      logger,
	}
}

func (h *AdminHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Apply admin authentication to all routes
	r.Use(h.authMw.RequireAdmin)

	// SMTP Statistics endpoints
	r.Route("/smtp", func(r chi.Router) {
		r.Get("/stats", h.GetSMTPStatsOverview)
		r.Get("/stats/domains/{domainId}", h.GetDomainSMTPStats)
		r.Get("/stats/timeline", h.GetSMTPTimelineStats)
		r.Get("/stats/distributions", h.GetSMTPDistributions)
		r.Get("/stats/senders", h.GetTopSenders)
	})

	// User management endpoints (admin access)
	r.Route("/users", func(r chi.Router) {
		r.Get("/", h.ListUsers)
		r.Get("/{id}", h.GetUser)
		r.Put("/{id}", h.UpdateUser)
		r.Delete("/{id}", h.DeleteUser)
	})

	return r
}

// SMTP Statistics handlers

// GetSMTPStatsOverview returns overview statistics for SMTP verification
func (h *AdminHandler) GetSMTPStatsOverview(w http.ResponseWriter, r *http.Request) {
	filter := h.parseStatsFilter(r)

	overview, err := h.smtpStatsUC.GetOverview(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to get SMTP stats overview", slog.String("error", err.Error()))
		h.respWriter.Error(w, r, errors.InternalServer("failed to retrieve statistics"))
		return
	}

	h.respWriter.Success(w, r, overview)
}

// GetDomainSMTPStats returns SMTP statistics for a specific domain
func (h *AdminHandler) GetDomainSMTPStats(w http.ResponseWriter, r *http.Request) {
	domainIDStr := chi.URLParam(r, "domainId")
	domainID, err := uuid.FromString(domainIDStr)
	if err != nil {
		h.respWriter.Error(w, r, errors.BadRequest("invalid domain ID"))
		return
	}

	filter := h.parseStatsFilter(r)
	page, pageSize := h.parsePagination(r)

	stats, total, err := h.smtpStatsUC.GetDomainStats(r.Context(), domainID, filter, page, pageSize)
	if err != nil {
		h.logger.Error("failed to get domain SMTP stats",
			slog.String("domain_id", domainIDStr),
			slog.String("error", err.Error()))
		h.respWriter.Error(w, r, errors.InternalServer("failed to retrieve domain statistics"))
		return
	}

	totalPages := (total + int64(pageSize) - 1) / int64(pageSize)

	response := map[string]interface{}{
		"data":        stats,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": totalPages,
	}

	h.respWriter.Success(w, r, response)
}

// GetSMTPTimelineStats returns time-series data for SMTP verification
func (h *AdminHandler) GetSMTPTimelineStats(w http.ResponseWriter, r *http.Request) {
	filter := h.parseStatsFilter(r)
	granularity := r.URL.Query().Get("granularity")
	if granularity == "" {
		granularity = "day"
	}

	timeSeriesData, err := h.smtpStatsUC.GetTimeSeriesData(r.Context(), filter, granularity)
	if err != nil {
		h.logger.Error("failed to get SMTP timeline stats", slog.String("error", err.Error()))
		h.respWriter.Error(w, r, errors.InternalServer("failed to retrieve timeline statistics"))
		return
	}

	h.respWriter.Success(w, r, timeSeriesData)
}

// GetSMTPDistributions returns distribution data for SMTP verification
func (h *AdminHandler) GetSMTPDistributions(w http.ResponseWriter, r *http.Request) {
	filter := h.parseStatsFilter(r)

	distributions, err := h.smtpStatsUC.GetDistributions(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to get SMTP distributions", slog.String("error", err.Error()))
		h.respWriter.Error(w, r, errors.InternalServer("failed to retrieve distribution statistics"))
		return
	}

	h.respWriter.Success(w, r, distributions)
}

// GetTopSenders returns top sender domains and IPs
func (h *AdminHandler) GetTopSenders(w http.ResponseWriter, r *http.Request) {
	filter := h.parseStatsFilter(r)

	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	senders, err := h.smtpStatsUC.GetTopSenders(r.Context(), filter, limit)
	if err != nil {
		h.logger.Error("failed to get top senders", slog.String("error", err.Error()))
		h.respWriter.Error(w, r, errors.InternalServer("failed to retrieve sender statistics"))
		return
	}

	h.respWriter.Success(w, r, senders)
}

// User management handlers

// ListUsers returns a paginated list of users
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	page, pageSize := h.parsePagination(r)
	search := r.URL.Query().Get("search")
	accountType := r.URL.Query().Get("account_type")

	var users []entities.User
	var total int64
	var err error

	if search != "" || accountType != "" {
		users, total, err = h.userUC.SearchUsers(r.Context(), page, pageSize, search, accountType)
	} else {
		users, total, err = h.userUC.ListUsers(r.Context(), page, pageSize)
	}

	if err != nil {
		h.logger.Error("failed to list users", slog.String("error", err.Error()))
		h.respWriter.Error(w, r, errors.InternalServer("failed to retrieve users"))
		return
	}

	h.respWriter.Paginated(w, r, users, total, page, pageSize)
}

// GetUser returns a specific user by ID
func (h *AdminHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "id")
	userID, err := uuid.FromString(userIDStr)
	if err != nil {
		h.respWriter.Error(w, r, errors.BadRequest("invalid user ID"))
		return
	}

	user, err := h.userUC.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to get user",
			slog.String("user_id", userIDStr),
			slog.String("error", err.Error()))
		h.respWriter.Error(w, r, errors.NotFound("user not found"))
		return
	}

	h.respWriter.Success(w, r, user)
}

// UpdateUser updates a user's information
func (h *AdminHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "id")
	userID, err := uuid.FromString(userIDStr)
	if err != nil {
		h.respWriter.Error(w, r, errors.BadRequest("invalid user ID"))
		return
	}

	var req UpdateUserRequest
	if err := httpPkg.ParseJSON(r, &req); err != nil {
		h.respWriter.Error(w, r, errors.BadRequest("invalid request body"))
		return
	}

	if err := h.validator.Struct(&req); err != nil {
		h.respWriter.Error(w, r, errors.BadRequest(err.Error()))
		return
	}

	// Get existing user
	user, err := h.userUC.GetUserByID(r.Context(), userID)
	if err != nil {
		h.respWriter.Error(w, r, errors.NotFound("user not found"))
		return
	}

	// Update fields
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.AccountType != "" {
		user.AccountType = entities.AccountType(req.AccountType)
	}
	user.UpdatedAt = time.Now()

	err = h.userUC.UpdateUser(r.Context(), *user)
	if err != nil {
		h.logger.Error("failed to update user",
			slog.String("user_id", userIDStr),
			slog.String("error", err.Error()))
		h.respWriter.Error(w, r, errors.InternalServer("failed to update user"))
		return
	}

	h.respWriter.Success(w, r, user)
}

// DeleteUser deletes a user
func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "id")
	userID, err := uuid.FromString(userIDStr)
	if err != nil {
		h.respWriter.Error(w, r, errors.BadRequest("invalid user ID"))
		return
	}

	// Check if trying to delete self
	claims, ok := middleware.GetUserFromContext(r.Context())
	if ok && claims.UserID == userIDStr {
		h.respWriter.Error(w, r, errors.BadRequest("cannot delete your own account"))
		return
	}

	err = h.userUC.DeleteUser(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to delete user",
			slog.String("user_id", userIDStr),
			slog.String("error", err.Error()))
		h.respWriter.Error(w, r, errors.InternalServer("failed to delete user"))
		return
	}

	h.respWriter.NoContent(w, r)
}

// Helper methods

// parseStatsFilter parses filter parameters from request
func (h *AdminHandler) parseStatsFilter(r *http.Request) entities.SMTPStatsFilter {
	filter := entities.SMTPStatsFilter{}

	// Parse domain_id
	if domainIDStr := r.URL.Query().Get("domain_id"); domainIDStr != "" {
		if domainID, err := uuid.FromString(domainIDStr); err == nil {
			filter.DomainID = &domainID
		}
	}

	// Parse email_address_id
	if emailIDStr := r.URL.Query().Get("email_address_id"); emailIDStr != "" {
		if emailID, err := uuid.FromString(emailIDStr); err == nil {
			filter.EmailAddressID = &emailID
		}
	}

	// Parse date range
	if startDateStr := r.URL.Query().Get("start_date"); startDateStr != "" {
		if startDate, err := time.Parse(time.RFC3339, startDateStr); err == nil {
			filter.StartDate = &startDate
		}
	}

	if endDateStr := r.URL.Query().Get("end_date"); endDateStr != "" {
		if endDate, err := time.Parse(time.RFC3339, endDateStr); err == nil {
			filter.EndDate = &endDate
		}
	}

	// Parse other filters
	filter.FinalAction = r.URL.Query().Get("final_action")
	filter.SenderDomain = r.URL.Query().Get("sender_domain")

	// Parse spam score range
	if minScoreStr := r.URL.Query().Get("min_spam_score"); minScoreStr != "" {
		if minScore, err := strconv.ParseFloat(minScoreStr, 64); err == nil {
			filter.MinSpamScore = &minScore
		}
	}

	if maxScoreStr := r.URL.Query().Get("max_spam_score"); maxScoreStr != "" {
		if maxScore, err := strconv.ParseFloat(maxScoreStr, 64); err == nil {
			filter.MaxSpamScore = &maxScore
		}
	}

	return filter
}

// parsePagination parses pagination parameters from request
func (h *AdminHandler) parsePagination(r *http.Request) (page, pageSize int) {
	page = 1
	pageSize = 50

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if parsedPage, err := strconv.Atoi(pageStr); err == nil && parsedPage > 0 {
			page = parsedPage
		}
	}

	if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
		if parsedPageSize, err := strconv.Atoi(pageSizeStr); err == nil && parsedPageSize > 0 && parsedPageSize <= 100 {
			pageSize = parsedPageSize
		}
	}

	return page, pageSize
}

// Request/Response models

type UpdateUserRequest struct {
	Email       string `json:"email,omitempty" validate:"omitempty,email"`
	AccountType string `json:"account_type,omitempty" validate:"omitempty,oneof=user admin super_admin"`
}
