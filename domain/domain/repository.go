package domain

import (
	"context"

	"mailsafe/domain/entities"

	"github.com/gofrs/uuid/v5"
)

type Repository interface {
	Create(ctx context.Context, domain *entities.Domain) error
	GetByID(ctx context.Context, id uuid.UUID) (*entities.Domain, error)
	GetByDomain(ctx context.Context, domain string) (*entities.Domain, error)
	GetByAPIKey(ctx context.Context, apiKey string) (*entities.Domain, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]*entities.Domain, error)
	Update(ctx context.Context, domain *entities.Domain) error
	Delete(ctx context.Context, id uuid.UUID) error
}