package domain

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"mailvault/domain/domain/mocks"
	"mailvault/domain/entities"
	userMocks "mailvault/domain/user/mocks"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
)

func TestCreateDomain(t *testing.T) {
	ctx := context.Background()
	userID := uuid.Must(uuid.NewV4())
	domain := "example.com"
	// Valid x25519 public key format
	publicKey := "x25519:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	t.Run("successful creation", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockRepo := &mocks.RepositoryMock{}
		mockUserRepo := &userMocks.RepositoryMock{}
		uc := NewUseCase(mockRepo, mockUserRepo)

		// Setup mocks
		mockUserRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			return &entities.User{
				ID:          userID,
				Email:       "test@example.com",
				AccountType: entities.AccountTypeFreemium,
			}, nil
		}
		mockRepo.GetByUserIDFunc = func(ctx context.Context, userID uuid.UUID) ([]*entities.Domain, error) {
			return []*entities.Domain{}, nil // No existing domains
		}
		mockRepo.GetByDomainFunc = func(ctx context.Context, domain string) (*entities.Domain, error) {
			return nil, sql.ErrNoRows
		}
		mockRepo.CreateFunc = func(ctx context.Context, domain *entities.Domain) error {
			return nil
		}

		// Execute
		result, err := uc.CreateDomain(ctx, CreateDomainInput{
			UserID:    userID,
			Domain:    domain,
			PublicKey: publicKey,
		})

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, userID, result.UserID)
		assert.Equal(t, domain, result.Domain)
		assert.Equal(t, publicKey, result.PublicKey)
		assert.False(t, result.Verified)
		assert.True(t, result.StorageEnabled) // Default is true
		assert.Contains(t, result.APIKey, "pm_")
		assert.NotEmpty(t, result.ID)

		// Verify calls
		assert.Len(t, mockRepo.GetByDomainCalls(), 1)
		assert.Equal(t, domain, mockRepo.GetByDomainCalls()[0].Domain)
		assert.Len(t, mockRepo.CreateCalls(), 1)
	})

	t.Run("freemium domain limit exceeded", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockRepo := &mocks.RepositoryMock{}
		mockUserRepo := &userMocks.RepositoryMock{}
		uc := NewUseCase(mockRepo, mockUserRepo)

		// Setup mocks - freemium user with 1 existing domain (at limit)
		mockUserRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			return &entities.User{
				ID:          userID,
				Email:       "test@example.com",
				AccountType: entities.AccountTypeFreemium,
			}, nil
		}
		mockRepo.GetByUserIDFunc = func(ctx context.Context, userID uuid.UUID) ([]*entities.Domain, error) {
			return []*entities.Domain{
				{ID: uuid.Must(uuid.NewV4()), Domain: "existing.com", UserID: userID},
			}, nil // 1 existing domain (freemium limit is 1)
		}

		// Execute
		result, err := uc.CreateDomain(ctx, CreateDomainInput{
			UserID:    userID,
			Domain:    domain,
			PublicKey: publicKey,
		})

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "domain limit exceeded")
		assert.Contains(t, err.Error(), "freemium accounts can have maximum 1 domain(s)")
	})

	t.Run("validation errors", func(t *testing.T) {
		testCases := []struct {
			name  string
			input CreateDomainInput
			error string
		}{
			{
				name:  "empty user ID",
				input: CreateDomainInput{Domain: domain, PublicKey: publicKey},
				error: "user ID is required",
			},
			{
				name:  "empty domain",
				input: CreateDomainInput{UserID: userID, PublicKey: publicKey},
				error: "domain is required",
			},
			{
				name:  "empty public key",
				input: CreateDomainInput{UserID: userID, Domain: domain},
				error: "public key is required",
			},
			{
				name:  "invalid domain format",
				input: CreateDomainInput{UserID: userID, Domain: "invalid-domain", PublicKey: publicKey},
				error: "invalid domain format",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create fresh mock for each validation test case
				mockRepo := &mocks.RepositoryMock{}
				mockUserRepo := &userMocks.RepositoryMock{}
				uc := NewUseCase(mockRepo, mockUserRepo)

				result, err := uc.CreateDomain(ctx, tc.input)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.error)
				assert.Nil(t, result)
			})
		}
	})

	t.Run("domain already exists", func(t *testing.T) {
		// Create fresh mock for each test case
		mockRepo := &mocks.RepositoryMock{}
		mockUserRepo := &userMocks.RepositoryMock{}
		uc := NewUseCase(mockRepo, mockUserRepo)

		existingDomain := &entities.Domain{
			ID:     uuid.Must(uuid.NewV4()),
			Domain: domain,
		}

		// Setup user mock
		mockUserRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			return &entities.User{
				ID:          userID,
				Email:       "test@example.com",
				AccountType: entities.AccountTypeFreemium,
			}, nil
		}
		mockRepo.GetByUserIDFunc = func(ctx context.Context, userID uuid.UUID) ([]*entities.Domain, error) {
			return []*entities.Domain{}, nil // No existing domains for limit check
		}
		mockRepo.GetByDomainFunc = func(ctx context.Context, domain string) (*entities.Domain, error) {
			return existingDomain, nil
		}

		result, err := uc.CreateDomain(ctx, CreateDomainInput{
			UserID:    userID,
			Domain:    domain,
			PublicKey: publicKey,
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
		assert.Nil(t, result)

		// Verify calls
		assert.Len(t, mockRepo.GetByDomainCalls(), 1)
	})

	t.Run("with webhook config", func(t *testing.T) {
		// Create fresh mock for each test case
		mockRepo := &mocks.RepositoryMock{}
		mockUserRepo := &userMocks.RepositoryMock{}
		uc := NewUseCase(mockRepo, mockUserRepo)

		webhookConfig := &entities.WebhookConfig{
			URL:     "https://example.com/webhook",
			Enabled: true,
		}

		// Setup user mock
		mockUserRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			return &entities.User{
				ID:          userID,
				Email:       "test@example.com",
				AccountType: entities.AccountTypeFreemium,
			}, nil
		}
		mockRepo.GetByUserIDFunc = func(ctx context.Context, userID uuid.UUID) ([]*entities.Domain, error) {
			return []*entities.Domain{}, nil // No existing domains
		}
		mockRepo.GetByDomainFunc = func(ctx context.Context, domain string) (*entities.Domain, error) {
			return nil, sql.ErrNoRows
		}
		mockRepo.CreateFunc = func(ctx context.Context, domain *entities.Domain) error {
			return nil
		}

		result, err := uc.CreateDomain(ctx, CreateDomainInput{
			UserID:        userID,
			Domain:        domain,
			PublicKey:     publicKey,
			WebhookConfig: webhookConfig,
		})

		assert.NoError(t, err)
		assert.NotNil(t, result.WebhookConfig)
		assert.Equal(t, webhookConfig.URL, result.WebhookConfig.URL)
		assert.True(t, result.WebhookConfig.Enabled)

		// Verify calls
		assert.Len(t, mockRepo.GetByDomainCalls(), 1)
		assert.Len(t, mockRepo.CreateCalls(), 1)
	})

	t.Run("storage disabled", func(t *testing.T) {
		// Create fresh mock for each test case
		mockRepo := &mocks.RepositoryMock{}
		mockUserRepo := &userMocks.RepositoryMock{}
		uc := NewUseCase(mockRepo, mockUserRepo)

		storageEnabled := false

		// Setup user mock
		mockUserRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			return &entities.User{
				ID:          userID,
				Email:       "test@example.com",
				AccountType: entities.AccountTypeFreemium,
			}, nil
		}
		mockRepo.GetByUserIDFunc = func(ctx context.Context, userID uuid.UUID) ([]*entities.Domain, error) {
			return []*entities.Domain{}, nil // No existing domains
		}
		mockRepo.GetByDomainFunc = func(ctx context.Context, domain string) (*entities.Domain, error) {
			return nil, sql.ErrNoRows
		}
		mockRepo.CreateFunc = func(ctx context.Context, domain *entities.Domain) error {
			return nil
		}

		result, err := uc.CreateDomain(ctx, CreateDomainInput{
			UserID:         userID,
			Domain:         domain,
			PublicKey:      publicKey,
			StorageEnabled: &storageEnabled,
		})

		assert.NoError(t, err)
		assert.False(t, result.StorageEnabled)

		// Verify calls
		assert.Len(t, mockRepo.GetByDomainCalls(), 1)
		assert.Len(t, mockRepo.CreateCalls(), 1)
	})
}

func TestGetDomainByID(t *testing.T) {
	ctx := context.Background()
	domainID := uuid.Must(uuid.NewV4())

	t.Run("successful retrieval", func(t *testing.T) {
		// Create fresh mock for each test case
		mockRepo := &mocks.RepositoryMock{}
		mockUserRepo := &userMocks.RepositoryMock{}
		uc := NewUseCase(mockRepo, mockUserRepo)

		expectedDomain := &entities.Domain{
			ID:     domainID,
			Domain: "example.com",
		}

		mockRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.Domain, error) {
			return expectedDomain, nil
		}

		result, err := uc.GetDomainByID(ctx, domainID)

		assert.NoError(t, err)
		assert.Equal(t, expectedDomain, result)

		// Verify calls
		assert.Len(t, mockRepo.GetByIDCalls(), 1)
		assert.Equal(t, domainID, mockRepo.GetByIDCalls()[0].ID)
	})

	t.Run("empty domain ID", func(t *testing.T) {
		// Create fresh mock for each test case
		mockRepo := &mocks.RepositoryMock{}
		mockUserRepo := &userMocks.RepositoryMock{}
		uc := NewUseCase(mockRepo, mockUserRepo)

		result, err := uc.GetDomainByID(ctx, uuid.Nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "domain ID is required")
		assert.Nil(t, result)
	})

	t.Run("domain not found", func(t *testing.T) {
		// Create fresh mock for each test case
		mockRepo := &mocks.RepositoryMock{}
		mockUserRepo := &userMocks.RepositoryMock{}
		uc := NewUseCase(mockRepo, mockUserRepo)

		mockRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.Domain, error) {
			return nil, sql.ErrNoRows
		}

		result, err := uc.GetDomainByID(ctx, domainID)

		assert.Error(t, err)
		assert.Nil(t, result)

		// Verify calls
		assert.Len(t, mockRepo.GetByIDCalls(), 1)
	})
}

func TestUpdateDomain(t *testing.T) {
	ctx := context.Background()
	domainID := uuid.Must(uuid.NewV4())

	t.Run("successful update", func(t *testing.T) {
		// Create fresh mock for each test case
		mockRepo := &mocks.RepositoryMock{}
		mockUserRepo := &userMocks.RepositoryMock{}
		uc := NewUseCase(mockRepo, mockUserRepo)

		existingDomain := &entities.Domain{
			ID:             domainID,
			UserID:         uuid.Must(uuid.NewV4()),
			Domain:         "example.com",
			PublicKey:      "old-key",
			APIKey:         "pm_test_api_key",
			Verified:       false,
			StorageEnabled: true,
			CreatedAt:      time.Now().UTC(),
			UpdatedAt:      time.Now().UTC(),
		}

		newPublicKey := "new-public-key"
		verified := true

		mockRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.Domain, error) {
			return existingDomain, nil
		}
		mockRepo.UpdateFunc = func(ctx context.Context, domain *entities.Domain) error {
			return nil
		}

		result, err := uc.UpdateDomain(ctx, domainID, UpdateDomainInput{
			PublicKey: &newPublicKey,
			Verified:  &verified,
		})

		assert.NoError(t, err)
		assert.Equal(t, newPublicKey, result.PublicKey)
		assert.True(t, result.Verified)

		// Verify calls
		assert.Len(t, mockRepo.GetByIDCalls(), 1)
		assert.Len(t, mockRepo.UpdateCalls(), 1)
	})

	t.Run("empty domain ID", func(t *testing.T) {
		// Create fresh mock for each test case
		mockRepo := &mocks.RepositoryMock{}
		mockUserRepo := &userMocks.RepositoryMock{}
		uc := NewUseCase(mockRepo, mockUserRepo)

		result, err := uc.UpdateDomain(ctx, uuid.Nil, UpdateDomainInput{})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "domain ID is required")
		assert.Nil(t, result)
	})
}

func TestDeleteDomain(t *testing.T) {
	ctx := context.Background()
	domainID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	otherUserID := uuid.Must(uuid.NewV4())

	t.Run("successful deletion", func(t *testing.T) {
		// Create fresh mock for each test case
		mockRepo := &mocks.RepositoryMock{}
		mockUserRepo := &userMocks.RepositoryMock{}
		uc := NewUseCase(mockRepo, mockUserRepo)

		domain := &entities.Domain{
			ID:     domainID,
			UserID: userID,
		}

		mockRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.Domain, error) {
			return domain, nil
		}
		mockRepo.DeleteFunc = func(ctx context.Context, id uuid.UUID) error {
			return nil
		}

		err := uc.DeleteDomain(ctx, domainID, userID)

		assert.NoError(t, err)

		// Verify calls
		assert.Len(t, mockRepo.GetByIDCalls(), 1)
		assert.Len(t, mockRepo.DeleteCalls(), 1)
	})

	t.Run("unauthorized deletion", func(t *testing.T) {
		// Create fresh mock for each test case
		mockRepo := &mocks.RepositoryMock{}
		mockUserRepo := &userMocks.RepositoryMock{}
		uc := NewUseCase(mockRepo, mockUserRepo)

		domain := &entities.Domain{
			ID:     domainID,
			UserID: userID,
		}

		mockRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.Domain, error) {
			return domain, nil
		}

		err := uc.DeleteDomain(ctx, domainID, otherUserID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized")

		// Verify calls
		assert.Len(t, mockRepo.GetByIDCalls(), 1)
		assert.Len(t, mockRepo.DeleteCalls(), 0) // Should not call delete for unauthorized user
	})
}

func TestDomainValidation(t *testing.T) {
	testCases := []struct {
		domain string
		valid  bool
	}{
		{"example.com", true},
		{"sub.example.com", true},
		{"test-domain.co.uk", true},
		{"123.test.org", true},
		{"invalid", false},
		{"", false},
		{".com", false},
		{"example.", false},
		{"ex ample.com", false},
		{string(make([]byte, 300)), false}, // Too long
	}

	for _, tc := range testCases {
		t.Run(tc.domain, func(t *testing.T) {
			result := isValidDomain(tc.domain)
			assert.Equal(t, tc.valid, result)
		})
	}
}

func TestGenerateAPIKey(t *testing.T) {
	apiKey, err := generateAPIKey()

	assert.NoError(t, err)
	assert.NotEmpty(t, apiKey)
	assert.True(t, len(apiKey) > 10)
	assert.Contains(t, apiKey, "pm_")

	// Generate another to ensure uniqueness
	apiKey2, err := generateAPIKey()
	assert.NoError(t, err)
	assert.NotEqual(t, apiKey, apiKey2)
}

func TestCreateDomainWithAutoCreateAddress(t *testing.T) {
	ctx := context.Background()
	userID := uuid.Must(uuid.NewV4())
	domain := "test.example.com"
	// Valid x25519 public key format
	publicKey := "x25519:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	t.Run("create domain with auto-create enabled", func(t *testing.T) {
		mockRepo := &mocks.RepositoryMock{}
		mockUserRepo := &userMocks.RepositoryMock{}
		uc := NewUseCase(mockRepo, mockUserRepo)
		
		autoCreateAddress := true

		// Setup mocks
		mockUserRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			return &entities.User{
				ID:          userID,
				Email:       "test@example.com",
				AccountType: entities.AccountTypeFreemium,
			}, nil
		}
		mockRepo.GetByUserIDFunc = func(ctx context.Context, userID uuid.UUID) ([]*entities.Domain, error) {
			return []*entities.Domain{}, nil // No existing domains
		}
		mockRepo.GetByDomainFunc = func(ctx context.Context, domain string) (*entities.Domain, error) {
			return nil, sql.ErrNoRows
		}
		mockRepo.CreateFunc = func(ctx context.Context, domain *entities.Domain) error {
			return nil
		}

		// Execute
		result, err := uc.CreateDomain(ctx, CreateDomainInput{
			UserID:           userID,
			Domain:           domain,
			PublicKey:        publicKey,
			AutoCreateAddress: &autoCreateAddress,
		})

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.AutoCreateAddress)
	})

	t.Run("create domain with auto-create disabled", func(t *testing.T) {
		mockRepo := &mocks.RepositoryMock{}
		mockUserRepo := &userMocks.RepositoryMock{}
		uc := NewUseCase(mockRepo, mockUserRepo)
		
		autoCreateAddress := false

		// Setup mocks
		mockUserRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			return &entities.User{
				ID:          userID,
				Email:       "test@example.com",
				AccountType: entities.AccountTypeFreemium,
			}, nil
		}
		mockRepo.GetByUserIDFunc = func(ctx context.Context, userID uuid.UUID) ([]*entities.Domain, error) {
			return []*entities.Domain{}, nil // No existing domains
		}
		mockRepo.GetByDomainFunc = func(ctx context.Context, domain string) (*entities.Domain, error) {
			return nil, sql.ErrNoRows
		}
		mockRepo.CreateFunc = func(ctx context.Context, domain *entities.Domain) error {
			return nil
		}

		// Execute
		result, err := uc.CreateDomain(ctx, CreateDomainInput{
			UserID:           userID,
			Domain:           domain,
			PublicKey:        publicKey,
			AutoCreateAddress: &autoCreateAddress,
		})

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.AutoCreateAddress)
	})

	t.Run("create domain with default auto-create (false)", func(t *testing.T) {
		mockRepo := &mocks.RepositoryMock{}
		mockUserRepo := &userMocks.RepositoryMock{}
		uc := NewUseCase(mockRepo, mockUserRepo)

		// Setup mocks
		mockUserRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.User, error) {
			return &entities.User{
				ID:          userID,
				Email:       "test@example.com",
				AccountType: entities.AccountTypeFreemium,
			}, nil
		}
		mockRepo.GetByUserIDFunc = func(ctx context.Context, userID uuid.UUID) ([]*entities.Domain, error) {
			return []*entities.Domain{}, nil // No existing domains
		}
		mockRepo.GetByDomainFunc = func(ctx context.Context, domain string) (*entities.Domain, error) {
			return nil, sql.ErrNoRows
		}
		mockRepo.CreateFunc = func(ctx context.Context, domain *entities.Domain) error {
			return nil
		}

		// Execute without AutoCreateAddress field
		result, err := uc.CreateDomain(ctx, CreateDomainInput{
			UserID:    userID,
			Domain:    domain,
			PublicKey: publicKey,
		})

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.AutoCreateAddress) // Default should be false
	})
}

func TestUpdateDomainAutoCreateAddress(t *testing.T) {
	ctx := context.Background()
	domainID := uuid.Must(uuid.NewV4())

	t.Run("update auto-create emails setting", func(t *testing.T) {
		mockRepo := &mocks.RepositoryMock{}
		mockUserRepo := &userMocks.RepositoryMock{}
		uc := NewUseCase(mockRepo, mockUserRepo)

		existingDomain := &entities.Domain{
			ID:               domainID,
			UserID:           uuid.Must(uuid.NewV4()),
			Domain:           "example.com",
			PublicKey:        "test-key",
			APIKey:           "pm_test_api_key",
			Verified:         true,
			StorageEnabled:   true,
			AutoCreateAddress: false, // Initially disabled
			CreatedAt:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
		}

		autoCreateAddress := true // Enable auto-create

		mockRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.Domain, error) {
			return existingDomain, nil
		}
		mockRepo.UpdateFunc = func(ctx context.Context, domain *entities.Domain) error {
			return nil
		}

		result, err := uc.UpdateDomain(ctx, domainID, UpdateDomainInput{
			AutoCreateAddress: &autoCreateAddress,
		})

		assert.NoError(t, err)
		assert.True(t, result.AutoCreateAddress)

		// Verify calls
		assert.Len(t, mockRepo.GetByIDCalls(), 1)
		assert.Len(t, mockRepo.UpdateCalls(), 1)
	})
}
