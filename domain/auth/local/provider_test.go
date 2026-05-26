package local

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/mailvault/mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memCredsRepo is an in-memory CredentialsRepo for tests.
type memCredsRepo struct {
	mu    sync.Mutex
	store map[uuid.UUID]*Credentials
}

func newMemCredsRepo() *memCredsRepo {
	return &memCredsRepo{store: map[uuid.UUID]*Credentials{}}
}

func (m *memCredsRepo) Create(_ context.Context, userID uuid.UUID, hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[userID] = &Credentials{
		UserID:         userID,
		PasswordHash:   hash,
		EmailConfirmed: true,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	return nil
}

func (m *memCredsRepo) GetByUserID(_ context.Context, userID uuid.UUID) (*Credentials, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.store[userID]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return c, nil
}

func (m *memCredsRepo) Delete(_ context.Context, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.store, userID)
	return nil
}

// memUserStore is an in-memory UserStore for tests.
type memUserStore struct {
	mu    sync.Mutex
	byID  map[uuid.UUID]*entities.User
	byEml map[string]*entities.User
}

func newMemUserStore() *memUserStore {
	return &memUserStore{
		byID:  map[uuid.UUID]*entities.User{},
		byEml: map[string]*entities.User{},
	}
}

func (m *memUserStore) Create(_ context.Context, u *entities.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byID[u.ID] = u
	m.byEml[u.Email] = u
	return nil
}

func (m *memUserStore) GetByID(_ context.Context, id uuid.UUID) (*entities.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.byID[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return u, nil
}

func (m *memUserStore) GetByEmail(_ context.Context, email string) (*entities.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.byEml[email]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return u, nil
}

const testSecret = "test-secret-key-32-bytes-minimum-aaaa"

func newTestProvider(t *testing.T) (*Provider, *memCredsRepo, *memUserStore) {
	t.Helper()
	creds := newMemCredsRepo()
	users := newMemUserStore()
	p, err := NewProvider(creds, users, testSecret, time.Hour)
	require.NoError(t, err)
	return p, creds, users
}

func TestNewProvider_ValidatesSecret(t *testing.T) {
	_, err := NewProvider(newMemCredsRepo(), newMemUserStore(), "too-short", time.Hour)
	assert.Error(t, err)
	_, err = NewProvider(newMemCredsRepo(), newMemUserStore(), testSecret, 0)
	assert.Error(t, err)
}

func TestCreateUser_HappyPath(t *testing.T) {
	p, creds, users := newTestProvider(t)
	ctx := context.Background()

	resp, err := p.CreateUser(ctx, "  Alice@Example.com ", "hunter22!")
	require.NoError(t, err)
	assert.NotEmpty(t, resp.UserID)
	assert.False(t, resp.RequiresConfirm)
	assert.NotEmpty(t, resp.AccessToken)

	// User row was created with normalized email and local provider tag.
	uid, _ := uuid.FromString(resp.UserID)
	u, err := users.GetByID(ctx, uid)
	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", u.Email)
	assert.Equal(t, ProviderName, u.AuthProvider)
	assert.Equal(t, entities.UserPlanFree, u.UserPlan)

	// Credentials row was persisted (hash != plaintext).
	c, err := creds.GetByUserID(ctx, uid)
	require.NoError(t, err)
	assert.NotEqual(t, "hunter22!", c.PasswordHash)
	assert.True(t, c.EmailConfirmed)
}

func TestCreateUser_DuplicateEmailRejected(t *testing.T) {
	p, _, _ := newTestProvider(t)
	ctx := context.Background()

	_, err := p.CreateUser(ctx, "dup@example.com", "hunter22!")
	require.NoError(t, err)
	_, err = p.CreateUser(ctx, "dup@example.com", "different!")
	assert.ErrorContains(t, err, "already exists")
}

func TestCreateUser_ShortPasswordRejected(t *testing.T) {
	p, _, _ := newTestProvider(t)
	_, err := p.CreateUser(context.Background(), "x@y.com", "short")
	assert.ErrorContains(t, err, "at least 8")
}

func TestLogin_HappyPath(t *testing.T) {
	p, _, _ := newTestProvider(t)
	ctx := context.Background()
	_, err := p.CreateUser(ctx, "bob@example.com", "hunter22!")
	require.NoError(t, err)

	token, err := p.Login(ctx, "BOB@example.com", "hunter22!")
	require.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestLogin_WrongPassword(t *testing.T) {
	p, _, _ := newTestProvider(t)
	ctx := context.Background()
	_, err := p.CreateUser(ctx, "carol@example.com", "hunter22!")
	require.NoError(t, err)

	_, err = p.Login(ctx, "carol@example.com", "wrong-pass")
	assert.ErrorContains(t, err, "invalid email or password")
}

func TestLogin_UnknownEmail(t *testing.T) {
	p, _, _ := newTestProvider(t)
	_, err := p.Login(context.Background(), "ghost@example.com", "hunter22!")
	assert.ErrorContains(t, err, "invalid email or password")
}

func TestLogin_RejectsNonLocalProviderUser(t *testing.T) {
	p, _, users := newTestProvider(t)
	ctx := context.Background()

	// Pre-existing Supabase-authenticated user; no local credentials.
	supaUser := &entities.User{
		ID:             uuid.Must(uuid.NewV4()),
		Email:          "supa@example.com",
		AuthProvider:   "supabase",
		AuthProviderID: "supa-id",
		AccountType:    entities.AccountTypeUser,
		UserPlan:       entities.UserPlanFree,
	}
	require.NoError(t, users.Create(ctx, supaUser))

	_, err := p.Login(ctx, "supa@example.com", "anything")
	assert.ErrorContains(t, err, "invalid email or password")
}

func TestValidateToken_RoundTrip(t *testing.T) {
	p, _, _ := newTestProvider(t)
	ctx := context.Background()

	resp, err := p.CreateUser(ctx, "dora@example.com", "hunter22!")
	require.NoError(t, err)

	user, err := p.ValidateToken(ctx, resp.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "dora@example.com", user.Email)
}

func TestValidateToken_RejectsTampered(t *testing.T) {
	p, _, _ := newTestProvider(t)
	ctx := context.Background()

	resp, err := p.CreateUser(ctx, "eric@example.com", "hunter22!")
	require.NoError(t, err)

	// Flip a character in the signature segment.
	tampered := resp.AccessToken[:len(resp.AccessToken)-2] + "AA"
	_, err = p.ValidateToken(ctx, tampered)
	assert.Error(t, err)
}

func TestValidateToken_RejectsExpired(t *testing.T) {
	creds := newMemCredsRepo()
	users := newMemUserStore()
	p, err := NewProvider(creds, users, testSecret, time.Millisecond)
	require.NoError(t, err)

	ctx := context.Background()
	resp, err := p.CreateUser(ctx, "fae@example.com", "hunter22!")
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	_, err = p.ValidateToken(ctx, resp.AccessToken)
	assert.Error(t, err)
}

func TestGetUserByID(t *testing.T) {
	p, _, _ := newTestProvider(t)
	ctx := context.Background()
	resp, err := p.CreateUser(ctx, "g@x.com", "hunter22!")
	require.NoError(t, err)
	u, err := p.GetUserByID(ctx, resp.UserID)
	require.NoError(t, err)
	assert.Equal(t, "g@x.com", u.Email)
}

func TestEmailConfirmStubs(t *testing.T) {
	p, _, _ := newTestProvider(t)
	assert.NoError(t, p.ResendConfirmation(context.Background(), "x@y.com"))
	tok, err := p.ConfirmEmail(context.Background(), "tok", "x@y.com")
	assert.NoError(t, err)
	assert.Empty(t, tok)
}

func TestProviderName(t *testing.T) {
	p, _, _ := newTestProvider(t)
	assert.Equal(t, "local", p.Provider())
}
