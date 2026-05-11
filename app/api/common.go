package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	"github.com/gofrs/uuid/v5"
)

var (
	ErrUnauthorized   = errors.New("unauthorized")
	ErrNotFound       = errors.New("not found")
	ErrBadRequest     = errors.New("bad request")
	ErrForbidden      = errors.New("forbidden")
	ErrConflict       = errors.New("conflict")
	ErrValidation     = errors.New("validation error")
	ErrInternalServer = errors.New("internal server error")
)

// parseUUID parses a string into a UUID
func ParseUUID(s string) (uuid.UUID, error) {
	return uuid.FromString(s)
}

// getUserIDFromContext extracts user ID from request context
func GetUserIDFromContext(r *http.Request) (uuid.UUID, error) {
	userIDStr, ok := r.Context().Value("user_id").(string)
	if !ok {
		return uuid.Nil, ErrUnauthorized
	}
	return ParseUUID(userIDStr)
}

// PaginationParams represents pagination parameters
type PaginationParams struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// getPaginationParams extracts pagination from query params
func GetPaginationParams(r *http.Request) PaginationParams {
	limit := 50 // default
	offset := 0 // default

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := ParseInt(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := ParseInt(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	return PaginationParams{
		Limit:  limit,
		Offset: offset,
	}
}

// parseInt converts string to int
func ParseInt(s string) (int, error) {
	var result int
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, errors.New("invalid integer")
		}
		result = result*10 + int(r-'0')
	}
	return result, nil
}

// PaginatedResponse represents a paginated API response
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Pagination struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
		Total  int `json:"total,omitempty"`
	} `json:"pagination"`
}

// successResponse sends a success response
func SuccessResponse(w http.ResponseWriter, r *http.Request, data interface{}) {
	render.JSON(w, r, data)
}

// createdResponse sends a 201 created response
func CreatedResponse(w http.ResponseWriter, r *http.Request, data interface{}) {
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, data)
}

// noContentResponse sends a 204 no content response
func NoContentResponse(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

type ErrorResponseBody struct {
	Error string `json:"error"`
}

func ErrorResponse(w http.ResponseWriter, r *http.Request, code int, err error) {
	// Sanitize error message to prevent information leakage
	sanitizedError := sanitizeError(err, code)

	render.Status(r, code)
	render.JSON(w, r, ErrorResponseBody{
		Error: sanitizedError,
	})
}

// SafeErrorResponse logs the real error and returns a sanitized error to the client
func SafeErrorResponse(w http.ResponseWriter, r *http.Request, code int, err error, userMsg string) {
	// Log the real error for debugging
	slog.Error("API Error",
		"path", r.URL.Path,
		"method", r.Method,
		"error", err.Error(),
		"status_code", code,
		"user_agent", r.Header.Get("User-Agent"),
		"remote_addr", r.RemoteAddr,
	)

	// Send safe message to user
	render.Status(r, code)
	render.JSON(w, r, ErrorResponseBody{
		Error: userMsg,
	})
}

// sanitizeError prevents sensitive information from leaking in error messages
func sanitizeError(err error, code int) string {
	if err == nil {
		return "unknown error"
	}

	errStr := err.Error()

	// Handle specific error types safely
	if errors.Is(err, sql.ErrNoRows) {
		return "resource not found"
	}

	// Sanitize database errors
	if strings.Contains(errStr, "database") ||
		strings.Contains(errStr, "postgres") ||
		strings.Contains(errStr, "connection") {
		return "service temporarily unavailable"
	}

	// Sanitize file system errors
	if strings.Contains(errStr, "permission denied") ||
		strings.Contains(errStr, "access denied") ||
		strings.Contains(errStr, "file not found") {
		return "access denied"
	}

	// Sanitize validation errors - these are usually safe to show
	if code == http.StatusBadRequest || code == http.StatusUnprocessableEntity {
		// Allow validation errors but strip sensitive paths
		if strings.Contains(errStr, "/") || strings.Contains(errStr, "\\") {
			return "invalid input format"
		}
		return errStr
	}

	// For server errors, return generic message
	if code >= 500 {
		return "internal server error"
	}

	return errStr
}

// ParseJSON parses JSON from request body
func ParseJSON(r *http.Request, v interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return ErrBadRequest
	}
	return nil
}

// ForbiddenResponse sends a 403 forbidden response
func ForbiddenResponse(w http.ResponseWriter, r *http.Request, err error) {
	render.Status(r, http.StatusForbidden)
	render.JSON(w, r, ErrorResponseBody{
		Error: err.Error(),
	})
}

// ConflictResponse sends a 409 conflict response
func ConflictResponse(w http.ResponseWriter, r *http.Request, err error) {
	render.Status(r, http.StatusConflict)
	render.JSON(w, r, ErrorResponseBody{
		Error: err.Error(),
	})
}

// ValidationErrorResponse sends a 422 validation error response
func ValidationErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	render.Status(r, http.StatusUnprocessableEntity)
	render.JSON(w, r, ErrorResponseBody{
		Error: err.Error(),
	})
}
