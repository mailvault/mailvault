package email

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"mailvault/domain/email/mocks"
	"mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
)

func TestCreateEmailAddressFromInput(t *testing.T) {
	ctx := context.Background()
	domainID := uuid.Must(uuid.NewV4())
	localPart := "test"

	t.Run("successful creation", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockEmailRepo := &mocks.EmailAddressRepositoryMock{}
		mockReceivedRepo := &mocks.ReceivedEmailRepositoryMock{}
		mockDomainRepo := &mocks.DomainRepositoryMock{}
		uc := NewUseCase(mockEmailRepo, mockReceivedRepo, mockDomainRepo)

		// Setup mocks
		mockEmailRepo.GetByLocalPartAndDomainFunc = func(ctx context.Context, localPart string, domainID uuid.UUID) (*entities.EmailAddress, error) {
			return nil, sql.ErrNoRows
		}
		mockEmailRepo.CreateFunc = func(ctx context.Context, emailAddress *entities.EmailAddress) error {
			return nil
		}

		// Execute
		result, err := uc.CreateEmailAddressFromInput(ctx, CreateEmailAddressInput{
			DomainID:  domainID,
			LocalPart: localPart,
		})

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, domainID, result.DomainID)
		assert.Equal(t, localPart, result.LocalPart)
		assert.False(t, result.IsCatchAll)
		assert.Empty(t, result.ForwardAddresses)

		// Verify calls
		assert.Len(t, mockEmailRepo.GetByLocalPartAndDomainCalls(), 1)
		assert.Len(t, mockEmailRepo.CreateCalls(), 1)
	})

	t.Run("validation errors", func(t *testing.T) {
		testCases := []struct {
			name  string
			input CreateEmailAddressInput
			error string
		}{
			{
				name:  "empty domain ID",
				input: CreateEmailAddressInput{LocalPart: localPart},
				error: "domain ID is required",
			},
			{
				name:  "empty local part",
				input: CreateEmailAddressInput{DomainID: domainID},
				error: "local part is required",
			},
			{
				name:  "invalid local part format",
				input: CreateEmailAddressInput{DomainID: domainID, LocalPart: "invalid@part"},
				error: "invalid local part format",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create fresh mocks for each validation test case
				mockEmailRepo := &mocks.EmailAddressRepositoryMock{}
				mockReceivedRepo := &mocks.ReceivedEmailRepositoryMock{}
				mockDomainRepo := &mocks.DomainRepositoryMock{}
				uc := NewUseCase(mockEmailRepo, mockReceivedRepo, mockDomainRepo)

				result, err := uc.CreateEmailAddressFromInput(ctx, tc.input)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.error)
				assert.Nil(t, result)
			})
		}
	})

	t.Run("email address already exists", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockEmailRepo := &mocks.EmailAddressRepositoryMock{}
		mockReceivedRepo := &mocks.ReceivedEmailRepositoryMock{}
		mockDomainRepo := &mocks.DomainRepositoryMock{}
		uc := NewUseCase(mockEmailRepo, mockReceivedRepo, mockDomainRepo)

		existingEmail := &entities.EmailAddress{
			ID:        uuid.Must(uuid.NewV4()),
			LocalPart: localPart,
		}

		mockEmailRepo.GetByLocalPartAndDomainFunc = func(ctx context.Context, localPart string, domainID uuid.UUID) (*entities.EmailAddress, error) {
			return existingEmail, nil
		}

		result, err := uc.CreateEmailAddressFromInput(ctx, CreateEmailAddressInput{
			DomainID:  domainID,
			LocalPart: localPart,
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
		assert.Nil(t, result)

		// Verify calls
		assert.Len(t, mockEmailRepo.GetByLocalPartAndDomainCalls(), 1)
	})

	t.Run("catch-all creation with existing catch-all", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockEmailRepo := &mocks.EmailAddressRepositoryMock{}
		mockReceivedRepo := &mocks.ReceivedEmailRepositoryMock{}
		mockDomainRepo := &mocks.DomainRepositoryMock{}
		uc := NewUseCase(mockEmailRepo, mockReceivedRepo, mockDomainRepo)

		existingCatchAll := &entities.EmailAddress{
			ID:         uuid.Must(uuid.NewV4()),
			IsCatchAll: true,
		}

		mockEmailRepo.GetByLocalPartAndDomainFunc = func(ctx context.Context, localPart string, domainID uuid.UUID) (*entities.EmailAddress, error) {
			return nil, sql.ErrNoRows
		}
		mockEmailRepo.GetCatchAllByDomainIDFunc = func(ctx context.Context, domainID uuid.UUID) (*entities.EmailAddress, error) {
			return existingCatchAll, nil
		}

		result, err := uc.CreateEmailAddressFromInput(ctx, CreateEmailAddressInput{
			DomainID:   domainID,
			LocalPart:  localPart,
			IsCatchAll: true,
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already has a catch-all")
		assert.Nil(t, result)

		// Verify calls
		assert.Len(t, mockEmailRepo.GetByLocalPartAndDomainCalls(), 1)
		assert.Len(t, mockEmailRepo.GetCatchAllByDomainIDCalls(), 1)
	})

	t.Run("with forward addresses", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockEmailRepo := &mocks.EmailAddressRepositoryMock{}
		mockReceivedRepo := &mocks.ReceivedEmailRepositoryMock{}
		mockDomainRepo := &mocks.DomainRepositoryMock{}
		uc := NewUseCase(mockEmailRepo, mockReceivedRepo, mockDomainRepo)

		forwardAddresses := []string{"forward@example.com", "backup@test.com"}

		mockEmailRepo.GetByLocalPartAndDomainFunc = func(ctx context.Context, localPart string, domainID uuid.UUID) (*entities.EmailAddress, error) {
			return nil, sql.ErrNoRows
		}
		mockEmailRepo.CreateFunc = func(ctx context.Context, emailAddress *entities.EmailAddress) error {
			return nil
		}

		result, err := uc.CreateEmailAddressFromInput(ctx, CreateEmailAddressInput{
			DomainID:         domainID,
			LocalPart:        localPart,
			ForwardAddresses: forwardAddresses,
		})

		assert.NoError(t, err)
		assert.Equal(t, forwardAddresses, result.ForwardAddresses)

		// Verify calls
		assert.Len(t, mockEmailRepo.GetByLocalPartAndDomainCalls(), 1)
		assert.Len(t, mockEmailRepo.CreateCalls(), 1)
	})

	t.Run("invalid forward address", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockEmailRepo := &mocks.EmailAddressRepositoryMock{}
		mockReceivedRepo := &mocks.ReceivedEmailRepositoryMock{}
		mockDomainRepo := &mocks.DomainRepositoryMock{}
		uc := NewUseCase(mockEmailRepo, mockReceivedRepo, mockDomainRepo)

		invalidForwardAddresses := []string{"invalid-email"}

		result, err := uc.CreateEmailAddressFromInput(ctx, CreateEmailAddressInput{
			DomainID:         domainID,
			LocalPart:        localPart,
			ForwardAddresses: invalidForwardAddresses,
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid forward address")
		assert.Nil(t, result)
	})
}

func TestUpdateEmailAddress(t *testing.T) {
	ctx := context.Background()
	emailID := uuid.Must(uuid.NewV4())
	domainID := uuid.Must(uuid.NewV4())

	t.Run("successful update", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockEmailRepo := &mocks.EmailAddressRepositoryMock{}
		mockReceivedRepo := &mocks.ReceivedEmailRepositoryMock{}
		mockDomainRepo := &mocks.DomainRepositoryMock{}
		uc := NewUseCase(mockEmailRepo, mockReceivedRepo, mockDomainRepo)

		existingEmail := &entities.EmailAddress{
			ID:         emailID,
			DomainID:   domainID,
			LocalPart:  "test",
			IsCatchAll: false,
			CreatedAt:  time.Now().UTC(),
			UpdatedAt:  time.Now().UTC(),
		}

		isCatchAll := true
		forwardAddresses := []string{"forward@example.com"}

		mockEmailRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*entities.EmailAddress, error) {
			return existingEmail, nil
		}
		mockEmailRepo.GetCatchAllByDomainIDFunc = func(ctx context.Context, domainID uuid.UUID) (*entities.EmailAddress, error) {
			return nil, sql.ErrNoRows
		}
		mockEmailRepo.UpdateFunc = func(ctx context.Context, emailAddress *entities.EmailAddress) error {
			return nil
		}

		result, err := uc.UpdateEmailAddress(ctx, emailID, UpdateEmailAddressInput{
			IsCatchAll:       &isCatchAll,
			ForwardAddresses: forwardAddresses,
		})

		assert.NoError(t, err)
		assert.True(t, result.IsCatchAll)
		assert.Equal(t, forwardAddresses, result.ForwardAddresses)

		// Verify calls
		assert.Len(t, mockEmailRepo.GetByIDCalls(), 1)
		assert.Len(t, mockEmailRepo.GetCatchAllByDomainIDCalls(), 1)
		assert.Len(t, mockEmailRepo.UpdateCalls(), 1)
	})

	t.Run("empty email ID", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockEmailRepo := &mocks.EmailAddressRepositoryMock{}
		mockReceivedRepo := &mocks.ReceivedEmailRepositoryMock{}
		mockDomainRepo := &mocks.DomainRepositoryMock{}
		uc := NewUseCase(mockEmailRepo, mockReceivedRepo, mockDomainRepo)

		result, err := uc.UpdateEmailAddress(ctx, uuid.Nil, UpdateEmailAddressInput{})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "email address ID is required")
		assert.Nil(t, result)
	})
}

func TestGetEmailAddressByAddress(t *testing.T) {
	ctx := context.Background()

	t.Run("successful retrieval", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockEmailRepo := &mocks.EmailAddressRepositoryMock{}
		mockReceivedRepo := &mocks.ReceivedEmailRepositoryMock{}
		mockDomainRepo := &mocks.DomainRepositoryMock{}
		uc := NewUseCase(mockEmailRepo, mockReceivedRepo, mockDomainRepo)

		domainID := uuid.Must(uuid.NewV4())
		domainEntity := &entities.Domain{
			ID:     domainID,
			Domain: "example.com",
		}
		emailAddress := &entities.EmailAddress{
			ID:        uuid.Must(uuid.NewV4()),
			DomainID:  domainID,
			LocalPart: "test",
		}

		mockDomainRepo.GetByDomainFunc = func(ctx context.Context, domain string) (*entities.Domain, error) {
			return domainEntity, nil
		}
		mockEmailRepo.GetByLocalPartAndDomainFunc = func(ctx context.Context, localPart string, domainID uuid.UUID) (*entities.EmailAddress, error) {
			return emailAddress, nil
		}

		result, err := uc.GetEmailAddressByAddress(ctx, "test@example.com")

		assert.NoError(t, err)
		assert.Equal(t, emailAddress, result)

		// Verify calls
		assert.Len(t, mockDomainRepo.GetByDomainCalls(), 1)
		assert.Len(t, mockEmailRepo.GetByLocalPartAndDomainCalls(), 1)
	})

	t.Run("invalid email format", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockEmailRepo := &mocks.EmailAddressRepositoryMock{}
		mockReceivedRepo := &mocks.ReceivedEmailRepositoryMock{}
		mockDomainRepo := &mocks.DomainRepositoryMock{}
		uc := NewUseCase(mockEmailRepo, mockReceivedRepo, mockDomainRepo)

		result, err := uc.GetEmailAddressByAddress(ctx, "invalid-email")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid email address format")
		assert.Nil(t, result)
	})
}

func TestProcessIncomingEmail(t *testing.T) {
	ctx := context.Background()
	emailAddressID := uuid.Must(uuid.NewV4())

	t.Run("successful processing", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockEmailRepo := &mocks.EmailAddressRepositoryMock{}
		mockReceivedRepo := &mocks.ReceivedEmailRepositoryMock{}
		mockDomainRepo := &mocks.DomainRepositoryMock{}
		uc := NewUseCase(mockEmailRepo, mockReceivedRepo, mockDomainRepo)

		mockReceivedRepo.CreateFunc = func(ctx context.Context, email *entities.ReceivedEmail) error {
			return nil
		}

		err := uc.ProcessIncomingEmail(ctx, ProcessIncomingEmailInput{
			EmailAddressID: emailAddressID,
			FromAddress:    "sender@test.com",
			Subject:        "Test Subject",
			Body:           "Test Body",
		})

		assert.NoError(t, err)

		// Verify calls
		assert.Len(t, mockReceivedRepo.CreateCalls(), 1)
	})

	t.Run("validation errors", func(t *testing.T) {
		testCases := []struct {
			name  string
			input ProcessIncomingEmailInput
			error string
		}{
			{
				name:  "empty email address ID",
				input: ProcessIncomingEmailInput{FromAddress: "sender@test.com", Body: "body"},
				error: "email address ID is required",
			},
			{
				name:  "empty from address",
				input: ProcessIncomingEmailInput{EmailAddressID: emailAddressID, Body: "body"},
				error: "from address is required",
			},
			{
				name:  "empty body",
				input: ProcessIncomingEmailInput{EmailAddressID: emailAddressID, FromAddress: "sender@test.com"},
				error: "body is required",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create fresh mocks for each validation test case
				mockEmailRepo := &mocks.EmailAddressRepositoryMock{}
				mockReceivedRepo := &mocks.ReceivedEmailRepositoryMock{}
				mockDomainRepo := &mocks.DomainRepositoryMock{}
				uc := NewUseCase(mockEmailRepo, mockReceivedRepo, mockDomainRepo)

				err := uc.ProcessIncomingEmail(ctx, tc.input)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.error)
			})
		}
	})
}

func TestGetReceivedEmails(t *testing.T) {
	ctx := context.Background()
	emailAddressID := uuid.Must(uuid.NewV4())

	t.Run("successful retrieval", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockEmailRepo := &mocks.EmailAddressRepositoryMock{}
		mockReceivedRepo := &mocks.ReceivedEmailRepositoryMock{}
		mockDomainRepo := &mocks.DomainRepositoryMock{}
		uc := NewUseCase(mockEmailRepo, mockReceivedRepo, mockDomainRepo)

		expectedEmails := []*entities.ReceivedEmail{
			{
				ID:             uuid.Must(uuid.NewV4()),
				EmailAddressID: &emailAddressID,
				FromAddress:    "sender@test.com",
			},
		}

		mockReceivedRepo.GetByEmailAddressIDFunc = func(ctx context.Context, emailAddressID uuid.UUID, limit, offset int) ([]*entities.ReceivedEmail, error) {
			return expectedEmails, nil
		}

		result, err := uc.GetReceivedEmails(ctx, emailAddressID, 0, 0) // Will use defaults

		assert.NoError(t, err)
		assert.Equal(t, expectedEmails, result)

		// Verify calls
		assert.Len(t, mockReceivedRepo.GetByEmailAddressIDCalls(), 1)
		assert.Equal(t, 50, mockReceivedRepo.GetByEmailAddressIDCalls()[0].Limit) // Default limit
	})

	t.Run("with custom limits", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockEmailRepo := &mocks.EmailAddressRepositoryMock{}
		mockReceivedRepo := &mocks.ReceivedEmailRepositoryMock{}
		mockDomainRepo := &mocks.DomainRepositoryMock{}
		uc := NewUseCase(mockEmailRepo, mockReceivedRepo, mockDomainRepo)

		expectedEmails := []*entities.ReceivedEmail{}

		mockReceivedRepo.GetByEmailAddressIDFunc = func(ctx context.Context, emailAddressID uuid.UUID, limit, offset int) ([]*entities.ReceivedEmail, error) {
			return expectedEmails, nil
		}

		result, err := uc.GetReceivedEmails(ctx, emailAddressID, 100, 10)

		assert.NoError(t, err)
		assert.Equal(t, expectedEmails, result)

		// Verify calls
		assert.Len(t, mockReceivedRepo.GetByEmailAddressIDCalls(), 1)
		assert.Equal(t, 100, mockReceivedRepo.GetByEmailAddressIDCalls()[0].Limit)
		assert.Equal(t, 10, mockReceivedRepo.GetByEmailAddressIDCalls()[0].Offset)
	})

	t.Run("limit too high", func(t *testing.T) {
		// Create fresh mocks for each test case
		mockEmailRepo := &mocks.EmailAddressRepositoryMock{}
		mockReceivedRepo := &mocks.ReceivedEmailRepositoryMock{}
		mockDomainRepo := &mocks.DomainRepositoryMock{}
		uc := NewUseCase(mockEmailRepo, mockReceivedRepo, mockDomainRepo)

		expectedEmails := []*entities.ReceivedEmail{}

		mockReceivedRepo.GetByEmailAddressIDFunc = func(ctx context.Context, emailAddressID uuid.UUID, limit, offset int) ([]*entities.ReceivedEmail, error) {
			return expectedEmails, nil
		}

		result, err := uc.GetReceivedEmails(ctx, emailAddressID, 2000, 0)

		assert.NoError(t, err)
		assert.Equal(t, expectedEmails, result)

		// Verify calls - should cap at 1000
		assert.Len(t, mockReceivedRepo.GetByEmailAddressIDCalls(), 1)
		assert.Equal(t, 1000, mockReceivedRepo.GetByEmailAddressIDCalls()[0].Limit)
	})
}

func TestEmailValidation(t *testing.T) {
	t.Run("local part validation", func(t *testing.T) {
		testCases := []struct {
			localPart string
			valid     bool
		}{
			{"test", true},
			{"test123", true},
			{"test-user", true},
			{"test.user", true},
			{"test_user", true},
			{"", false},
			{".test", false},
			{"test.", false},
			{"test@invalid", false},
			{string(make([]byte, 70)), false}, // Too long
		}

		for _, tc := range testCases {
			t.Run(tc.localPart, func(t *testing.T) {
				result := isValidLocalPart(tc.localPart)
				assert.Equal(t, tc.valid, result, "LocalPart: %s", tc.localPart)
			})
		}
	})

	t.Run("email validation", func(t *testing.T) {
		testCases := []struct {
			email string
			valid bool
		}{
			{"test@example.com", true},
			{"user.name@domain.co.uk", true},
			{"test+tag@example.org", true},
			{"invalid-email", false},
			{"@domain.com", false},
			{"user@", false},
			{"user@domain", false},
		}

		for _, tc := range testCases {
			t.Run(tc.email, func(t *testing.T) {
				result := isValidEmail(tc.email)
				assert.Equal(t, tc.valid, result, "Email: %s", tc.email)
			})
		}
	})
}
