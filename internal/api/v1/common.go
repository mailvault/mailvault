package v1

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/gofrs/uuid/v5"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrNotFound     = errors.New("not found")
	ErrBadRequest   = errors.New("bad request")
)

// parseUUID parses a string into a UUID
func parseUUID(s string) (uuid.UUID, error) {
	return uuid.FromString(s)
}

// getUserIDFromContext extracts user ID from request context
func getUserIDFromContext(r *http.Request) (uuid.UUID, error) {
	userIDStr, ok := r.Context().Value("user_id").(string)
	if !ok {
		return uuid.Nil, ErrUnauthorized
	}
	return parseUUID(userIDStr)
}

// PaginationParams represents pagination parameters
type PaginationParams struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// getPaginationParams extracts pagination from query params
func getPaginationParams(r *http.Request) PaginationParams {
	limit := 50 // default
	offset := 0 // default

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := parseInt(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := parseInt(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	return PaginationParams{
		Limit:  limit,
		Offset: offset,
	}
}

// parseInt converts string to int
func parseInt(s string) (int, error) {
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
func successResponse(w http.ResponseWriter, r *http.Request, data interface{}) {
	render.JSON(w, r, data)
}

// createdResponse sends a 201 created response
func createdResponse(w http.ResponseWriter, r *http.Request, data interface{}) {
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, data)
}

// noContentResponse sends a 204 no content response
func noContentResponse(w http.ResponseWriter, r *http.Request) {
	render.Status(r, http.StatusNoContent)
}
