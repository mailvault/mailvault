package user

import (
	"context"

	"mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
)

//go:generate moq -skip-ensure -stub -pkg mocks -out mocks/repository.go . Repository
type Repository interface {
	Create(ctx context.Context, user *entities.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*entities.User, error)
	GetByEmail(ctx context.Context, email string) (*entities.User, error)
	GetByAuthProvider(ctx context.Context, provider, providerID string) (*entities.User, error)
	Update(ctx context.Context, user *entities.User) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, page, pageSize int) ([]entities.User, int64, error)
	SearchUsers(ctx context.Context, page, pageSize int, search, accountType string) ([]entities.User, int64, error)
}
