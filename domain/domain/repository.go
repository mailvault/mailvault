package domain

import (
	"context"

	"mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
)

//go:generate moq -skip-ensure -stub -pkg mocks -out mocks/repository.go . Repository

type Repository interface {
	Create(ctx context.Context, domain *entities.Domain) error
	GetByID(ctx context.Context, id uuid.UUID) (*entities.Domain, error)
	GetByDomain(ctx context.Context, domain string) (*entities.Domain, error)
	GetByAPIKey(ctx context.Context, apiKey string) (*entities.Domain, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]*entities.Domain, error)
	Update(ctx context.Context, domain *entities.Domain) error
	Delete(ctx context.Context, id uuid.UUID) error
}
