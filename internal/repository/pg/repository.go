package pg

import (
	"context"

	"mailsafe/domain/user"
	domain "mailsafe/domain/domain"
	"mailsafe/domain/email"

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
	db                     *pgxpool.Pool
	UserRepo               user.Repository
	DomainRepo             domain.Repository
	EmailAddressRepo       email.EmailAddressRepository
	ReceivedEmailRepo      email.ReceivedEmailRepository
}

// NewRepository creates a new Repository instance with all sub-repositories
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db:                     db,
		UserRepo:               NewUserRepository(db),
		DomainRepo:             NewDomainRepository(db),
		EmailAddressRepo:       NewEmailAddressRepository(db),
		ReceivedEmailRepo:      NewReceivedEmailRepository(db),
	}
}

// WithTx creates repository instances that use the provided transaction
func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{
		db:                     r.db,
		UserRepo:               NewUserRepository(tx),
		DomainRepo:             NewDomainRepository(tx),
		EmailAddressRepo:       NewEmailAddressRepository(tx),
		ReceivedEmailRepo:      NewReceivedEmailRepository(tx),
	}
}

// BeginTx starts a new transaction
func (r *Repository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.db.Begin(ctx)
}

