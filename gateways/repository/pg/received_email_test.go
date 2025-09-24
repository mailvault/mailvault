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

func setupReceivedEmailTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDSN())
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	return pool
}

func createUserDomainAndTwoEmails(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	now := time.Now().UTC()

	userID := uuid.Must(uuid.NewV4())
	_, err := pool.Exec(context.Background(), `
        INSERT INTO users (id, email, auth_provider, auth_provider_id, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6)
    `, userID, "recv_email_user@example.com", "supabase", "recvprov", now, now)
	require.NoError(t, err)

	domainID := uuid.Must(uuid.NewV4())
	_, err = pool.Exec(context.Background(), `
        INSERT INTO domains (id, user_id, domain, public_key, api_key, verification_status, storage_enabled, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `, domainID, userID, "recv.tld", "pubkey", "pm_recv_api_key", "verified", true, now, now)
	require.NoError(t, err)

	email1ID := uuid.Must(uuid.NewV4())
	_, err = pool.Exec(context.Background(), `
        INSERT INTO email_addresses (id, domain_id, local_part, forward_addresses, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6)
    `, email1ID, domainID, "alice", []string{}, now, now)
	require.NoError(t, err)

	email2ID := uuid.Must(uuid.NewV4())
	_, err = pool.Exec(context.Background(), `
        INSERT INTO email_addresses (id, domain_id, local_part, forward_addresses, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6)
    `, email2ID, domainID, "bob", []string{}, now, now)
	require.NoError(t, err)

	return email1ID, email2ID, domainID
}

func TestReceivedEmailRepository_CRUDAndSequence(t *testing.T) {
	pool := setupReceivedEmailTestPool(t)
	email1ID, email2ID, _ := createUserDomainAndTwoEmails(t, pool)
	repo := NewReceivedEmailRepository(pool)
	ctx := context.Background()

	// Insert three emails for email1 to validate sequence auto-increment
	subj1 := "Hello 1"
	e1 := &entities.ReceivedEmail{
		ID:             uuid.Must(uuid.NewV4()),
		EmailAddressID: &email1ID,
		FromAddress:    "sender1@example.com",
		Subject:        &subj1,
		EncryptedBody:  "enc1",
		ReceivedAt:     time.Now().UTC().Add(-3 * time.Minute),
	}
	require.NoError(t, repo.Create(ctx, e1))
	assert.Equal(t, 1, e1.SequenceNumber)

	var subj2 *string // nil subject
	e2 := &entities.ReceivedEmail{
		ID:             uuid.Must(uuid.NewV4()),
		EmailAddressID: &email1ID,
		FromAddress:    "sender2@example.com",
		Subject:        subj2,
		EncryptedBody:  "enc2",
		ReceivedAt:     time.Now().UTC().Add(-2 * time.Minute),
	}
	require.NoError(t, repo.Create(ctx, e2))
	assert.Equal(t, 2, e2.SequenceNumber)

	subj3 := "Hello 3"
	e3 := &entities.ReceivedEmail{
		ID:             uuid.Must(uuid.NewV4()),
		EmailAddressID: &email1ID,
		FromAddress:    "sender3@example.com",
		Subject:        &subj3,
		EncryptedBody:  "enc3",
		ReceivedAt:     time.Now().UTC().Add(-1 * time.Minute),
	}
	require.NoError(t, repo.Create(ctx, e3))
	assert.Equal(t, 3, e3.SequenceNumber)

	// Insert first email for email2; its sequence must start at 1 independently
	eOther := &entities.ReceivedEmail{
		ID:             uuid.Must(uuid.NewV4()),
		EmailAddressID: &email2ID,
		FromAddress:    "someone@example.com",
		EncryptedBody:  "encx",
		ReceivedAt:     time.Now().UTC(),
	}
	require.NoError(t, repo.Create(ctx, eOther))
	assert.Equal(t, 1, eOther.SequenceNumber)

	// Count
	cnt1, err := repo.Count(ctx, email1ID)
	require.NoError(t, err)
	assert.Equal(t, int64(3), cnt1)
	cnt2, err := repo.Count(ctx, email2ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), cnt2)

	// GetByID
	got, err := repo.GetByID(ctx, e2.ID)
	require.NoError(t, err)
	assert.Equal(t, e2.ID, got.ID)
	require.NotNil(t, got.EmailAddressID)
	assert.Equal(t, email1ID, *got.EmailAddressID)
	assert.Equal(t, 2, got.SequenceNumber)
	assert.Equal(t, "enc2", got.EncryptedBody)

	// List by email address with ordering and pagination (DESC by sequence)
	list, err := repo.GetByEmailAddressID(ctx, email1ID, 2, 0)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, 3, list[0].SequenceNumber)
	assert.Equal(t, 2, list[1].SequenceNumber)

	list2, err := repo.GetByEmailAddressID(ctx, email1ID, 2, 2)
	require.NoError(t, err)
	require.Len(t, list2, 1)
	assert.Equal(t, 1, list2[0].SequenceNumber)

	// Delete and ensure it is gone
	require.NoError(t, repo.Delete(ctx, e2.ID))
	_, err = repo.GetByID(ctx, e2.ID)
	require.Error(t, err)
	assert.Equal(t, sql.ErrNoRows, err)

	cnt1After, err := repo.Count(ctx, email1ID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), cnt1After)
}
