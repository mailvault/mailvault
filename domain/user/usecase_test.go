package user

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"mailsafe/domain/entities"
	"mailsafe/domain/user/mocks"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
)

func TestCreateUser(t *testing.T) {
	ctx := context.Background()
	email := "test@example.com"
	authProvider := "supabase"
	authProviderID := "test-provider-id"

	t.Run("successful creation", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockRepo := &mocks.RepositoryMock{}
		uc := NewUseCase(mockRepo)

		// Setup mocks
		mockRepo.GetByEmailFunc = func(ctx context.Context, email string) (*entities.User, error) {
			return nil, sql.ErrNoRows
		}
		mockRepo.CreateFunc = func(ctx context.Context, user *entities.User) error {
			return nil
		}

		// Execute
		result, err := uc.CreateUser(ctx, CreateUserInput{
			Email:          email,
			AuthProvider:   authProvider,
			AuthProviderID: authProviderID,
		})

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, email, result.Email)
		assert.Equal(t, authProvider, result.AuthProvider)
		assert.Equal(t, authProviderID, result.AuthProviderID)
		assert.NotEmpty(t, result.ID)
		assert.False(t, result.CreatedAt.IsZero())
		assert.False(t, result.UpdatedAt.IsZero())

		// Verify calls
		assert.Len(t, mockRepo.GetByEmailCalls(), 1)
		assert.Len(t, mockRepo.CreateCalls(), 1)
	})

	t.Run("validation errors", func(t *testing.T) {
		testCases := []struct {
			name  string
			input CreateUserInput
			error string
		}{
			{
				name:  "empty email",
				input: CreateUserInput{AuthProvider: authProvider, AuthProviderID: authProviderID},
				error: "email is required",
			},
			{
				name:  "empty auth provider",
				input: CreateUserInput{Email: email, AuthProviderID: authProviderID},
				error: "auth provider is required",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create fresh mocks for each validation test case
				mockRepo := &mocks.RepositoryMock{}
				uc := NewUseCase(mockRepo)

				result, err := uc.CreateUser(ctx, tc.input)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.error)
				assert.Nil(t, result)
			})
		}
	})

	t.Run("user already exists", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockRepo := &mocks.RepositoryMock{}
		uc := NewUseCase(mockRepo)

		existingUser := &entities.User{
			ID:    uuid.Must(uuid.NewV4()),
			Email: email,
		}

		mockRepo.GetByEmailFunc = func(ctx context.Context, email string) (*entities.User, error) {
			return existingUser, nil
		}

		result, err := uc.CreateUser(ctx, CreateUserInput{
			Email:          email,
			AuthProvider:   authProvider,
			AuthProviderID: authProviderID,
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
		assert.Nil(t, result)

		// Verify calls
		assert.Len(t, mockRepo.GetByEmailCalls(), 1)
	})

	t.Run("repository error", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockRepo := &mocks.RepositoryMock{}
		uc := NewUseCase(mockRepo)

		mockRepo.GetByEmailFunc = func(ctx context.Context, email string) (*entities.User, error) {
			return nil, sql.ErrNoRows
		}
		mockRepo.CreateFunc = func(ctx context.Context, user *entities.User) error {
			return assert.AnError
		}

		result, err := uc.CreateUser(ctx, CreateUserInput{
			Email:          email,
			AuthProvider:   authProvider,
			AuthProviderID: authProviderID,
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create user")
		assert.Nil(t, result)

		// Verify calls
		assert.Len(t, mockRepo.GetByEmailCalls(), 1)
		assert.Len(t, mockRepo.CreateCalls(), 1)
	})
}

func TestGetUserByID(t *testing.T) {
	ctx := context.Background()
	userID := uuid.Must(uuid.NewV4())

	t.Run("successful retrieval", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockRepo := &mocks.RepositoryMock{}
		uc := NewUseCase(mockRepo)

		expectedUser := &entities.User{
			ID:    userID,
			Email: "test@example.com",
		}

		mockRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			return expectedUser, nil
		}

		result, err := uc.GetUserByID(ctx, userID)

		assert.NoError(t, err)
		assert.Equal(t, expectedUser, result)

		// Verify calls
		assert.Len(t, mockRepo.GetByIDCalls(), 1)
		assert.Equal(t, userID, mockRepo.GetByIDCalls()[0].ID)
	})

	t.Run("empty user ID", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockRepo := &mocks.RepositoryMock{}
		uc := NewUseCase(mockRepo)

		result, err := uc.GetUserByID(ctx, uuid.Nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user ID is required")
		assert.Nil(t, result)
	})

	t.Run("user not found", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockRepo := &mocks.RepositoryMock{}
		uc := NewUseCase(mockRepo)

		mockRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			return nil, sql.ErrNoRows
		}

		result, err := uc.GetUserByID(ctx, userID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get user")
		assert.Nil(t, result)

		// Verify calls
		assert.Len(t, mockRepo.GetByIDCalls(), 1)
	})
}

func TestGetUserByEmail(t *testing.T) {
	ctx := context.Background()
	email := "test@example.com"

	t.Run("successful retrieval", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockRepo := &mocks.RepositoryMock{}
		uc := NewUseCase(mockRepo)

		expectedUser := &entities.User{
			ID:    uuid.Must(uuid.NewV4()),
			Email: email,
		}

		mockRepo.GetByEmailFunc = func(ctx context.Context, email string) (*entities.User, error) {
			return expectedUser, nil
		}

		result, err := uc.GetUserByEmail(ctx, email)

		assert.NoError(t, err)
		assert.Equal(t, expectedUser, result)

		// Verify calls
		assert.Len(t, mockRepo.GetByEmailCalls(), 1)
		assert.Equal(t, email, mockRepo.GetByEmailCalls()[0].Email)
	})

	t.Run("empty email", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockRepo := &mocks.RepositoryMock{}
		uc := NewUseCase(mockRepo)

		result, err := uc.GetUserByEmail(ctx, "")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "email is required")
		assert.Nil(t, result)
	})
}

func TestGetOrCreateUserByAuthProvider(t *testing.T) {
	ctx := context.Background()
	provider := "supabase"
	providerID := "test-provider-id"
	email := "test@example.com"

	t.Run("user exists by auth provider", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockRepo := &mocks.RepositoryMock{}
		uc := NewUseCase(mockRepo)

		existingUser := &entities.User{
			ID:             uuid.Must(uuid.NewV4()),
			Email:          email,
			AuthProvider:   provider,
			AuthProviderID: providerID,
		}

		mockRepo.GetByAuthProviderFunc = func(ctx context.Context, provider, providerID string) (*entities.User, error) {
			return existingUser, nil
		}

		result, err := uc.GetOrCreateUserByAuthProvider(ctx, provider, providerID, email)

		assert.NoError(t, err)
		assert.Equal(t, existingUser, result)

		// Verify calls
		assert.Len(t, mockRepo.GetByAuthProviderCalls(), 1)
		assert.Equal(t, provider, mockRepo.GetByAuthProviderCalls()[0].Provider)
		assert.Equal(t, providerID, mockRepo.GetByAuthProviderCalls()[0].ProviderID)
	})

	t.Run("user exists by email, update auth provider", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockRepo := &mocks.RepositoryMock{}
		uc := NewUseCase(mockRepo)

		originalUpdateTime := time.Now().Add(-1 * time.Hour)
		existingUser := &entities.User{
			ID:             uuid.Must(uuid.NewV4()),
			Email:          email,
			AuthProvider:   "basic", // Different provider
			AuthProviderID: "old-id",
			UpdatedAt:      originalUpdateTime,
		}

		mockRepo.GetByAuthProviderFunc = func(ctx context.Context, provider, providerID string) (*entities.User, error) {
			return nil, sql.ErrNoRows
		}
		mockRepo.GetByEmailFunc = func(ctx context.Context, email string) (*entities.User, error) {
			return existingUser, nil
		}
		mockRepo.UpdateFunc = func(ctx context.Context, user *entities.User) error {
			return nil
		}

		result, err := uc.GetOrCreateUserByAuthProvider(ctx, provider, providerID, email)

		assert.NoError(t, err)
		assert.Equal(t, provider, result.AuthProvider)
		assert.Equal(t, providerID, result.AuthProviderID)
		assert.True(t, result.UpdatedAt.After(originalUpdateTime))

		// Verify calls
		assert.Len(t, mockRepo.GetByAuthProviderCalls(), 1)
		assert.Len(t, mockRepo.GetByEmailCalls(), 1)
		assert.Len(t, mockRepo.UpdateCalls(), 1)
	})

	t.Run("user exists by email, same auth provider", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockRepo := &mocks.RepositoryMock{}
		uc := NewUseCase(mockRepo)

		existingUser := &entities.User{
			ID:             uuid.Must(uuid.NewV4()),
			Email:          email,
			AuthProvider:   provider,
			AuthProviderID: providerID,
		}

		mockRepo.GetByAuthProviderFunc = func(ctx context.Context, provider, providerID string) (*entities.User, error) {
			return nil, sql.ErrNoRows
		}
		mockRepo.GetByEmailFunc = func(ctx context.Context, email string) (*entities.User, error) {
			return existingUser, nil
		}

		result, err := uc.GetOrCreateUserByAuthProvider(ctx, provider, providerID, email)

		assert.NoError(t, err)
		assert.Equal(t, existingUser, result)

		// Verify calls
		assert.Len(t, mockRepo.GetByAuthProviderCalls(), 1)
		assert.Len(t, mockRepo.GetByEmailCalls(), 1)
	})

	t.Run("create new user", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockRepo := &mocks.RepositoryMock{}
		uc := NewUseCase(mockRepo)

		// Track call count for GetByEmail since it's called multiple times
		getByEmailCallCount := 0
		mockRepo.GetByAuthProviderFunc = func(ctx context.Context, provider, providerID string) (*entities.User, error) {
			return nil, sql.ErrNoRows
		}
		mockRepo.GetByEmailFunc = func(ctx context.Context, email string) (*entities.User, error) {
			getByEmailCallCount++
			return nil, sql.ErrNoRows
		}
		mockRepo.CreateFunc = func(ctx context.Context, user *entities.User) error {
			return nil
		}

		result, err := uc.GetOrCreateUserByAuthProvider(ctx, provider, providerID, email)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, email, result.Email)
		assert.Equal(t, provider, result.AuthProvider)
		assert.Equal(t, providerID, result.AuthProviderID)

		// Verify calls - GetByEmail is called twice (once in GetOrCreateUserByAuthProvider, once in CreateUser)
		assert.Len(t, mockRepo.GetByAuthProviderCalls(), 1)
		assert.Equal(t, 2, getByEmailCallCount)
		assert.Len(t, mockRepo.CreateCalls(), 1)
	})

	t.Run("validation errors", func(t *testing.T) {
		testCases := []struct {
			name       string
			provider   string
			providerID string
			email      string
			error      string
		}{
			{
				name:       "empty provider",
				provider:   "",
				providerID: providerID,
				email:      email,
				error:      "provider, provider ID, and email are required",
			},
			{
				name:       "empty provider ID",
				provider:   provider,
				providerID: "",
				email:      email,
				error:      "provider, provider ID, and email are required",
			},
			{
				name:       "empty email",
				provider:   provider,
				providerID: providerID,
				email:      "",
				error:      "provider, provider ID, and email are required",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create fresh mocks for each validation test case
				mockRepo := &mocks.RepositoryMock{}
				uc := NewUseCase(mockRepo)

				result, err := uc.GetOrCreateUserByAuthProvider(ctx, tc.provider, tc.providerID, tc.email)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.error)
				assert.Nil(t, result)
			})
		}
	})

	t.Run("update auth provider error", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockRepo := &mocks.RepositoryMock{}
		uc := NewUseCase(mockRepo)

		existingUser := &entities.User{
			ID:             uuid.Must(uuid.NewV4()),
			Email:          email,
			AuthProvider:   "basic",
			AuthProviderID: "old-id",
		}

		mockRepo.GetByAuthProviderFunc = func(ctx context.Context, provider, providerID string) (*entities.User, error) {
			return nil, sql.ErrNoRows
		}
		mockRepo.GetByEmailFunc = func(ctx context.Context, email string) (*entities.User, error) {
			return existingUser, nil
		}
		mockRepo.UpdateFunc = func(ctx context.Context, user *entities.User) error {
			return assert.AnError
		}

		result, err := uc.GetOrCreateUserByAuthProvider(ctx, provider, providerID, email)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update user auth provider")
		assert.Nil(t, result)

		// Verify calls
		assert.Len(t, mockRepo.GetByAuthProviderCalls(), 1)
		assert.Len(t, mockRepo.GetByEmailCalls(), 1)
		assert.Len(t, mockRepo.UpdateCalls(), 1)
	})
}
