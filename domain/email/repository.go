package email

import (
	"context"

	"privatemail/domain/entities"

	"github.com/gofrs/uuid/v5"
)

type EmailAddressRepository interface {
	Create(ctx context.Context, emailAddress *entities.EmailAddress) error
	GetByID(ctx context.Context, id uuid.UUID) (*entities.EmailAddress, error)
	GetByDomainID(ctx context.Context, domainID uuid.UUID) ([]*entities.EmailAddress, error)
	GetByLocalPartAndDomain(ctx context.Context, localPart string, domainID uuid.UUID) (*entities.EmailAddress, error)
	GetCatchAllByDomainID(ctx context.Context, domainID uuid.UUID) (*entities.EmailAddress, error)
	Update(ctx context.Context, emailAddress *entities.EmailAddress) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type ReceivedEmailRepository interface {
	Create(ctx context.Context, email *entities.ReceivedEmail) error
	GetByID(ctx context.Context, id uuid.UUID) (*entities.ReceivedEmail, error)
	GetByEmailAddressID(ctx context.Context, emailAddressID uuid.UUID, limit, offset int) ([]*entities.ReceivedEmail, error)
	Count(ctx context.Context, emailAddressID uuid.UUID) (int64, error)
	Delete(ctx context.Context, id uuid.UUID) error
}