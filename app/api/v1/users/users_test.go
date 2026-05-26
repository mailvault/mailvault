package users

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mailvault/mailvault/app/api/v1/users/mocks"
	"github.com/mailvault/mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
)

func TestUsersHandlers_Me(t *testing.T) {
	t.Parallel()

	userID := uuid.Must(uuid.NewV4())
	now := time.Now().UTC()

	userMock := &mocks.UseCaseMock{
		GetUserByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			assert.Equal(t, userID, id)
			return &entities.User{ID: id, Email: "user@example.com", AuthProvider: "stub", CreatedAt: now}, nil
		},
	}
	h := NewUsersHandlers(userMock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	ctx := context.WithValue(req.Context(), "user_id", userID.String())
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.Me(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp UserResult
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, userID.String(), resp.ID)
}

func TestUsersHandlers_Me_Unauthorized(t *testing.T) {
	t.Parallel()

	userMock := &mocks.UseCaseMock{}
	h := NewUsersHandlers(userMock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	w := httptest.NewRecorder()

	h.Me(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUsersHandlers_Me_InvalidUUID(t *testing.T) {
	t.Parallel()

	userMock := &mocks.UseCaseMock{}
	h := NewUsersHandlers(userMock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	ctx := context.WithValue(req.Context(), "user_id", "not-a-uuid")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.Me(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
