package pg

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"mailsafe/domain/entities"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUserRepository_Create(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewUserRepository(mockDB)
	ctx := context.Background()

	user := &entities.User{
		ID:             uuid.Must(uuid.NewV4()),
		Email:          "test@example.com",
		AuthProvider:   "supabase",
		AuthProviderID: "test-provider-id",
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	t.Run("successful creation", func(t *testing.T) {
		mockDB.On("Exec", ctx, mock.AnythingOfType("string"),
			user.ID, user.Email, user.AuthProvider, user.AuthProviderID,
			user.CreatedAt, user.UpdatedAt).Return(NewMockCommandTag(1), nil).Once()

		err := repo.Create(ctx, user)

		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("database error", func(t *testing.T) {
		mockDB.ExpectedCalls = nil
		mockDB.On("Exec", ctx, mock.AnythingOfType("string"),
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything,
		).Return(pgconn.CommandTag{}, assert.AnError).Once()

		err := repo.Create(ctx, user)

		assert.Error(t, err)
		mockDB.AssertExpectations(t)
	})
}

func TestUserRepository_GetByID(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewUserRepository(mockDB)
	ctx := context.Background()

	userID := uuid.Must(uuid.NewV4())

	t.Run("successful retrieval", func(t *testing.T) {
		expectedUser := &entities.User{
			ID:             userID,
			Email:          "test@example.com",
			AuthProvider:   "supabase",
			AuthProviderID: "provider-id",
			CreatedAt:      time.Now().UTC(),
			UpdatedAt:      time.Now().UTC(),
		}

		mockRow := &MockRow{
			data: []interface{}{
				expectedUser.ID,
				expectedUser.Email,
				expectedUser.AuthProvider,
				expectedUser.AuthProviderID,
				expectedUser.CreatedAt,
				expectedUser.UpdatedAt,
			},
		}

		mockDB.On("QueryRow", ctx, mock.AnythingOfType("string"), userID).Return(mockRow).Once()

		result, err := repo.GetByID(ctx, userID)

		assert.NoError(t, err)
		assert.Equal(t, expectedUser.ID, result.ID)
		assert.Equal(t, expectedUser.Email, result.Email)
		assert.Equal(t, expectedUser.AuthProvider, result.AuthProvider)
		assert.Equal(t, expectedUser.AuthProviderID, result.AuthProviderID)

		mockDB.AssertExpectations(t)
	})

	t.Run("user not found", func(t *testing.T) {
		mockRow := &MockRow{
			scanResult: pgx.ErrNoRows,
		}

		mockDB.On("QueryRow", ctx, mock.AnythingOfType("string"), userID).Return(mockRow).Once()

		result, err := repo.GetByID(ctx, userID)

		assert.Error(t, err)
		assert.Equal(t, sql.ErrNoRows, err)
		assert.Nil(t, result)

		mockDB.AssertExpectations(t)
	})

	t.Run("database error", func(t *testing.T) {
		mockRow := &MockRow{
			scanResult: assert.AnError,
		}

		mockDB.On("QueryRow", ctx, mock.AnythingOfType("string"), userID).Return(mockRow).Once()

		result, err := repo.GetByID(ctx, userID)

		assert.Error(t, err)
		assert.NotEqual(t, sql.ErrNoRows, err)
		assert.Nil(t, result)

		mockDB.AssertExpectations(t)
	})
}

func TestUserRepository_GetByEmail(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewUserRepository(mockDB)
	ctx := context.Background()

	email := "test@example.com"

	t.Run("successful retrieval", func(t *testing.T) {
		expectedUser := &entities.User{
			ID:             uuid.Must(uuid.NewV4()),
			Email:          email,
			AuthProvider:   "supabase",
			AuthProviderID: "provider-id",
			CreatedAt:      time.Now().UTC(),
			UpdatedAt:      time.Now().UTC(),
		}

		mockRow := &MockRow{
			data: []interface{}{
				expectedUser.ID,
				expectedUser.Email,
				expectedUser.AuthProvider,
				expectedUser.AuthProviderID,
				expectedUser.CreatedAt,
				expectedUser.UpdatedAt,
			},
		}

		mockDB.On("QueryRow", ctx, mock.AnythingOfType("string"), email).Return(mockRow).Once()

		result, err := repo.GetByEmail(ctx, email)

		assert.NoError(t, err)
		assert.Equal(t, expectedUser.Email, result.Email)
		assert.Equal(t, expectedUser.AuthProvider, result.AuthProvider)

		mockDB.AssertExpectations(t)
	})

	t.Run("user not found", func(t *testing.T) {
		mockRow := &MockRow{
			scanResult: pgx.ErrNoRows,
		}

		mockDB.On("QueryRow", ctx, mock.AnythingOfType("string"), email).Return(mockRow).Once()

		result, err := repo.GetByEmail(ctx, email)

		assert.Error(t, err)
		assert.Equal(t, sql.ErrNoRows, err)
		assert.Nil(t, result)

		mockDB.AssertExpectations(t)
	})
}

func TestUserRepository_GetByAuthProvider(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewUserRepository(mockDB)
	ctx := context.Background()

	provider := "supabase"
	providerID := "test-provider-id"

	t.Run("successful retrieval", func(t *testing.T) {
		expectedUser := &entities.User{
			ID:             uuid.Must(uuid.NewV4()),
			Email:          "test@example.com",
			AuthProvider:   provider,
			AuthProviderID: providerID,
			CreatedAt:      time.Now().UTC(),
			UpdatedAt:      time.Now().UTC(),
		}

		mockRow := &MockRow{
			data: []interface{}{
				expectedUser.ID,
				expectedUser.Email,
				expectedUser.AuthProvider,
				expectedUser.AuthProviderID,
				expectedUser.CreatedAt,
				expectedUser.UpdatedAt,
			},
		}

		mockDB.On("QueryRow", ctx, mock.AnythingOfType("string"), provider, providerID).Return(mockRow).Once()

		result, err := repo.GetByAuthProvider(ctx, provider, providerID)

		assert.NoError(t, err)
		assert.Equal(t, expectedUser.AuthProvider, result.AuthProvider)
		assert.Equal(t, expectedUser.AuthProviderID, result.AuthProviderID)

		mockDB.AssertExpectations(t)
	})

	t.Run("user not found", func(t *testing.T) {
		mockRow := &MockRow{
			scanResult: pgx.ErrNoRows,
		}

		mockDB.On("QueryRow", ctx, mock.AnythingOfType("string"), provider, providerID).Return(mockRow).Once()

		result, err := repo.GetByAuthProvider(ctx, provider, providerID)

		assert.Error(t, err)
		assert.Equal(t, sql.ErrNoRows, err)
		assert.Nil(t, result)

		mockDB.AssertExpectations(t)
	})
}

func TestUserRepository_Update(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewUserRepository(mockDB)
	ctx := context.Background()

	user := &entities.User{
		ID:             uuid.Must(uuid.NewV4()),
		Email:          "updated@example.com",
		AuthProvider:   "firebase",
		AuthProviderID: "updated-provider-id",
		UpdatedAt:      time.Now().UTC(),
	}

	t.Run("successful update", func(t *testing.T) {
		mockDB.On("Exec", ctx, mock.AnythingOfType("string"),
			user.ID, user.Email, user.AuthProvider, user.AuthProviderID,
			user.UpdatedAt).Return(NewMockCommandTag(1), nil).Once()

		err := repo.Update(ctx, user)

		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("user not found", func(t *testing.T) {
		mockDB.ExpectedCalls = nil
		mockDB.On("Exec", ctx, mock.AnythingOfType("string"),
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
			mock.Anything,
		).Return(NewMockCommandTag(0), nil).Once()

		err := repo.Update(ctx, user)

		assert.Error(t, err)
		assert.Equal(t, sql.ErrNoRows, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("database error", func(t *testing.T) {
		mockDB.ExpectedCalls = nil
		mockDB.On("Exec", ctx, mock.AnythingOfType("string"),
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
			mock.Anything,
		).Return(pgconn.CommandTag{}, assert.AnError).Once()

		err := repo.Update(ctx, user)

		assert.Error(t, err)
		mockDB.AssertExpectations(t)
	})
}

func TestUserRepository_Delete(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewUserRepository(mockDB)
	ctx := context.Background()

	userID := uuid.Must(uuid.NewV4())

	t.Run("successful deletion", func(t *testing.T) {
		mockDB.On("Exec", ctx, mock.AnythingOfType("string"), userID).Return(NewMockCommandTag(1), nil).Once()

		err := repo.Delete(ctx, userID)

		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("user not found", func(t *testing.T) {
		mockDB.On("Exec", ctx, mock.AnythingOfType("string"), userID).Return(NewMockCommandTag(0), nil).Once()

		err := repo.Delete(ctx, userID)

		assert.Error(t, err)
		assert.Equal(t, sql.ErrNoRows, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("database error", func(t *testing.T) {
		mockDB.On("Exec", ctx, mock.AnythingOfType("string"), userID).Return(pgconn.CommandTag{}, assert.AnError).Once()

		err := repo.Delete(ctx, userID)

		assert.Error(t, err)
		mockDB.AssertExpectations(t)
	})
}
