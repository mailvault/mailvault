package pg

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupDomainTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDSN())
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	return pool
}

func createDomainTestUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV4())
	now := time.Now().UTC()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO users (id, email, auth_provider, auth_provider_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, id, "domain_test@example.com", "supabase", "domainprov", now, now)
	require.NoError(t, err)
	return id
}

func TestDomainRepository_CRUD(t *testing.T) {
	pool := setupDomainTestPool(t)
	repo := NewDomainRepository(pool)
	ctx := context.Background()

	userID := createDomainTestUser(t, pool)

	d := &entities.Domain{
		ID:               uuid.Must(uuid.NewV4()),
		UserID:           userID,
		Domain:           "myexample.com",
		PublicKey:        "pubkey",
		APIKey:           "pm_domain_api_key",
		Verified:         false,
		StorageEnabled:   true,
		AutoCreateAddress: false,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	// Create
	require.NoError(t, repo.Create(ctx, d))

	// GetByID
	got, err := repo.GetByID(ctx, d.ID)
	require.NoError(t, err)
	assert.Equal(t, d.Domain, got.Domain)
	assert.False(t, got.Verified)
	assert.False(t, got.AutoCreateAddress)

	// GetByDomain
	got2, err := repo.GetByDomain(ctx, d.Domain)
	require.NoError(t, err)
	assert.Equal(t, d.ID, got2.ID)

	// GetByAPIKey
	got3, err := repo.GetByAPIKey(ctx, d.APIKey)
	require.NoError(t, err)
	assert.Equal(t, d.ID, got3.ID)

	// GetByUserID
	list, err := repo.GetByUserID(ctx, userID)
	require.NoError(t, err)
	require.NotEmpty(t, list)

	// Update
	d.Verified = true
	d.StorageEnabled = false
	d.AutoCreateAddress = true
	d.UpdatedAt = time.Now().UTC()
	d.WebhookConfig = &entities.WebhookConfig{URL: "https://hook.tld", Enabled: true}
	require.NoError(t, repo.Update(ctx, d))

	got4, err := repo.GetByID(ctx, d.ID)
	require.NoError(t, err)
	assert.True(t, got4.Verified)
	assert.False(t, got4.StorageEnabled)
	assert.True(t, got4.AutoCreateAddress)
	require.NotNil(t, got4.WebhookConfig)
	assert.Equal(t, "https://hook.tld", got4.WebhookConfig.URL)

	// Delete
	require.NoError(t, repo.Delete(ctx, d.ID))
	_, err = repo.GetByID(ctx, d.ID)
	require.Error(t, err)
	assert.Equal(t, sql.ErrNoRows, err)
}
