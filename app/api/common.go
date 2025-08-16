package api

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
	render.Status(r, code)
	render.JSON(w, r, ErrorResponseBody{
		Error: err.Error(),
	})
}
