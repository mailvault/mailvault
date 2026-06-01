package webhook_config_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/mailvault/mailvault/domain/entities"
	"github.com/mailvault/mailvault/domain/webhook_config"
	"github.com/mailvault/mailvault/domain/webhook_config/mocks"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubDomainRepo is a tiny in-test stand-in for webhook_config.DomainRepository
// (no moq generated for it, since the surface is one method).
type stubDomainRepo struct {
	getByID func(ctx context.Context, id uuid.UUID) (*entities.Domain, error)
}

func (s *stubDomainRepo) GetByID(ctx context.Context, id uuid.UUID) (*entities.Domain, error) {
	return s.getByID(ctx, id)
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func ownerOf(domainID uuid.UUID, userID uuid.UUID) *stubDomainRepo {
	return &stubDomainRepo{
		getByID: func(_ context.Context, id uuid.UUID) (*entities.Domain, error) {
			if id != domainID {
				return nil, errors.New("not found")
			}
			return &entities.Domain{ID: domainID, UserID: userID}, nil
		},
	}
}

func validCreateInput(domainID, userID uuid.UUID) webhook_config.CreateWebhookConfigInput {
	return webhook_config.CreateWebhookConfigInput{
		DomainID: domainID,
		UserID:   userID,
		Name:     "main",
		URL:      "https://hooks.example.test/incoming",
		AuthType: entities.WebhookAuthTypeNone,
	}
}

func TestCreate_HappyPath_PersistsConfigAndAudit(t *testing.T) {
	domainID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	var createdConfig *entities.WebhookConfiguration
	var createdAudit *entities.WebhookConfigurationAudit
	repo := &mocks.RepositoryMock{
		GetByDomainIDAndNameFunc: func(_ context.Context, _ uuid.UUID, _ string) (*entities.WebhookConfiguration, error) {
			return nil, errors.New("not found") // unique name
		},
		CreateFunc: func(_ context.Context, c *entities.WebhookConfiguration) error {
			createdConfig = c
			return nil
		},
		CreateAuditFunc: func(_ context.Context, a *entities.WebhookConfigurationAudit) error {
			createdAudit = a
			return nil
		},
	}
	uc := webhook_config.NewUseCase(repo, ownerOf(domainID, userID), discardLogger())

	got, err := uc.CreateWebhookConfiguration(context.Background(), validCreateInput(domainID, userID))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, createdConfig)
	assert.Equal(t, domainID, createdConfig.DomainID)
	assert.True(t, createdConfig.Enabled, "new configs should be enabled by default")
	assert.Equal(t, "POST", createdConfig.Method, "Method defaults to POST")
	assert.Equal(t, 30, createdConfig.TimeoutSeconds, "TimeoutSeconds default is 30")
	assert.Equal(t, 3, createdConfig.MaxRetries, "MaxRetries default is 3")
	require.NotNil(t, createdAudit)
	assert.Equal(t, "created", createdAudit.Action)
}

func TestCreate_UnauthorizedDomain_Rejected(t *testing.T) {
	domainID := uuid.Must(uuid.NewV4())
	owner := uuid.Must(uuid.NewV4())
	otherUser := uuid.Must(uuid.NewV4())

	repo := &mocks.RepositoryMock{
		CreateFunc: func(_ context.Context, _ *entities.WebhookConfiguration) error {
			t.Fatal("Create must not be called when domain ownership check fails")
			return nil
		},
	}
	uc := webhook_config.NewUseCase(repo, ownerOf(domainID, owner), discardLogger())

	in := validCreateInput(domainID, otherUser) // different user
	_, err := uc.CreateWebhookConfiguration(context.Background(), in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestCreate_DuplicateNameRejected(t *testing.T) {
	domainID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	repo := &mocks.RepositoryMock{
		GetByDomainIDAndNameFunc: func(_ context.Context, _ uuid.UUID, _ string) (*entities.WebhookConfiguration, error) {
			// existing config with same name → duplicate
			return &entities.WebhookConfiguration{ID: uuid.Must(uuid.NewV4()), Name: "main"}, nil
		},
		CreateFunc: func(_ context.Context, _ *entities.WebhookConfiguration) error {
			t.Fatal("Create must not be called when duplicate name detected")
			return nil
		},
	}
	uc := webhook_config.NewUseCase(repo, ownerOf(domainID, userID), discardLogger())

	_, err := uc.CreateWebhookConfiguration(context.Background(), validCreateInput(domainID, userID))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestGet_HappyPath_VerifiesOwnership(t *testing.T) {
	domainID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	configID := uuid.Must(uuid.NewV4())

	repo := &mocks.RepositoryMock{
		GetByIDFunc: func(_ context.Context, _ uuid.UUID) (*entities.WebhookConfiguration, error) {
			return &entities.WebhookConfiguration{ID: configID, DomainID: domainID, Name: "main"}, nil
		},
	}
	uc := webhook_config.NewUseCase(repo, ownerOf(domainID, userID), discardLogger())

	got, err := uc.GetWebhookConfiguration(context.Background(), configID, userID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, configID, got.ID)
}

func TestGet_RejectsAcrossUsers(t *testing.T) {
	domainID := uuid.Must(uuid.NewV4())
	owner := uuid.Must(uuid.NewV4())
	other := uuid.Must(uuid.NewV4())
	configID := uuid.Must(uuid.NewV4())

	repo := &mocks.RepositoryMock{
		GetByIDFunc: func(_ context.Context, _ uuid.UUID) (*entities.WebhookConfiguration, error) {
			return &entities.WebhookConfiguration{ID: configID, DomainID: domainID}, nil
		},
	}
	uc := webhook_config.NewUseCase(repo, ownerOf(domainID, owner), discardLogger())

	_, err := uc.GetWebhookConfiguration(context.Background(), configID, other)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestList_ScopesToDomainOwner(t *testing.T) {
	domainID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	repo := &mocks.RepositoryMock{
		GetByDomainIDFunc: func(_ context.Context, id uuid.UUID) ([]*entities.WebhookConfiguration, error) {
			assert.Equal(t, domainID, id, "List must scope by domain")
			return []*entities.WebhookConfiguration{
				{ID: uuid.Must(uuid.NewV4()), DomainID: domainID, Name: "main"},
				{ID: uuid.Must(uuid.NewV4()), DomainID: domainID, Name: "backup"},
			}, nil
		},
	}
	uc := webhook_config.NewUseCase(repo, ownerOf(domainID, userID), discardLogger())

	got, err := uc.ListWebhookConfigurations(context.Background(), domainID, userID)
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestEnable_FlipsEnabledAndWritesAudit(t *testing.T) {
	domainID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	configID := uuid.Must(uuid.NewV4())

	original := &entities.WebhookConfiguration{ID: configID, DomainID: domainID, Name: "main", Enabled: false}
	var updated *entities.WebhookConfiguration
	var audit *entities.WebhookConfigurationAudit
	repo := &mocks.RepositoryMock{
		GetByIDFunc: func(_ context.Context, _ uuid.UUID) (*entities.WebhookConfiguration, error) { return original, nil },
		UpdateFunc: func(_ context.Context, c *entities.WebhookConfiguration) error {
			updated = c
			return nil
		},
		CreateAuditFunc: func(_ context.Context, a *entities.WebhookConfigurationAudit) error {
			audit = a
			return nil
		},
	}
	uc := webhook_config.NewUseCase(repo, ownerOf(domainID, userID), discardLogger())

	require.NoError(t, uc.EnableWebhookConfiguration(context.Background(), configID, userID))
	require.NotNil(t, updated)
	assert.True(t, updated.Enabled, "Enable must flip the flag")
	require.NotNil(t, audit)
	assert.Equal(t, "enabled", audit.Action)
}

func TestDisable_FlipsEnabledAndWritesAudit(t *testing.T) {
	domainID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	configID := uuid.Must(uuid.NewV4())

	original := &entities.WebhookConfiguration{ID: configID, DomainID: domainID, Name: "main", Enabled: true}
	var updated *entities.WebhookConfiguration
	var audit *entities.WebhookConfigurationAudit
	repo := &mocks.RepositoryMock{
		GetByIDFunc: func(_ context.Context, _ uuid.UUID) (*entities.WebhookConfiguration, error) { return original, nil },
		UpdateFunc: func(_ context.Context, c *entities.WebhookConfiguration) error {
			updated = c
			return nil
		},
		CreateAuditFunc: func(_ context.Context, a *entities.WebhookConfigurationAudit) error {
			audit = a
			return nil
		},
	}
	uc := webhook_config.NewUseCase(repo, ownerOf(domainID, userID), discardLogger())

	require.NoError(t, uc.DisableWebhookConfiguration(context.Background(), configID, userID))
	require.NotNil(t, updated)
	assert.False(t, updated.Enabled, "Disable must clear the flag")
	require.NotNil(t, audit)
	assert.Equal(t, "disabled", audit.Action)
}

func TestDelete_RemovesConfigAndWritesAudit(t *testing.T) {
	domainID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	configID := uuid.Must(uuid.NewV4())

	var deletedID uuid.UUID
	var audit *entities.WebhookConfigurationAudit
	repo := &mocks.RepositoryMock{
		GetByIDFunc: func(_ context.Context, _ uuid.UUID) (*entities.WebhookConfiguration, error) {
			return &entities.WebhookConfiguration{ID: configID, DomainID: domainID, Name: "main"}, nil
		},
		DeleteFunc: func(_ context.Context, id uuid.UUID) error {
			deletedID = id
			return nil
		},
		CreateAuditFunc: func(_ context.Context, a *entities.WebhookConfigurationAudit) error {
			audit = a
			return nil
		},
	}
	uc := webhook_config.NewUseCase(repo, ownerOf(domainID, userID), discardLogger())

	require.NoError(t, uc.DeleteWebhookConfiguration(context.Background(), configID, userID))
	assert.Equal(t, configID, deletedID)
	require.NotNil(t, audit)
	assert.Equal(t, "deleted", audit.Action)
}
