package email

import (
	"context"

	"mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
)

//go:generate moq -skip-ensure -stub -pkg mocks -out mocks/email_address_repository.go . EmailAddressRepository
//go:generate moq -skip-ensure -stub -pkg mocks -out mocks/received_email_repository.go . ReceivedEmailRepository
//go:generate moq -skip-ensure -stub -pkg mocks -out mocks/domain_repository.go . DomainRepository

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
	GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int, domain string) ([]*entities.ReceivedEmail, int, error)
	Count(ctx context.Context, emailAddressID uuid.UUID) (int64, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type DomainRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*entities.Domain, error)
	GetByDomain(ctx context.Context, domain string) (*entities.Domain, error)
}
