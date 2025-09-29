package admin

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"mailvault/domain/entities"
	"mailvault/domain/email_provider"
	"mailvault/app/api"
	"mailvault/app/api/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
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

//go:generate moq -skip-ensure -stub -pkg mocks -out mocks/provider_uc.go . ProviderUseCase
type ProviderUseCase interface {
	GetProvider(ctx context.Context, id uuid.UUID) (*entities.EmailProvider, error)
	GetDomainProviders(ctx context.Context, domainID uuid.UUID) ([]*entities.EmailProvider, error)
	GetProviderStats(ctx context.Context, filters *email_provider.ProviderStatsFilters) ([]*entities.EmailProviderStats, error)
	UpdateProvider(ctx context.Context, id uuid.UUID, req email_provider.UpdateProviderRequest) (*entities.EmailProvider, error)
	DeleteProvider(ctx context.Context, id uuid.UUID) error
	ResetProviderHealth(ctx context.Context, providerID uuid.UUID) error
}

type AdminHandler struct {
	smtpStatsUC  SMTPStatsUseCase
	userUC       UserUseCase
	providerUC   ProviderUseCase
	authMw       *middleware.AuthMiddleware
	validator    *validator.Validate
	logger       *slog.Logger
}

func NewAdminHandler(
	smtpStatsUC SMTPStatsUseCase,
	userUC UserUseCase,
	providerUC ProviderUseCase,
	authMw *middleware.AuthMiddleware,
	logger *slog.Logger,
) *AdminHandler {
	return &AdminHandler{
		smtpStatsUC: smtpStatsUC,
		userUC:      userUC,
		providerUC:  providerUC,
		authMw:      authMw,
		validator:   validator.New(),
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

	// Provider management endpoints (admin access)
	r.Route("/providers", func(r chi.Router) {
		r.Get("/", h.ListAllProviders)
		r.Get("/{id}", h.GetProvider)
		r.Put("/{id}", h.UpdateProvider)
		r.Delete("/{id}", h.DeleteProvider)
		r.Post("/{id}/reset-health", h.ResetProviderHealth)
		r.Get("/{id}/stats", h.GetProviderStats)
		r.Get("/stats", h.GetProvidersOverview)
	})

	// Domain-specific provider management
	r.Route("/domains/{domainId}/providers", func(r chi.Router) {
		r.Get("/", h.GetDomainProviders)
	})

	// Analytics and monitoring dashboard
	r.Route("/analytics", func(r chi.Router) {
		r.Get("/overview", h.GetAnalyticsOverview)
		r.Get("/providers", h.GetProviderAnalytics)
		r.Get("/emails", h.GetEmailAnalytics)
	})

	// Real-time monitoring
	r.Route("/monitoring", func(r chi.Router) {
		r.Get("/status", h.GetSystemStatus)
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
		api.ErrorResponse(w, r, http.StatusInternalServerError, api.ErrInternalServer)
		return
	}

	api.SuccessResponse(w, r, overview)
}

// GetDomainSMTPStats returns SMTP statistics for a specific domain
func (h *AdminHandler) GetDomainSMTPStats(w http.ResponseWriter, r *http.Request) {
	domainIDStr := chi.URLParam(r, "domainId")
	domainID, err := uuid.FromString(domainIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, api.ErrBadRequest)
		return
	}

	filter := h.parseStatsFilter(r)
	page, pageSize := h.parsePagination(r)

	stats, total, err := h.smtpStatsUC.GetDomainStats(r.Context(), domainID, filter, page, pageSize)
	if err != nil {
		h.logger.Error("failed to get domain SMTP stats",
			slog.String("domain_id", domainIDStr),
			slog.String("error", err.Error()))
		api.ErrorResponse(w, r, http.StatusInternalServerError, api.ErrInternalServer)
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

	api.SuccessResponse(w, r, response)
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
		api.ErrorResponse(w, r, http.StatusInternalServerError, api.ErrInternalServer)
		return
	}

	api.SuccessResponse(w, r, timeSeriesData)
}

// GetSMTPDistributions returns distribution data for SMTP verification
func (h *AdminHandler) GetSMTPDistributions(w http.ResponseWriter, r *http.Request) {
	filter := h.parseStatsFilter(r)

	distributions, err := h.smtpStatsUC.GetDistributions(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to get SMTP distributions", slog.String("error", err.Error()))
		api.ErrorResponse(w, r, http.StatusInternalServerError, api.ErrInternalServer)
		return
	}

	api.SuccessResponse(w, r, distributions)
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
		api.ErrorResponse(w, r, http.StatusInternalServerError, api.ErrInternalServer)
		return
	}

	api.SuccessResponse(w, r, senders)
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
		api.ErrorResponse(w, r, http.StatusInternalServerError, api.ErrInternalServer)
		return
	}

	totalPages := (total + int64(pageSize) - 1) / int64(pageSize)
		response := map[string]interface{}{
			"data": users,
			"pagination": map[string]interface{}{
				"total":       total,
				"page":        page,
				"page_size":   pageSize,
				"total_pages": totalPages,
			},
		}
		api.SuccessResponse(w, r, response)
}

// GetUser returns a specific user by ID
func (h *AdminHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "id")
	userID, err := uuid.FromString(userIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, api.ErrBadRequest)
		return
	}

	user, err := h.userUC.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to get user",
			slog.String("user_id", userIDStr),
			slog.String("error", err.Error()))
		api.ErrorResponse(w, r, http.StatusNotFound, api.ErrNotFound)
		return
	}

	api.SuccessResponse(w, r, user)
}

// UpdateUser updates a user's information
func (h *AdminHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "id")
	userID, err := uuid.FromString(userIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, api.ErrBadRequest)
		return
	}

	var req UpdateUserRequest
	if err := api.ParseJSON(r, &req); err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, api.ErrBadRequest)
		return
	}

	if err := h.validator.Struct(&req); err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, api.ErrValidation)
		return
	}

	// Get existing user
	user, err := h.userUC.GetUserByID(r.Context(), userID)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusNotFound, api.ErrNotFound)
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
		api.ErrorResponse(w, r, http.StatusInternalServerError, api.ErrInternalServer)
		return
	}

	api.SuccessResponse(w, r, user)
}

// DeleteUser deletes a user
func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "id")
	userID, err := uuid.FromString(userIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, api.ErrBadRequest)
		return
	}

	// Check if trying to delete self
	claims, ok := middleware.GetUserFromContext(r.Context())
	if ok && claims.UserID == userIDStr {
		api.ErrorResponse(w, r, http.StatusBadRequest, api.ErrBadRequest)
		return
	}

	err = h.userUC.DeleteUser(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to delete user",
			slog.String("user_id", userIDStr),
			slog.String("error", err.Error()))
		api.ErrorResponse(w, r, http.StatusInternalServerError, api.ErrInternalServer)
		return
	}

	api.NoContentResponse(w, r)
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

// Provider management handlers

// ListAllProviders returns a paginated list of all providers across all domains
func (h *AdminHandler) ListAllProviders(w http.ResponseWriter, r *http.Request) {
	page, pageSize := h.parsePagination(r)
	filters := h.parseProviderFilters(r)

	stats, err := h.providerUC.GetProviderStats(r.Context(), filters)
	if err != nil {
		h.logger.Error("failed to get provider stats", slog.String("error", err.Error()))
		api.ErrorResponse(w, r, http.StatusInternalServerError, api.ErrInternalServer)
		return
	}

	// Apply pagination
	total := int64(len(stats))
	startIdx := (page - 1) * pageSize
	endIdx := startIdx + pageSize

	if startIdx >= len(stats) {
		stats = []*entities.EmailProviderStats{}
	} else {
		if endIdx > len(stats) {
			endIdx = len(stats)
		}
		stats = stats[startIdx:endIdx]
	}

	totalPages := (total + int64(pageSize) - 1) / int64(pageSize)

	response := map[string]interface{}{
		"data": stats,
		"pagination": map[string]interface{}{
			"total":       total,
			"page":        page,
			"page_size":   pageSize,
			"total_pages": totalPages,
		},
	}

	api.SuccessResponse(w, r, response)
}

// GetProvider returns a specific provider by ID
func (h *AdminHandler) GetProvider(w http.ResponseWriter, r *http.Request) {
	providerIDStr := chi.URLParam(r, "id")
	providerID, err := uuid.FromString(providerIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, api.ErrBadRequest)
		return
	}

	provider, err := h.providerUC.GetProvider(r.Context(), providerID)
	if err != nil {
		h.logger.Error("failed to get provider",
			slog.String("provider_id", providerIDStr),
			slog.String("error", err.Error()))
		api.ErrorResponse(w, r, http.StatusNotFound, api.ErrNotFound)
		return
	}

	api.SuccessResponse(w, r, provider)
}

// UpdateProvider updates a provider's configuration
func (h *AdminHandler) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	providerIDStr := chi.URLParam(r, "id")
	providerID, err := uuid.FromString(providerIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, api.ErrBadRequest)
		return
	}

	var req AdminUpdateProviderRequest
	if err := api.ParseJSON(r, &req); err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, api.ErrBadRequest)
		return
	}

	if err := h.validator.Struct(&req); err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, api.ErrValidation)
		return
	}

	// Convert admin request to domain request
	updateReq := email_provider.UpdateProviderRequest{}
	if req.Name != nil {
		updateReq.Name = req.Name
	}
	if req.Status != nil {
		status := entities.EmailProviderStatus(*req.Status)
		updateReq.Status = &status
	}
	if req.Priority != nil {
		updateReq.Priority = req.Priority
	}
	if req.IsDefault != nil {
		updateReq.IsDefault = req.IsDefault
	}
	if req.IsEnabled != nil {
		updateReq.IsEnabled = req.IsEnabled
	}
	if req.RateLimit != nil {
		updateReq.RateLimit = req.RateLimit
	}
	if req.MaxRetries != nil {
		updateReq.MaxRetries = req.MaxRetries
	}
	if req.RetryDelay != nil {
		updateReq.RetryDelay = req.RetryDelay
	}
	if req.FailoverDelay != nil {
		updateReq.FailoverDelay = req.FailoverDelay
	}

	provider, err := h.providerUC.UpdateProvider(r.Context(), providerID, updateReq)
	if err != nil {
		h.logger.Error("failed to update provider",
			slog.String("provider_id", providerIDStr),
			slog.String("error", err.Error()))
		api.ErrorResponse(w, r, http.StatusInternalServerError, api.ErrInternalServer)
		return
	}

	api.SuccessResponse(w, r, provider)
}

// DeleteProvider deletes a provider
func (h *AdminHandler) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	providerIDStr := chi.URLParam(r, "id")
	providerID, err := uuid.FromString(providerIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, api.ErrBadRequest)
		return
	}

	err = h.providerUC.DeleteProvider(r.Context(), providerID)
	if err != nil {
		h.logger.Error("failed to delete provider",
			slog.String("provider_id", providerIDStr),
			slog.String("error", err.Error()))
		api.ErrorResponse(w, r, http.StatusInternalServerError, api.ErrInternalServer)
		return
	}

	api.NoContentResponse(w, r)
}

// ResetProviderHealth resets a provider's health status
func (h *AdminHandler) ResetProviderHealth(w http.ResponseWriter, r *http.Request) {
	providerIDStr := chi.URLParam(r, "id")
	providerID, err := uuid.FromString(providerIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, api.ErrBadRequest)
		return
	}

	err = h.providerUC.ResetProviderHealth(r.Context(), providerID)
	if err != nil {
		h.logger.Error("failed to reset provider health",
			slog.String("provider_id", providerIDStr),
			slog.String("error", err.Error()))
		api.ErrorResponse(w, r, http.StatusInternalServerError, api.ErrInternalServer)
		return
	}

	api.SuccessResponse(w, r, map[string]string{"message": "Provider health reset successfully"})
}

// GetProviderStats returns statistics for a specific provider
func (h *AdminHandler) GetProviderStats(w http.ResponseWriter, r *http.Request) {
	providerIDStr := chi.URLParam(r, "id")
	providerID, err := uuid.FromString(providerIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, api.ErrBadRequest)
		return
	}

	filters := &email_provider.ProviderStatsFilters{
		ProviderIDs: []uuid.UUID{providerID},
	}

	// Parse time range
	if startDateStr := r.URL.Query().Get("start_date"); startDateStr != "" {
		if startDate, err := time.Parse(time.RFC3339, startDateStr); err == nil {
			filters.Since = &startDate
		}
	}

	if endDateStr := r.URL.Query().Get("end_date"); endDateStr != "" {
		if endDate, err := time.Parse(time.RFC3339, endDateStr); err == nil {
			filters.Until = &endDate
		}
	}

	stats, err := h.providerUC.GetProviderStats(r.Context(), filters)
	if err != nil {
		h.logger.Error("failed to get provider stats",
			slog.String("provider_id", providerIDStr),
			slog.String("error", err.Error()))
		api.ErrorResponse(w, r, http.StatusInternalServerError, api.ErrInternalServer)
		return
	}

	if len(stats) == 0 {
		api.ErrorResponse(w, r, http.StatusNotFound, api.ErrNotFound)
		return
	}

	api.SuccessResponse(w, r, stats[0])
}

// GetProvidersOverview returns overview statistics for all providers
func (h *AdminHandler) GetProvidersOverview(w http.ResponseWriter, r *http.Request) {
	filters := h.parseProviderFilters(r)

	stats, err := h.providerUC.GetProviderStats(r.Context(), filters)
	if err != nil {
		h.logger.Error("failed to get providers overview", slog.String("error", err.Error()))
		api.ErrorResponse(w, r, http.StatusInternalServerError, api.ErrInternalServer)
		return
	}

	// Calculate aggregated statistics
	overview := h.calculateProvidersOverview(stats)

	api.SuccessResponse(w, r, overview)
}

// GetDomainProviders returns all providers for a specific domain
func (h *AdminHandler) GetDomainProviders(w http.ResponseWriter, r *http.Request) {
	domainIDStr := chi.URLParam(r, "domainId")
	domainID, err := uuid.FromString(domainIDStr)
	if err != nil {
		api.ErrorResponse(w, r, http.StatusBadRequest, api.ErrBadRequest)
		return
	}

	providers, err := h.providerUC.GetDomainProviders(r.Context(), domainID)
	if err != nil {
		h.logger.Error("failed to get domain providers",
			slog.String("domain_id", domainIDStr),
			slog.String("error", err.Error()))
		api.ErrorResponse(w, r, http.StatusInternalServerError, api.ErrInternalServer)
		return
	}

	api.SuccessResponse(w, r, providers)
}

// Analytics calculation helper methods

// parseTimeRangeFilters parses time range filters with default duration
func (h *AdminHandler) parseTimeRangeFilters(r *http.Request, defaultDuration time.Duration) *TimeRangeFilter {
	filter := &TimeRangeFilter{}

	// Parse start_date
	if startDateStr := r.URL.Query().Get("start_date"); startDateStr != "" {
		if startDate, err := time.Parse(time.RFC3339, startDateStr); err == nil {
			filter.StartDate = &startDate
		}
	}

	// Parse end_date
	if endDateStr := r.URL.Query().Get("end_date"); endDateStr != "" {
		if endDate, err := time.Parse(time.RFC3339, endDateStr); err == nil {
			filter.EndDate = &endDate
		}
	}

	// Set defaults if not provided
	if filter.EndDate == nil {
		now := time.Now()
		filter.EndDate = &now
	}
	if filter.StartDate == nil {
		startTime := filter.EndDate.Add(-defaultDuration)
		filter.StartDate = &startTime
	}

	return filter
}

// calculateSystemOverview combines SMTP and provider stats into system overview
func (h *AdminHandler) calculateSystemOverview(smtpOverview *entities.SMTPStatsOverview, providerStats []*entities.EmailProviderStats, filters *TimeRangeFilter) map[string]interface{} {
	overview := map[string]interface{}{
		"period": map[string]interface{}{
			"start": filters.StartDate.Format(time.RFC3339),
			"end":   filters.EndDate.Format(time.RFC3339),
		},
		"incoming_emails": map[string]interface{}{
			"total_processed":   smtpOverview.TotalProcessed,
			"accepted":          smtpOverview.AcceptedCount,
			"rejected":          smtpOverview.RejectedCount,
			"quarantined":       smtpOverview.QuarantinedCount,
			"temp_failures":     smtpOverview.TempFailCount,
			"average_spam_score": smtpOverview.AverageSpamScore,
			"action_breakdown":   smtpOverview.ActionBreakdown,
		},
		"outgoing_emails": h.calculateProvidersOverview(providerStats),
		"providers": map[string]interface{}{
			"total_count":     len(providerStats),
			"active_count":    h.countActiveProviders(providerStats),
			"healthy_count":   h.countHealthyProviders(providerStats),
			"unhealthy_count": h.countUnhealthyProviders(providerStats),
		},
		"system_health": h.calculateOverallHealth(smtpOverview, providerStats),
	}

	return overview
}

// calculateProviderAnalytics provides detailed provider performance analysis
func (h *AdminHandler) calculateProviderAnalytics(stats []*entities.EmailProviderStats, filters *TimeRangeFilter) map[string]interface{} {
	analytics := map[string]interface{}{
		"period": map[string]interface{}{
			"start": filters.StartDate.Format(time.RFC3339),
			"end":   filters.EndDate.Format(time.RFC3339),
		},
		"providers": stats,
		"summary": h.calculateProvidersOverview(stats),
		"performance_ranking": h.rankProvidersByPerformance(stats),
		"cost_analysis": h.calculateCostAnalysis(stats),
		"reliability_scores": h.calculateReliabilityScores(stats),
	}

	return analytics
}

// calculateEmailAnalytics combines incoming and outgoing email analytics
func (h *AdminHandler) calculateEmailAnalytics(smtpOverview *entities.SMTPStatsOverview, timeSeries []entities.TimeSeriesPoint, providerStats []*entities.EmailProviderStats, filters *TimeRangeFilter) map[string]interface{} {
	analytics := map[string]interface{}{
		"period": map[string]interface{}{
			"start": filters.StartDate.Format(time.RFC3339),
			"end":   filters.EndDate.Format(time.RFC3339),
		},
		"incoming": map[string]interface{}{
			"overview":     smtpOverview,
			"time_series": timeSeries,
			"trends":      h.calculateIncomingTrends(timeSeries),
		},
		"outgoing": map[string]interface{}{
			"overview": h.calculateProvidersOverview(providerStats),
			"by_provider": providerStats,
			"trends": h.calculateOutgoingTrends(providerStats),
		},
		"flow_analysis": h.calculateEmailFlow(smtpOverview, providerStats),
	}

	return analytics
}

// calculateSystemStatus determines overall system health
func (h *AdminHandler) calculateSystemStatus(stats []*entities.EmailProviderStats, filters *TimeRangeFilter) map[string]interface{} {
	status := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"overall_status": "healthy",
		"providers": map[string]interface{}{
			"total":        len(stats),
			"healthy":      h.countHealthyProviders(stats),
			"unhealthy":    h.countUnhealthyProviders(stats),
			"maintenance": 0,
		},
		"metrics": h.calculateLiveMetrics(stats, filters),
		"alerts": h.generateSystemAlerts(stats, filters),
	}

	// Determine overall status based on provider health
	healthyCount := h.countHealthyProviders(stats)
	totalCount := len(stats)

	if totalCount == 0 {
		status["overall_status"] = "no_providers"
	} else if healthyCount == 0 {
		status["overall_status"] = "critical"
	} else if float64(healthyCount)/float64(totalCount) < 0.5 {
		status["overall_status"] = "degraded"
	} else if healthyCount < totalCount {
		status["overall_status"] = "warning"
	}

	return status
}

// Helper methods for analytics calculations

func (h *AdminHandler) countActiveProviders(stats []*entities.EmailProviderStats) int {
	count := 0
	for _, stat := range stats {
		if stat.TotalSent > 0 {
			count++
		}
	}
	return count
}

func (h *AdminHandler) countHealthyProviders(stats []*entities.EmailProviderStats) int {
	count := 0
	for _, stat := range stats {
		if stat.HealthScore > 80 && stat.SuccessRate > 90 {
			count++
		}
	}
	return count
}

func (h *AdminHandler) countUnhealthyProviders(stats []*entities.EmailProviderStats) int {
	count := 0
	for _, stat := range stats {
		if stat.HealthScore < 50 || stat.SuccessRate < 70 {
			count++
		}
	}
	return count
}

func (h *AdminHandler) calculateOverallHealth(smtpOverview *entities.SMTPStatsOverview, providerStats []*entities.EmailProviderStats) map[string]interface{} {
	health := map[string]interface{}{
		"score": 100.0,
		"status": "healthy",
		"factors": map[string]interface{}{},
	}

	// Calculate health based on provider performance
	if len(providerStats) > 0 {
		totalHealth := 0.0
		for _, stat := range providerStats {
			totalHealth += stat.HealthScore
		}
		avgHealth := totalHealth / float64(len(providerStats))
		health["score"] = avgHealth

		if avgHealth >= 90 {
			health["status"] = "excellent"
		} else if avgHealth >= 80 {
			health["status"] = "good"
		} else if avgHealth >= 60 {
			health["status"] = "fair"
		} else {
			health["status"] = "poor"
		}
	}

	return health
}

func (h *AdminHandler) rankProvidersByPerformance(stats []*entities.EmailProviderStats) []map[string]interface{} {
	ranking := make([]map[string]interface{}, len(stats))
	for i, stat := range stats {
		performanceScore := (stat.SuccessRate*0.4 + stat.HealthScore*0.3 + (100-stat.AvgResponseTime/10)*0.3)
		ranking[i] = map[string]interface{}{
			"provider_id":        stat.ProviderID.String(),
			"provider_name":      stat.ProviderName,
			"provider_type":      string(stat.ProviderType),
			"performance_score": performanceScore,
			"success_rate":      stat.SuccessRate,
			"health_score":      stat.HealthScore,
			"avg_response_time": stat.AvgResponseTime,
		}
	}
	return ranking
}

func (h *AdminHandler) calculateCostAnalysis(stats []*entities.EmailProviderStats) map[string]interface{} {
	analysis := map[string]interface{}{
		"total_estimated_cost": 0.0,
		"cost_per_email":      0.0,
		"by_provider":         []map[string]interface{}{},
	}

	totalCost := 0.0
	totalEmails := int64(0)
	providerCosts := []map[string]interface{}{}

	for _, stat := range stats {
		providerCost := 0.0
		if stat.EstimatedCost != nil {
			providerCost = *stat.EstimatedCost
			totalCost += providerCost
		}
		totalEmails += stat.TotalSent

		providerCosts = append(providerCosts, map[string]interface{}{
			"provider_name": stat.ProviderName,
			"cost":          providerCost,
			"emails_sent":   stat.TotalSent,
			"cost_per_email": func() float64 {
				if stat.TotalSent > 0 && stat.CostPerEmail != nil {
					return *stat.CostPerEmail
				}
				return 0.0
			}(),
		})
	}

	analysis["total_estimated_cost"] = totalCost
	analysis["by_provider"] = providerCosts
	if totalEmails > 0 {
		analysis["cost_per_email"] = totalCost / float64(totalEmails)
	}

	return analysis
}

func (h *AdminHandler) calculateReliabilityScores(stats []*entities.EmailProviderStats) []map[string]interface{} {
	scores := make([]map[string]interface{}, len(stats))
	for i, stat := range stats {
		// Calculate reliability based on success rate, health, and consistency
		reliabilityScore := (stat.SuccessRate*0.5 + stat.HealthScore*0.3 + (100-stat.BounceRate)*0.2)

		scores[i] = map[string]interface{}{
			"provider_name":      stat.ProviderName,
			"reliability_score": reliabilityScore,
			"success_rate":      stat.SuccessRate,
			"health_score":      stat.HealthScore,
			"bounce_rate":       stat.BounceRate,
			"uptime_estimate":   h.calculateUptimeEstimate(stat),
		}
	}
	return scores
}

func (h *AdminHandler) calculateUptimeEstimate(stat *entities.EmailProviderStats) float64 {
	// Simple uptime estimate based on health score and success rate
	return (stat.HealthScore + stat.SuccessRate) / 2
}

func (h *AdminHandler) calculateIncomingTrends(timeSeries []entities.TimeSeriesPoint) map[string]interface{} {
	if len(timeSeries) < 2 {
		return map[string]interface{}{"trend": "insufficient_data"}
	}

	// Calculate simple trend
	first := timeSeries[0]
	last := timeSeries[len(timeSeries)-1]

	return map[string]interface{}{
		"accepted_trend":    h.calculateTrendDirection(first.Accepted, last.Accepted),
		"rejected_trend":    h.calculateTrendDirection(first.Rejected, last.Rejected),
		"quarantined_trend": h.calculateTrendDirection(first.Quarantined, last.Quarantined),
		"volume_change":     float64(last.Accepted-first.Accepted) / float64(first.Accepted+1) * 100,
	}
}

func (h *AdminHandler) calculateOutgoingTrends(stats []*entities.EmailProviderStats) map[string]interface{} {
	// For now, return current state trends - in production you'd want historical data
	totalSent := int64(0)
	totalSuccess := int64(0)
	for _, stat := range stats {
		totalSent += stat.TotalSent
		totalSuccess += stat.TotalDelivered
	}

	successRate := float64(0)
	if totalSent > 0 {
		successRate = float64(totalSuccess) / float64(totalSent) * 100
	}

	return map[string]interface{}{
		"total_sent":    totalSent,
		"success_rate":  successRate,
		"trend_status": "stable", // Would need historical data for real trends
	}
}

func (h *AdminHandler) calculateEmailFlow(smtpOverview *entities.SMTPStatsOverview, providerStats []*entities.EmailProviderStats) map[string]interface{} {
	totalIncoming := smtpOverview.TotalProcessed
	totalOutgoing := int64(0)
	for _, stat := range providerStats {
		totalOutgoing += stat.TotalSent
	}

	return map[string]interface{}{
		"incoming_volume": totalIncoming,
		"outgoing_volume": totalOutgoing,
		"flow_ratio":     func() float64 {
			if totalIncoming > 0 {
				return float64(totalOutgoing) / float64(totalIncoming)
			}
			return 0.0
		}(),
		"acceptance_rate": func() float64 {
			if totalIncoming > 0 {
				return float64(smtpOverview.AcceptedCount) / float64(totalIncoming) * 100
			}
			return 0.0
		}(),
	}
}

func (h *AdminHandler) calculateLiveMetrics(stats []*entities.EmailProviderStats, filters *TimeRangeFilter) map[string]interface{} {
	totalSent := int64(0)
	totalDelivered := int64(0)
	totalFailed := int64(0)
	avgResponseTime := 0.0

	for _, stat := range stats {
		totalSent += stat.TotalSent
		totalDelivered += stat.TotalDelivered
		totalFailed += stat.TotalFailed
		avgResponseTime += stat.AvgResponseTime
	}

	if len(stats) > 0 {
		avgResponseTime = avgResponseTime / float64(len(stats))
	}

	return map[string]interface{}{
		"timestamp":         time.Now().Format(time.RFC3339),
		"emails_sent":       totalSent,
		"emails_delivered": totalDelivered,
		"emails_failed":     totalFailed,
		"success_rate": func() float64 {
			if totalSent > 0 {
				return float64(totalDelivered) / float64(totalSent) * 100
			}
			return 0.0
		}(),
		"avg_response_time": avgResponseTime,
		"active_providers":  h.countActiveProviders(stats),
	}
}

func (h *AdminHandler) generateSystemAlerts(stats []*entities.EmailProviderStats, filters *TimeRangeFilter) []map[string]interface{} {
	alerts := []map[string]interface{}{}

	for _, stat := range stats {
		// Health score alerts
		if stat.HealthScore < 50 {
			alerts = append(alerts, map[string]interface{}{
				"type":         "critical",
				"category":     "provider_health",
				"message":      fmt.Sprintf("Provider %s has low health score: %.1f%%", stat.ProviderName, stat.HealthScore),
				"provider_id":  stat.ProviderID.String(),
				"provider_name": stat.ProviderName,
				"severity":     "high",
				"timestamp":    time.Now().Format(time.RFC3339),
			})
		}

		// Success rate alerts
		if stat.SuccessRate < 70 {
			alerts = append(alerts, map[string]interface{}{
				"type":         "warning",
				"category":     "delivery_rate",
				"message":      fmt.Sprintf("Provider %s has low success rate: %.1f%%", stat.ProviderName, stat.SuccessRate),
				"provider_id":  stat.ProviderID.String(),
				"provider_name": stat.ProviderName,
				"severity":     "medium",
				"timestamp":    time.Now().Format(time.RFC3339),
			})
		}

		// High bounce rate alerts
		if stat.BounceRate > 10 {
			alerts = append(alerts, map[string]interface{}{
				"type":         "warning",
				"category":     "bounce_rate",
				"message":      fmt.Sprintf("Provider %s has high bounce rate: %.1f%%", stat.ProviderName, stat.BounceRate),
				"provider_id":  stat.ProviderID.String(),
				"provider_name": stat.ProviderName,
				"severity":     "medium",
				"timestamp":    time.Now().Format(time.RFC3339),
			})
		}

		// Response time alerts
		if stat.AvgResponseTime > 5000 { // 5 seconds
			alerts = append(alerts, map[string]interface{}{
				"type":         "info",
				"category":     "performance",
				"message":      fmt.Sprintf("Provider %s has slow response time: %.0fms", stat.ProviderName, stat.AvgResponseTime),
				"provider_id":  stat.ProviderID.String(),
				"provider_name": stat.ProviderName,
				"severity":     "low",
				"timestamp":    time.Now().Format(time.RFC3339),
			})
		}
	}

	return alerts
}

func (h *AdminHandler) calculateTrendDirection(oldValue, newValue int64) string {
	if newValue > oldValue {
		return "increasing"
	} else if newValue < oldValue {
		return "decreasing"
	}
	return "stable"
}

// parseProviderFilters parses provider filter parameters from request
func (h *AdminHandler) parseProviderFilters(r *http.Request) *email_provider.ProviderStatsFilters {
	filters := &email_provider.ProviderStatsFilters{}

	// Parse domain_id
	if domainIDStr := r.URL.Query().Get("domain_id"); domainIDStr != "" {
		if domainID, err := uuid.FromString(domainIDStr); err == nil {
			filters.DomainID = &domainID
		}
	}

	// Parse provider_type
	if providerTypeStr := r.URL.Query().Get("provider_type"); providerTypeStr != "" {
		providerType := entities.EmailProviderType(providerTypeStr)
		filters.ProviderType = &providerType
	}

	// Parse active filter
	if activeStr := r.URL.Query().Get("active"); activeStr != "" {
		if active, err := strconv.ParseBool(activeStr); err == nil {
			filters.Active = &active
		}
	}

	// Parse date range
	if startDateStr := r.URL.Query().Get("start_date"); startDateStr != "" {
		if startDate, err := time.Parse(time.RFC3339, startDateStr); err == nil {
			filters.Since = &startDate
		}
	}

	if endDateStr := r.URL.Query().Get("end_date"); endDateStr != "" {
		if endDate, err := time.Parse(time.RFC3339, endDateStr); err == nil {
			filters.Until = &endDate
		}
	}

	return filters
}

// calculateProvidersOverview calculates aggregated statistics from provider stats
func (h *AdminHandler) calculateProvidersOverview(stats []*entities.EmailProviderStats) map[string]interface{} {
	overview := map[string]interface{}{
		"total_providers": len(stats),
		"active_providers": 0,
		"total_emails_sent": int64(0),
		"total_emails_delivered": int64(0),
		"total_emails_failed": int64(0),
		"overall_success_rate": 0.0,
		"provider_types": make(map[string]int),
	}

	if len(stats) == 0 {
		return overview
	}

	var totalSent, totalDelivered, totalFailed int64
	providerTypes := make(map[string]int)
	activeCount := 0

	for _, stat := range stats {
		totalSent += stat.TotalSent
		totalDelivered += stat.TotalDelivered
		totalFailed += stat.TotalFailed

		providerTypeStr := string(stat.ProviderType)
		providerTypes[providerTypeStr]++

		// Consider provider active if it has sent emails recently or has high success rate
		if stat.TotalSent > 0 && stat.SuccessRate > 0.5 {
			activeCount++
		}
	}

	overview["active_providers"] = activeCount
	overview["total_emails_sent"] = totalSent
	overview["total_emails_delivered"] = totalDelivered
	overview["total_emails_failed"] = totalFailed
	overview["provider_types"] = providerTypes

	if totalSent > 0 {
		overview["overall_success_rate"] = float64(totalDelivered) / float64(totalSent) * 100
	}

	return overview
}

// Analytics and monitoring dashboard handlers

// GetAnalyticsOverview returns comprehensive system overview
func (h *AdminHandler) GetAnalyticsOverview(w http.ResponseWriter, r *http.Request) {
	// Parse time range (default to last 30 days)
	filters := h.parseTimeRangeFilters(r, 30*24*time.Hour)

	// Get SMTP stats overview
	smtpOverview, err := h.smtpStatsUC.GetOverview(r.Context(), entities.SMTPStatsFilter{
		StartDate: filters.StartDate,
		EndDate:   filters.EndDate,
	})
	if err != nil {
		h.logger.Error("failed to get SMTP overview", slog.String("error", err.Error()))
		// Continue with empty SMTP data rather than failing entirely
		smtpOverview = &entities.SMTPStatsOverview{}
	}

	// Get provider stats
	providerFilters := &email_provider.ProviderStatsFilters{
		Since: filters.StartDate,
		Until: filters.EndDate,
	}
	providerStats, err := h.providerUC.GetProviderStats(r.Context(), providerFilters)
	if err != nil {
		h.logger.Error("failed to get provider stats", slog.String("error", err.Error()))
		providerStats = []*entities.EmailProviderStats{}
	}

	// Calculate combined analytics
	overview := h.calculateSystemOverview(smtpOverview, providerStats, filters)

	api.SuccessResponse(w, r, overview)
}

// GetProviderAnalytics returns detailed provider performance analytics
func (h *AdminHandler) GetProviderAnalytics(w http.ResponseWriter, r *http.Request) {
	filters := h.parseTimeRangeFilters(r, 7*24*time.Hour) // Default to last 7 days

	providerFilters := &email_provider.ProviderStatsFilters{
		Since: filters.StartDate,
		Until: filters.EndDate,
	}

	// Parse additional filters
	if domainIDStr := r.URL.Query().Get("domain_id"); domainIDStr != "" {
		if domainID, err := uuid.FromString(domainIDStr); err == nil {
			providerFilters.DomainID = &domainID
		}
	}

	if providerTypeStr := r.URL.Query().Get("provider_type"); providerTypeStr != "" {
		providerType := entities.EmailProviderType(providerTypeStr)
		providerFilters.ProviderType = &providerType
	}

	stats, err := h.providerUC.GetProviderStats(r.Context(), providerFilters)
	if err != nil {
		h.logger.Error("failed to get provider analytics", slog.String("error", err.Error()))
		api.ErrorResponse(w, r, http.StatusInternalServerError, api.ErrInternalServer)
		return
	}

	// Calculate detailed analytics
	analytics := h.calculateProviderAnalytics(stats, filters)

	api.SuccessResponse(w, r, analytics)
}

// GetEmailAnalytics returns email flow and delivery analytics
func (h *AdminHandler) GetEmailAnalytics(w http.ResponseWriter, r *http.Request) {
	filters := h.parseTimeRangeFilters(r, 24*time.Hour) // Default to last 24 hours

	// Get SMTP stats for incoming emails
	smtpFilters := entities.SMTPStatsFilter{
		StartDate: filters.StartDate,
		EndDate:   filters.EndDate,
	}

	// Parse domain filter
	if domainIDStr := r.URL.Query().Get("domain_id"); domainIDStr != "" {
		if domainID, err := uuid.FromString(domainIDStr); err == nil {
			smtpFilters.DomainID = &domainID
		}
	}

	// Get SMTP overview for incoming emails
	smtpOverview, err := h.smtpStatsUC.GetOverview(r.Context(), smtpFilters)
	if err != nil {
		h.logger.Error("failed to get email analytics", slog.String("error", err.Error()))
		api.ErrorResponse(w, r, http.StatusInternalServerError, api.ErrInternalServer)
		return
	}

	// Get time series data for trends
	granularity := r.URL.Query().Get("granularity")
	if granularity == "" {
		granularity = "hour"
	}

	timeSeries, err := h.smtpStatsUC.GetTimeSeriesData(r.Context(), smtpFilters, granularity)
	if err != nil {
		h.logger.Error("failed to get time series data", slog.String("error", err.Error()))
		timeSeries = []entities.TimeSeriesPoint{}
	}

	// Get provider stats for outgoing emails
	providerFilters := &email_provider.ProviderStatsFilters{
		Since: filters.StartDate,
		Until: filters.EndDate,
	}
	if smtpFilters.DomainID != nil {
		providerFilters.DomainID = smtpFilters.DomainID
	}

	providerStats, err := h.providerUC.GetProviderStats(r.Context(), providerFilters)
	if err != nil {
		h.logger.Error("failed to get provider stats for email analytics", slog.String("error", err.Error()))
		providerStats = []*entities.EmailProviderStats{}
	}

	// Calculate email analytics
	analytics := h.calculateEmailAnalytics(smtpOverview, timeSeries, providerStats, filters)

	api.SuccessResponse(w, r, analytics)
}

// GetSystemStatus returns overall system health status
func (h *AdminHandler) GetSystemStatus(w http.ResponseWriter, r *http.Request) {
	filters := h.parseTimeRangeFilters(r, 5*time.Minute)

	// Get provider health
	providerFilters := &email_provider.ProviderStatsFilters{
		Since: filters.StartDate,
		Until: filters.EndDate,
	}

	stats, err := h.providerUC.GetProviderStats(r.Context(), providerFilters)
	if err != nil {
		h.logger.Error("failed to get system status", slog.String("error", err.Error()))
		api.ErrorResponse(w, r, http.StatusInternalServerError, api.ErrInternalServer)
		return
	}

	// Calculate overall system status
	status := h.calculateSystemStatus(stats, filters)

	api.SuccessResponse(w, r, status)
}

// Request/Response models

type UpdateUserRequest struct {
	Email       string `json:"email,omitempty" validate:"omitempty,email"`
	AccountType string `json:"account_type,omitempty" validate:"omitempty,oneof=user admin super_admin"`
}

type AdminUpdateProviderRequest struct {
	Name          *string `json:"name,omitempty" validate:"omitempty,min=1,max=100"`
	Status        *string `json:"status,omitempty" validate:"omitempty,oneof=active inactive error"`
	Priority      *int    `json:"priority,omitempty" validate:"omitempty,min=0,max=100"`
	IsDefault     *bool   `json:"is_default,omitempty"`
	IsEnabled     *bool   `json:"is_enabled,omitempty"`
	RateLimit     *int    `json:"rate_limit,omitempty" validate:"omitempty,min=1,max=10000"`
	MaxRetries    *int    `json:"max_retries,omitempty" validate:"omitempty,min=0,max=10"`
	RetryDelay    *int    `json:"retry_delay,omitempty" validate:"omitempty,min=0,max=3600"`
	FailoverDelay *int    `json:"failover_delay,omitempty" validate:"omitempty,min=0,max=7200"`
}

type TimeRangeFilter struct {
	StartDate *time.Time `json:"start_date,omitempty"`
	EndDate   *time.Time `json:"end_date,omitempty"`
}
