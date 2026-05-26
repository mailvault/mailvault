package pg

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/mailvault/mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupUserTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDSN())
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestUserRepository_CRUD(t *testing.T) {
	pool := setupUserTestPool(t)
	repo := NewUserRepository(pool)
	ctx := context.Background()

	u := &entities.User{
		ID:             uuid.Must(uuid.NewV4()),
		Email:          "user_test@example.com",
		AuthProvider:   "supabase",
		AuthProviderID: "userprov",
		AccountType:    entities.AccountTypeUser,
		UserPlan:       entities.UserPlanFree,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	// Create
	require.NoError(t, repo.Create(ctx, u))

	// GetByID
	got, err := repo.GetByID(ctx, u.ID)
	require.NoError(t, err)
	assert.Equal(t, u.Email, got.Email)

	// GetByEmail
	got2, err := repo.GetByEmail(ctx, u.Email)
	require.NoError(t, err)
	assert.Equal(t, u.ID, got2.ID)

	// GetByAuthProvider
	got3, err := repo.GetByAuthProvider(ctx, u.AuthProvider, u.AuthProviderID)
	require.NoError(t, err)
	assert.Equal(t, u.ID, got3.ID)

	// Update
	u.Email = "updated_user@example.com"
	u.AuthProvider = "firebase"
	u.AuthProviderID = "firebaseprov"
	u.UpdatedAt = time.Now().UTC()
	require.NoError(t, repo.Update(ctx, u))

	got4, err := repo.GetByID(ctx, u.ID)
	require.NoError(t, err)
	assert.Equal(t, "updated_user@example.com", got4.Email)
	assert.Equal(t, "firebase", got4.AuthProvider)

	// Delete
	require.NoError(t, repo.Delete(ctx, u.ID))
	_, err = repo.GetByID(ctx, u.ID)
	require.Error(t, err)
	assert.Equal(t, sql.ErrNoRows, err)
}
