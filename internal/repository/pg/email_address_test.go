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

func TestEmailAddressRepository_Create(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewEmailAddressRepository(mockDB)
	ctx := context.Background()

	emailAddress := &entities.EmailAddress{
		ID:               uuid.Must(uuid.NewV4()),
		DomainID:         uuid.Must(uuid.NewV4()),
		LocalPart:        "test",
		IsCatchAll:       false,
		ForwardAddresses: []string{"forward@example.com"},
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	t.Run("successful creation", func(t *testing.T) {
		mockDB.On("Exec", ctx, mock.AnythingOfType("string"),
			emailAddress.ID, emailAddress.DomainID, emailAddress.LocalPart,
			emailAddress.IsCatchAll, emailAddress.ForwardAddresses,
			emailAddress.CreatedAt, emailAddress.UpdatedAt).Return(NewMockCommandTag(1), nil).Once()

		err := repo.Create(ctx, emailAddress)

		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("database error", func(t *testing.T) {
		mockDB.ExpectedCalls = nil
		mockDB.On("Exec", ctx, mock.AnythingOfType("string"),
			mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything,
			mock.Anything, mock.Anything,
		).Return(pgconn.CommandTag{}, assert.AnError).Once()

		err := repo.Create(ctx, emailAddress)

		assert.Error(t, err)
		mockDB.AssertExpectations(t)
	})
}

func TestEmailAddressRepository_GetByID(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewEmailAddressRepository(mockDB)
	ctx := context.Background()

	emailID := uuid.Must(uuid.NewV4())

	t.Run("successful retrieval", func(t *testing.T) {
		expectedEmail := &entities.EmailAddress{
			ID:               emailID,
			DomainID:         uuid.Must(uuid.NewV4()),
			LocalPart:        "test",
			IsCatchAll:       true,
			ForwardAddresses: []string{"forward@example.com", "backup@test.com"},
			CreatedAt:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
		}

		mockRow := &MockRow{
			data: []interface{}{
				expectedEmail.ID,
				expectedEmail.DomainID,
				expectedEmail.LocalPart,
				expectedEmail.IsCatchAll,
				expectedEmail.ForwardAddresses,
				expectedEmail.CreatedAt,
				expectedEmail.UpdatedAt,
			},
		}

		mockDB.On("QueryRow", ctx, mock.AnythingOfType("string"), emailID).Return(mockRow).Once()

		result, err := repo.GetByID(ctx, emailID)

		assert.NoError(t, err)
		assert.Equal(t, expectedEmail.ID, result.ID)
		assert.Equal(t, expectedEmail.LocalPart, result.LocalPart)
		assert.Equal(t, expectedEmail.IsCatchAll, result.IsCatchAll)
		assert.Equal(t, expectedEmail.ForwardAddresses, result.ForwardAddresses)

		mockDB.AssertExpectations(t)
	})

	t.Run("email address not found", func(t *testing.T) {
		mockRow := &MockRow{
			scanResult: pgx.ErrNoRows,
		}

		mockDB.On("QueryRow", ctx, mock.AnythingOfType("string"), emailID).Return(mockRow).Once()

		result, err := repo.GetByID(ctx, emailID)

		assert.Error(t, err)
		assert.Equal(t, sql.ErrNoRows, err)
		assert.Nil(t, result)

		mockDB.AssertExpectations(t)
	})
}

func TestEmailAddressRepository_GetByDomainID(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewEmailAddressRepository(mockDB)
	ctx := context.Background()

	domainID := uuid.Must(uuid.NewV4())

	t.Run("successful retrieval", func(t *testing.T) {
		email1ID := uuid.Must(uuid.NewV4())
		email2ID := uuid.Must(uuid.NewV4())

		mockRows := &MockRows{
			rows: [][]interface{}{
				{
					email1ID,
					domainID,
					"test1",
					false,
					[]string{"forward1@example.com"},
					time.Now().UTC(),
					time.Now().UTC(),
				},
				{
					email2ID,
					domainID,
					"test2",
					true,
					[]string{},
					time.Now().UTC(),
					time.Now().UTC(),
				},
			},
		}

		mockDB.On("Query", ctx, mock.AnythingOfType("string"), domainID).Return(mockRows, nil).Once()

		result, err := repo.GetByDomainID(ctx, domainID)

		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "test1", result[0].LocalPart)
		assert.Equal(t, "test2", result[1].LocalPart)
		assert.False(t, result[0].IsCatchAll)
		assert.True(t, result[1].IsCatchAll)

		mockDB.AssertExpectations(t)
	})

	t.Run("no email addresses found", func(t *testing.T) {
		mockRows := &MockRows{
			rows: [][]interface{}{},
		}

		mockDB.On("Query", ctx, mock.AnythingOfType("string"), domainID).Return(mockRows, nil).Once()

		result, err := repo.GetByDomainID(ctx, domainID)

		assert.NoError(t, err)
		assert.Empty(t, result)

		mockDB.AssertExpectations(t)
	})
}

func TestEmailAddressRepository_GetByLocalPartAndDomain(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewEmailAddressRepository(mockDB)
	ctx := context.Background()

	localPart := "test"
	domainID := uuid.Must(uuid.NewV4())

	t.Run("successful retrieval", func(t *testing.T) {
		expectedEmail := &entities.EmailAddress{
			ID:               uuid.Must(uuid.NewV4()),
			DomainID:         domainID,
			LocalPart:        localPart,
			IsCatchAll:       false,
			ForwardAddresses: []string{},
			CreatedAt:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
		}

		mockRow := &MockRow{
			data: []interface{}{
				expectedEmail.ID,
				expectedEmail.DomainID,
				expectedEmail.LocalPart,
				expectedEmail.IsCatchAll,
				expectedEmail.ForwardAddresses,
				expectedEmail.CreatedAt,
				expectedEmail.UpdatedAt,
			},
		}

		mockDB.On("QueryRow", ctx, mock.AnythingOfType("string"), localPart, domainID).Return(mockRow).Once()

		result, err := repo.GetByLocalPartAndDomain(ctx, localPart, domainID)

		assert.NoError(t, err)
		assert.Equal(t, expectedEmail.LocalPart, result.LocalPart)
		assert.Equal(t, expectedEmail.DomainID, result.DomainID)

		mockDB.AssertExpectations(t)
	})
}

func TestEmailAddressRepository_GetCatchAllByDomainID(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewEmailAddressRepository(mockDB)
	ctx := context.Background()

	domainID := uuid.Must(uuid.NewV4())

	t.Run("catch-all found", func(t *testing.T) {
		expectedEmail := &entities.EmailAddress{
			ID:         uuid.Must(uuid.NewV4()),
			DomainID:   domainID,
			LocalPart:  "*",
			IsCatchAll: true,
			CreatedAt:  time.Now().UTC(),
			UpdatedAt:  time.Now().UTC(),
		}

		mockRow := &MockRow{
			data: []interface{}{
				expectedEmail.ID,
				expectedEmail.DomainID,
				expectedEmail.LocalPart,
				expectedEmail.IsCatchAll,
				[]string{},
				expectedEmail.CreatedAt,
				expectedEmail.UpdatedAt,
			},
		}

		mockDB.On("QueryRow", ctx, mock.AnythingOfType("string"), domainID).Return(mockRow).Once()

		result, err := repo.GetCatchAllByDomainID(ctx, domainID)

		assert.NoError(t, err)
		assert.True(t, result.IsCatchAll)
		assert.Equal(t, domainID, result.DomainID)

		mockDB.AssertExpectations(t)
	})

	t.Run("no catch-all found", func(t *testing.T) {
		mockRow := &MockRow{
			scanResult: pgx.ErrNoRows,
		}

		mockDB.On("QueryRow", ctx, mock.AnythingOfType("string"), domainID).Return(mockRow).Once()

		result, err := repo.GetCatchAllByDomainID(ctx, domainID)

		assert.Error(t, err)
		assert.Equal(t, sql.ErrNoRows, err)
		assert.Nil(t, result)

		mockDB.AssertExpectations(t)
	})
}

func TestEmailAddressRepository_Update(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewEmailAddressRepository(mockDB)
	ctx := context.Background()

	emailAddress := &entities.EmailAddress{
		ID:               uuid.Must(uuid.NewV4()),
		DomainID:         uuid.Must(uuid.NewV4()),
		LocalPart:        "updated",
		IsCatchAll:       true,
		ForwardAddresses: []string{"new@example.com"},
		UpdatedAt:        time.Now().UTC(),
	}

	t.Run("successful update", func(t *testing.T) {
		mockDB.On("Exec", ctx, mock.AnythingOfType("string"),
			emailAddress.ID, emailAddress.DomainID, emailAddress.LocalPart,
			emailAddress.IsCatchAll, emailAddress.ForwardAddresses,
			emailAddress.UpdatedAt).Return(NewMockCommandTag(1), nil).Once()

		err := repo.Update(ctx, emailAddress)

		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("email address not found", func(t *testing.T) {
		mockDB.ExpectedCalls = nil
		mockDB.On("Exec", ctx, mock.AnythingOfType("string"),
			mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything,
			mock.Anything,
		).Return(NewMockCommandTag(0), nil).Once()

		err := repo.Update(ctx, emailAddress)

		assert.Error(t, err)
		assert.Equal(t, sql.ErrNoRows, err)
		mockDB.AssertExpectations(t)
	})
}

func TestEmailAddressRepository_Delete(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewEmailAddressRepository(mockDB)
	ctx := context.Background()

	emailID := uuid.Must(uuid.NewV4())

	t.Run("successful deletion", func(t *testing.T) {
		mockDB.On("Exec", ctx, mock.AnythingOfType("string"), emailID).Return(NewMockCommandTag(1), nil).Once()

		err := repo.Delete(ctx, emailID)

		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("email address not found", func(t *testing.T) {
		mockDB.On("Exec", ctx, mock.AnythingOfType("string"), emailID).Return(NewMockCommandTag(0), nil).Once()

		err := repo.Delete(ctx, emailID)

		assert.Error(t, err)
		assert.Equal(t, sql.ErrNoRows, err)
		mockDB.AssertExpectations(t)
	})
}
