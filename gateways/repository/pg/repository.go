package pg

import (
	"context"

	domain "mailvault/domain/domain"
	"mailvault/domain/email"
	"mailvault/domain/email_provider"
	"mailvault/domain/email_sending"
	"mailvault/domain/user"
	"mailvault/domain/validation"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX represents a database transaction interface
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

// Repository aggregates all repositories and provides transaction support
type Repository struct {
	db                    *pgxpool.Pool
	UserRepo              user.Repository
	DomainRepo            domain.Repository
	EmailAddressRepo      email.EmailAddressRepository
	ReceivedEmailRepo     email.ReceivedEmailRepository
	SentEmailRepo         email_sending.Repository
	ValidationRepo        validation.Repository
	EmailProviderRepo     email_provider.Repository
	EmailProviderLogRepo  email_provider.LogRepository
}

// NewRepository creates a new Repository instance with all sub-repositories
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db:                   db,
		UserRepo:             NewUserRepository(db),
		DomainRepo:           NewDomainRepository(db),
		EmailAddressRepo:     NewEmailAddressRepository(db),
		ReceivedEmailRepo:    NewReceivedEmailRepository(db),
		SentEmailRepo:        NewSentEmailRepository(db),
		ValidationRepo:       NewValidationRepository(db),
		EmailProviderRepo:    NewEmailProviderRepository(db),
		EmailProviderLogRepo: NewEmailProviderLogRepository(db),
	}
}

// WithTx creates repository instances that use the provided transaction
func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{
		db:                   r.db,
		UserRepo:             NewUserRepository(tx),
		DomainRepo:           NewDomainRepository(tx),
		EmailAddressRepo:     NewEmailAddressRepository(tx),
		ReceivedEmailRepo:    NewReceivedEmailRepository(tx),
		SentEmailRepo:        NewSentEmailRepository(tx),
		ValidationRepo:       NewValidationRepository(tx),
		EmailProviderRepo:    NewEmailProviderRepository(tx),
		EmailProviderLogRepo: NewEmailProviderLogRepository(tx),
	}
}

// BeginTx starts a new transaction
func (r *Repository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.db.Begin(ctx)
}

// DB exposes the underlying connection pool as a DBTX for read-only queries
func (r *Repository) DB() DBTX {
	return r.db
}
