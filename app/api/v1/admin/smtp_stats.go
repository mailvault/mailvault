package admin

import (
	"log/slog"
	"mailvault/internal/errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/gofrs/uuid/v5"
)

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
