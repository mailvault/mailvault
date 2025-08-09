package pg

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"mailsafe/domain/entities"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupEmailTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDSN())
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	return pool
}

func createEmailTestUserAndDomain(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	t.Helper()
	userID := uuid.Must(uuid.NewV4())
	now := time.Now().UTC()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO users (id, email, auth_provider, auth_provider_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, userID, "email_test@example.com", "supabase", "emailprov", now, now)
	require.NoError(t, err)

	domainID := uuid.Must(uuid.NewV4())
	_, err = pool.Exec(context.Background(), `
		INSERT INTO domains (id, user_id, domain, public_key, api_key, verified, storage_enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, domainID, userID, "addr.com", "pub", "pm_email_api_key", false, true, now, now)
	require.NoError(t, err)

	return userID, domainID
}

func TestEmailAddressRepository_CRUD(t *testing.T) {
	pool := setupEmailTestPool(t)
	_, domainID := createEmailTestUserAndDomain(t, pool)
	repo := NewEmailAddressRepository(pool)
	ctx := context.Background()

	e := &entities.EmailAddress{
		ID:               uuid.Must(uuid.NewV4()),
		DomainID:         domainID,
		LocalPart:        "john",
		IsCatchAll:       false,
		ForwardAddresses: []string{"fwd@x.tld"},
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	// Create
	require.NoError(t, repo.Create(ctx, e))

	// GetByID
	got, err := repo.GetByID(ctx, e.ID)
	require.NoError(t, err)
	assert.Equal(t, e.LocalPart, got.LocalPart)

	// GetByDomainID
	list, err := repo.GetByDomainID(ctx, domainID)
	require.NoError(t, err)
	require.NotEmpty(t, list)

	// GetByLocalPartAndDomain
	got2, err := repo.GetByLocalPartAndDomain(ctx, "john", domainID)
	require.NoError(t, err)
	assert.Equal(t, e.ID, got2.ID)

	// Catch-all none yet
	_, err = repo.GetCatchAllByDomainID(ctx, domainID)
	require.Error(t, err)
	assert.Equal(t, sql.ErrNoRows, err)

	// Create catch-all
	catch := &entities.EmailAddress{
		ID:         uuid.Must(uuid.NewV4()),
		DomainID:   domainID,
		LocalPart:  "*",
		IsCatchAll: true,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	require.NoError(t, repo.Create(ctx, catch))

	got3, err := repo.GetCatchAllByDomainID(ctx, domainID)
	require.NoError(t, err)
	assert.True(t, got3.IsCatchAll)

	// Update
	e.ForwardAddresses = []string{"new@x.tld"}
	e.IsCatchAll = false
	e.UpdatedAt = time.Now().UTC()
	require.NoError(t, repo.Update(ctx, e))

	// Delete
	require.NoError(t, repo.Delete(ctx, e.ID))
	_, err = repo.GetByID(ctx, e.ID)
	require.Error(t, err)
	assert.Equal(t, sql.ErrNoRows, err)
}
