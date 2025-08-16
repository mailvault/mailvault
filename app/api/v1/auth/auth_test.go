package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mailvault/app/api/v1/mocks"
	"mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
)

// fakeAuthProvider is a lightweight stub for auth.Provider
type fakeAuthProvider struct {
	provider       string
	createUserFunc func(email, password string) (string, error)
	loginFunc      func(email, password string) (string, error)
}

func (f *fakeAuthProvider) Provider() string { return f.provider }
func (f *fakeAuthProvider) CreateUser(_ context.Context, email, password string) (string, error) {
	return f.createUserFunc(email, password)
}
func (f *fakeAuthProvider) Login(_ context.Context, email, password string) (string, error) {
	return f.loginFunc(email, password)
}
func (f *fakeAuthProvider) ValidateToken(_ context.Context, _ string) (*entities.User, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeAuthProvider) GetUserByID(_ context.Context, _ string) (*entities.User, error) {
	return nil, errors.New("not implemented")
}

func TestAuthHandlers_Register_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	userID := uuid.Must(uuid.NewV4())
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)

	authProv := &fakeAuthProvider{
		provider: "stub",
		createUserFunc: func(email, password string) (string, error) {
			assert.Equal(t, "user@example.com", email)
			assert.Equal(t, "password123", password)
			return "auth-prov-id", nil
		},
		loginFunc: func(email, password string) (string, error) { return "", nil },
	}

	userMock := &mocks.UserUseCaseMock{
		GetOrCreateUserByAuthProviderFunc: func(ctx context.Context, provider, providerID, email string) (*entities.User, error) {
			assert.Equal(t, "stub", provider)
			assert.Equal(t, "auth-prov-id", providerID)
			assert.Equal(t, "user@example.com", email)
			return &entities.User{
				ID:           userID,
				Email:        email,
				AuthProvider: provider,
				CreatedAt:    now,
			}, nil
		},
	}

	h := NewAuthHandlers(authProv, userMock, []byte("secret"), "1h")

	body, _ := json.Marshal(RegisterRequest{Email: "user@example.com", Password: "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	h.Register(w, req)

	// Assert
	assert.Equal(t, http.StatusCreated, w.Code)
	var resp AuthResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
	if assert.NotNil(t, resp.User) {
		assert.Equal(t, userID.String(), resp.User.ID)
		assert.Equal(t, "user@example.com", resp.User.Email)
		assert.Equal(t, "stub", resp.User.AuthProvider)
		assert.Equal(t, now.Format("2006-01-02T15:04:05Z07:00"), resp.User.CreatedAt)
	}
}

func TestAuthHandlers_Register_ValidationError(t *testing.T) {
	t.Parallel()

	authProv := &fakeAuthProvider{provider: "stub", createUserFunc: func(email, password string) (string, error) { return "", nil }, loginFunc: func(email, password string) (string, error) { return "", nil }}
	userMock := &mocks.UserUseCaseMock{}
	h := NewAuthHandlers(authProv, userMock, []byte("secret"), "1h")

	body, _ := json.Marshal(RegisterRequest{Email: "bad-email", Password: "123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Register(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthHandlers_Register_AuthProviderError(t *testing.T) {
	t.Parallel()

	authProv := &fakeAuthProvider{
		provider:       "stub",
		createUserFunc: func(email, password string) (string, error) { return "", errors.New("provider err") },
		loginFunc:      func(email, password string) (string, error) { return "", nil },
	}
	userMock := &mocks.UserUseCaseMock{}
	h := NewAuthHandlers(authProv, userMock, []byte("secret"), "1h")

	body, _ := json.Marshal(RegisterRequest{Email: "user@example.com", Password: "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Register(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthHandlers_Register_UserUseCaseError(t *testing.T) {
	t.Parallel()

	authProv := &fakeAuthProvider{provider: "stub", createUserFunc: func(email, password string) (string, error) { return "prov-id", nil }, loginFunc: func(email, password string) (string, error) { return "", nil }}
	userMock := &mocks.UserUseCaseMock{
		GetOrCreateUserByAuthProviderFunc: func(ctx context.Context, provider, providerID, email string) (*entities.User, error) {
			return nil, errors.New("db err")
		},
	}
	h := NewAuthHandlers(authProv, userMock, []byte("secret"), "1h")

	body, _ := json.Marshal(RegisterRequest{Email: "user@example.com", Password: "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Register(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAuthHandlers_Login_Success(t *testing.T) {
	t.Parallel()

	userID := uuid.Must(uuid.NewV4())
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)

	authProv := &fakeAuthProvider{
		provider:       "stub",
		createUserFunc: func(email, password string) (string, error) { return "", nil },
		loginFunc: func(email, password string) (string, error) {
			assert.Equal(t, "user@example.com", email)
			assert.Equal(t, "password123", password)
			return "prov-token", nil
		},
	}
	userMock := &mocks.UserUseCaseMock{
		GetUserByEmailFunc: func(ctx context.Context, email string) (*entities.User, error) {
			return &entities.User{ID: userID, Email: email, AuthProvider: "stub", CreatedAt: now}, nil
		},
	}

	h := NewAuthHandlers(authProv, userMock, []byte("secret"), "1h")

	body, _ := json.Marshal(LoginRequest{Email: "user@example.com", Password: "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Login(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp AuthResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
	if assert.NotNil(t, resp.User) {
		assert.Equal(t, userID.String(), resp.User.ID)
		assert.Equal(t, now.Format("2006-01-02T15:04:05Z07:00"), resp.User.CreatedAt)
	}
}

func TestAuthHandlers_Login_AuthError(t *testing.T) {
	t.Parallel()

	authProv := &fakeAuthProvider{provider: "stub", createUserFunc: func(email, password string) (string, error) { return "", nil }, loginFunc: func(email, password string) (string, error) { return "", errors.New("bad creds") }}
	userMock := &mocks.UserUseCaseMock{}
	h := NewAuthHandlers(authProv, userMock, []byte("secret"), "1h")

	body, _ := json.Marshal(LoginRequest{Email: "user@example.com", Password: "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Login(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthHandlers_Login_UserError(t *testing.T) {
	t.Parallel()

	authProv := &fakeAuthProvider{provider: "stub", createUserFunc: func(email, password string) (string, error) { return "", nil }, loginFunc: func(email, password string) (string, error) { return "", nil }}
	userMock := &mocks.UserUseCaseMock{GetUserByEmailFunc: func(ctx context.Context, email string) (*entities.User, error) { return nil, errors.New("db err") }}
	h := NewAuthHandlers(authProv, userMock, []byte("secret"), "1h")

	body, _ := json.Marshal(LoginRequest{Email: "user@example.com", Password: "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Login(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAuthHandlers_Login_InvalidJSON(t *testing.T) {
	t.Parallel()

	authProv := &fakeAuthProvider{provider: "stub", createUserFunc: func(email, password string) (string, error) { return "", nil }, loginFunc: func(email, password string) (string, error) { return "", nil }}
	userMock := &mocks.UserUseCaseMock{}
	h := NewAuthHandlers(authProv, userMock, []byte("secret"), "1h")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader([]byte("{")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Login(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
