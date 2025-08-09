package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"mailsafe/domain/entities"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDBTX implements DBTX interface for testing
type MockDBTX struct {
	mock.Mock
}

func (m *MockDBTX) Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	mockArgs := []interface{}{ctx, query}
	mockArgs = append(mockArgs, args...)
	callArgs := m.Called(mockArgs...)

	if callArgs.Get(0) == nil {
		return nil, callArgs.Error(1)
	}
	return callArgs.Get(0).(pgx.Rows), callArgs.Error(1)
}

func (m *MockDBTX) QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row {
	mockArgs := []interface{}{ctx, query}
	mockArgs = append(mockArgs, args...)
	callArgs := m.Called(mockArgs...)
	return callArgs.Get(0).(pgx.Row)
}

func (m *MockDBTX) Exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	mockArgs := []interface{}{ctx, query}
	mockArgs = append(mockArgs, args...)
	callArgs := m.Called(mockArgs...)

	if callArgs.Get(0) == nil {
		return pgconn.CommandTag{}, callArgs.Error(1)
	}
	return callArgs.Get(0).(pgconn.CommandTag), callArgs.Error(1)
}

// MockRow implements pgx.Row for testing
type MockRow struct {
	mock.Mock
	scanResult error
	data       []interface{}
}

func (m *MockRow) Scan(dest ...interface{}) error {
	if m.scanResult != nil {
		return m.scanResult
	}

	// Copy mock data to destination
	for i := range dest {
		if i < len(m.data) && m.data[i] != nil {
			switch v := dest[i].(type) {
			case *uuid.UUID:
				*v = m.data[i].(uuid.UUID)
			case *string:
				*v = m.data[i].(string)
			case *bool:
				*v = m.data[i].(bool)
			case *[]byte:
				*v = m.data[i].([]byte)
			case *[]string:
				*v = m.data[i].([]string)
			case *time.Time:
				*v = m.data[i].(time.Time)
			}
		}
	}
	return nil
}

// MockRows implements pgx.Rows for testing
type MockRows struct {
	mock.Mock
	rows [][]interface{}
	pos  int
}

func (m *MockRows) Next() bool {
	m.pos++
	return m.pos <= len(m.rows)
}

func (m *MockRows) Scan(dest ...interface{}) error {
	if m.pos <= 0 || m.pos > len(m.rows) {
		return sql.ErrNoRows
	}

	row := m.rows[m.pos-1]
	for i := range dest {
		if i < len(row) && row[i] != nil {
			switch v := dest[i].(type) {
			case *uuid.UUID:
				*v = row[i].(uuid.UUID)
			case *string:
				*v = row[i].(string)
			case *bool:
				*v = row[i].(bool)
			case *[]byte:
				*v = row[i].([]byte)
			case *[]string:
				*v = row[i].([]string)
			case *time.Time:
				*v = row[i].(time.Time)
			}
		}
	}
	return nil
}

func (m *MockRows) Close() {
	// Mock implementation
}

func (m *MockRows) Err() error {
	return nil
}

// Satisfy pgx.Rows interface requirements not used by our tests
func (m *MockRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (m *MockRows) Conn() *pgx.Conn                              { return nil }
func (m *MockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (m *MockRows) RawValues() [][]byte                          { return nil }
func (m *MockRows) Values() ([]any, error)                       { return nil, nil }

// MockCommandTag implements pgconn.CommandTag for testing
func NewMockCommandTag(rowsAffected int64) pgconn.CommandTag {
	return pgconn.NewCommandTag("INSERT 0 " + string(rune('0'+rowsAffected)))
}

func TestDomainRepository_Create(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewDomainRepository(mockDB)
	ctx := context.Background()

	domain := &entities.Domain{
		ID:             uuid.Must(uuid.NewV4()),
		UserID:         uuid.Must(uuid.NewV4()),
		Domain:         "example.com",
		PublicKey:      "test-public-key",
		APIKey:         "pm_test_api_key",
		Verified:       false,
		StorageEnabled: true,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	t.Run("successful creation", func(t *testing.T) {
		mockDB.On("Exec", ctx, mock.AnythingOfType("string"),
			domain.ID, domain.UserID, domain.Domain, domain.PublicKey,
			domain.APIKey, domain.Verified, mock.Anything, domain.StorageEnabled,
			domain.CreatedAt, domain.UpdatedAt).Return(NewMockCommandTag(1), nil).Once()

		err := repo.Create(ctx, domain)

		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("with webhook config", func(t *testing.T) {
		d := &entities.Domain{
			ID:        uuid.Must(uuid.NewV4()),
			UserID:    uuid.Must(uuid.NewV4()),
			Domain:    "webhook.com",
			PublicKey: "test-key",
			APIKey:    "pm_webhook_key",
			WebhookConfig: &entities.WebhookConfig{
				URL:     "https://example.com/webhook",
				Enabled: true,
			},
			StorageEnabled: true,
			CreatedAt:      time.Now().UTC(),
			UpdatedAt:      time.Now().UTC(),
		}

		mockDB.On("Exec", ctx, mock.AnythingOfType("string"),
			mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return(NewMockCommandTag(1), nil).Once()

		err := repo.Create(ctx, d)

		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("database error", func(t *testing.T) {
		mockDB.On("Exec", ctx, mock.AnythingOfType("string"),
			mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return(pgconn.CommandTag{}, assert.AnError).Once()

		err := repo.Create(ctx, domain)

		assert.Error(t, err)
		mockDB.AssertExpectations(t)
	})
}

func TestDomainRepository_GetByID(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewDomainRepository(mockDB)
	ctx := context.Background()

	domainID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	t.Run("successful retrieval", func(t *testing.T) {
		d := &entities.Domain{
			ID:             domainID,
			UserID:         userID,
			Domain:         "example.com",
			PublicKey:      "test-key",
			APIKey:         "pm_api_key",
			Verified:       true,
			StorageEnabled: true,
			CreatedAt:      time.Now().UTC(),
			UpdatedAt:      time.Now().UTC(),
		}

		mockRow := &MockRow{
			data: []interface{}{
				d.ID,
				d.UserID,
				d.Domain,
				d.PublicKey,
				d.APIKey,
				d.Verified,
				[]byte{}, // Empty webhook config
				d.StorageEnabled,
				d.CreatedAt,
				d.UpdatedAt,
			},
		}

		mockDB.On("QueryRow", ctx, mock.AnythingOfType("string"), domainID).Return(mockRow).Once()

		result, err := repo.GetByID(ctx, domainID)

		assert.NoError(t, err)
		assert.Equal(t, d.ID, result.ID)
		assert.Equal(t, d.Domain, result.Domain)
		assert.Equal(t, d.Verified, result.Verified)

		mockDB.AssertExpectations(t)
	})

	t.Run("domain not found", func(t *testing.T) {
		mockRow := &MockRow{
			scanResult: pgx.ErrNoRows,
		}

		mockDB.On("QueryRow", ctx, mock.AnythingOfType("string"), domainID).Return(mockRow).Once()

		result, err := repo.GetByID(ctx, domainID)

		assert.Error(t, err)
		assert.Equal(t, sql.ErrNoRows, err)
		assert.Nil(t, result)

		mockDB.AssertExpectations(t)
	})

	t.Run("with webhook config", func(t *testing.T) {
		webhookConfig := &entities.WebhookConfig{
			URL:     "https://example.com/webhook",
			Enabled: true,
			Secret:  "webhook-secret",
		}

		webhookJSON, _ := json.Marshal(webhookConfig)

		mockRow := &MockRow{
			data: []interface{}{
				domainID,
				userID,
				"webhook.com",
				"test-key",
				"pm_webhook_key",
				false,
				webhookJSON,
				true,
				time.Now().UTC(),
				time.Now().UTC(),
			},
		}

		mockDB.On("QueryRow", ctx, mock.AnythingOfType("string"), domainID).Return(mockRow).Once()

		result, err := repo.GetByID(ctx, domainID)

		assert.NoError(t, err)
		assert.NotNil(t, result.WebhookConfig)
		assert.Equal(t, webhookConfig.URL, result.WebhookConfig.URL)
		assert.Equal(t, webhookConfig.Enabled, result.WebhookConfig.Enabled)

		mockDB.AssertExpectations(t)
	})
}

func TestDomainRepository_GetByUserID(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewDomainRepository(mockDB)
	ctx := context.Background()

	userID := uuid.Must(uuid.NewV4())

	t.Run("successful retrieval", func(t *testing.T) {
		domain1ID := uuid.Must(uuid.NewV4())
		domain2ID := uuid.Must(uuid.NewV4())

		mockRows := &MockRows{
			rows: [][]interface{}{
				{
					domain1ID,
					userID,
					"domain1.com",
					"key1",
					"pm_key1",
					true,
					[]byte{},
					true,
					time.Now().UTC(),
					time.Now().UTC(),
				},
				{
					domain2ID,
					userID,
					"domain2.com",
					"key2",
					"pm_key2",
					false,
					[]byte{},
					false,
					time.Now().UTC(),
					time.Now().UTC(),
				},
			},
		}

		mockDB.On("Query", ctx, mock.AnythingOfType("string"), userID).Return(mockRows, nil).Once()

		result, err := repo.GetByUserID(ctx, userID)

		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "domain1.com", result[0].Domain)
		assert.Equal(t, "domain2.com", result[1].Domain)

		mockDB.AssertExpectations(t)
	})

	t.Run("no domains found", func(t *testing.T) {
		mockRows := &MockRows{
			rows: [][]interface{}{},
		}

		mockDB.On("Query", ctx, mock.AnythingOfType("string"), userID).Return(mockRows, nil).Once()

		result, err := repo.GetByUserID(ctx, userID)

		assert.NoError(t, err)
		assert.Empty(t, result)

		mockDB.AssertExpectations(t)
	})

	t.Run("database error", func(t *testing.T) {
		mockDB.On("Query", ctx, mock.AnythingOfType("string"), userID).Return(nil, assert.AnError).Once()

		result, err := repo.GetByUserID(ctx, userID)

		assert.Error(t, err)
		assert.Nil(t, result)

		mockDB.AssertExpectations(t)
	})
}

func TestDomainRepository_Update(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewDomainRepository(mockDB)
	ctx := context.Background()

	domain := &entities.Domain{
		ID:             uuid.Must(uuid.NewV4()),
		UserID:         uuid.Must(uuid.NewV4()),
		Domain:         "updated.com",
		PublicKey:      "updated-key",
		APIKey:         "pm_updated_key",
		Verified:       true,
		StorageEnabled: false,
		UpdatedAt:      time.Now().UTC(),
	}

	t.Run("successful update", func(t *testing.T) {
		mockDB.On("Exec", ctx, mock.AnythingOfType("string"),
			domain.ID, domain.UserID, domain.Domain, domain.PublicKey,
			domain.APIKey, domain.Verified, mock.Anything, domain.StorageEnabled,
			domain.UpdatedAt).Return(NewMockCommandTag(1), nil).Once()

		err := repo.Update(ctx, domain)

		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("domain not found", func(t *testing.T) {
		mockDB.ExpectedCalls = nil
		mockDB.On("Exec", ctx, mock.AnythingOfType("string"),
			mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return(NewMockCommandTag(0), nil).Once()

		err := repo.Update(ctx, domain)

		assert.Error(t, err)
		assert.Equal(t, sql.ErrNoRows, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("database error", func(t *testing.T) {
		mockDB.ExpectedCalls = nil
		mockDB.On("Exec", ctx, mock.AnythingOfType("string"),
			mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return(pgconn.CommandTag{}, assert.AnError).Once()

		err := repo.Update(ctx, domain)

		assert.Error(t, err)
		mockDB.AssertExpectations(t)
	})
}

func TestDomainRepository_Delete(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewDomainRepository(mockDB)
	ctx := context.Background()

	domainID := uuid.Must(uuid.NewV4())

	t.Run("successful deletion", func(t *testing.T) {
		mockDB.On("Exec", ctx, mock.AnythingOfType("string"), domainID).Return(NewMockCommandTag(1), nil).Once()

		err := repo.Delete(ctx, domainID)

		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("domain not found", func(t *testing.T) {
		mockDB.On("Exec", ctx, mock.AnythingOfType("string"), domainID).Return(NewMockCommandTag(0), nil).Once()

		err := repo.Delete(ctx, domainID)

		assert.Error(t, err)
		assert.Equal(t, sql.ErrNoRows, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("database error", func(t *testing.T) {
		mockDB.On("Exec", ctx, mock.AnythingOfType("string"), domainID).Return(pgconn.CommandTag{}, assert.AnError).Once()

		err := repo.Delete(ctx, domainID)

		assert.Error(t, err)
		mockDB.AssertExpectations(t)
	})
}

func TestDomainRepository_GetByDomain(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewDomainRepository(mockDB)
	ctx := context.Background()

	domainName := "test.com"

	t.Run("successful retrieval", func(t *testing.T) {
		mockRow := &MockRow{
			data: []interface{}{
				uuid.Must(uuid.NewV4()),
				uuid.Must(uuid.NewV4()),
				domainName,
				"test-key",
				"pm_test_key",
				true,
				[]byte{},
				true,
				time.Now().UTC(),
				time.Now().UTC(),
			},
		}

		mockDB.On("QueryRow", ctx, mock.AnythingOfType("string"), domainName).Return(mockRow).Once()

		result, err := repo.GetByDomain(ctx, domainName)

		assert.NoError(t, err)
		assert.Equal(t, domainName, result.Domain)

		mockDB.AssertExpectations(t)
	})
}

func TestDomainRepository_GetByAPIKey(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewDomainRepository(mockDB)
	ctx := context.Background()

	apiKey := "pm_test_api_key"

	t.Run("successful retrieval", func(t *testing.T) {
		mockRow := &MockRow{
			data: []interface{}{
				uuid.Must(uuid.NewV4()),
				uuid.Must(uuid.NewV4()),
				"test.com",
				"test-key",
				apiKey,
				false,
				[]byte{},
				true,
				time.Now().UTC(),
				time.Now().UTC(),
			},
		}

		mockDB.On("QueryRow", ctx, mock.AnythingOfType("string"), apiKey).Return(mockRow).Once()

		result, err := repo.GetByAPIKey(ctx, apiKey)

		assert.NoError(t, err)
		assert.Equal(t, apiKey, result.APIKey)

		mockDB.AssertExpectations(t)
	})
}
