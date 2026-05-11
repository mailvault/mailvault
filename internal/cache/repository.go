package cache

import (
	"context"
	"fmt"
	"time"

	"mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
)

// RepositoryCache provides caching for database operations
type RepositoryCache struct {
	cache  Cache
	config CacheConfig
}

// CacheConfig holds repository caching configuration
type CacheConfig struct {
	// TTL for different entity types
	UserTTL          time.Duration
	DomainTTL        time.Duration
	EmailAddressTTL  time.Duration
	ReceivedEmailTTL time.Duration

	// Enable/disable caching for different entity types
	CacheUsers          bool
	CacheDomains        bool
	CacheEmailAddresses bool
	CacheReceivedEmails bool

	// Cache frequently accessed queries
	CacheQueries bool
	QueryTTL     time.Duration
}

// DefaultCacheConfig returns production-ready cache configuration
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		UserTTL:             30 * time.Minute, // Users don't change often
		DomainTTL:           15 * time.Minute, // Domains change occasionally
		EmailAddressTTL:     10 * time.Minute, // Email addresses change more often
		ReceivedEmailTTL:    5 * time.Minute,  // Recent emails for quick access
		CacheUsers:          true,
		CacheDomains:        true,
		CacheEmailAddresses: true,
		CacheReceivedEmails: false, // Don't cache received emails by default
		CacheQueries:        true,
		QueryTTL:            5 * time.Minute,
	}
}

// NewRepositoryCache creates a new repository cache
func NewRepositoryCache(cache Cache, config CacheConfig) *RepositoryCache {
	return &RepositoryCache{
		cache:  cache,
		config: config,
	}
}

// User caching methods

func (rc *RepositoryCache) CacheUser(ctx context.Context, user *entities.User) error {
	if !rc.config.CacheUsers || rc.cache == nil {
		return nil
	}

	key := fmt.Sprintf("user:id:%s", user.ID.String())
	return rc.cache.Set(ctx, key, user, rc.config.UserTTL)
}

func (rc *RepositoryCache) GetCachedUser(ctx context.Context, userID uuid.UUID) (*entities.User, error) {
	if !rc.config.CacheUsers || rc.cache == nil {
		return nil, ErrCacheMiss
	}

	key := fmt.Sprintf("user:id:%s", userID.String())
	var user entities.User
	err := rc.cache.Get(ctx, key, &user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (rc *RepositoryCache) CacheUserByEmail(ctx context.Context, email string, user *entities.User) error {
	if !rc.config.CacheUsers || rc.cache == nil {
		return nil
	}

	key := fmt.Sprintf("user:email:%s", email)
	return rc.cache.Set(ctx, key, user, rc.config.UserTTL)
}

func (rc *RepositoryCache) GetCachedUserByEmail(ctx context.Context, email string) (*entities.User, error) {
	if !rc.config.CacheUsers || rc.cache == nil {
		return nil, ErrCacheMiss
	}

	key := fmt.Sprintf("user:email:%s", email)
	var user entities.User
	err := rc.cache.Get(ctx, key, &user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (rc *RepositoryCache) InvalidateUser(ctx context.Context, userID uuid.UUID, email string) error {
	if !rc.config.CacheUsers || rc.cache == nil {
		return nil
	}

	// Delete both ID and email-based cache entries
	rc.cache.Delete(ctx, fmt.Sprintf("user:id:%s", userID.String()))
	if email != "" {
		rc.cache.Delete(ctx, fmt.Sprintf("user:email:%s", email))
	}

	return nil
}

// Domain caching methods

func (rc *RepositoryCache) CacheDomain(ctx context.Context, domain *entities.Domain) error {
	if !rc.config.CacheDomains || rc.cache == nil {
		return nil
	}

	// Cache by ID
	idKey := fmt.Sprintf("domain:id:%s", domain.ID.String())
	if err := rc.cache.Set(ctx, idKey, domain, rc.config.DomainTTL); err != nil {
		return err
	}

	// Cache by name
	nameKey := fmt.Sprintf("domain:name:%s", domain.Domain)
	if err := rc.cache.Set(ctx, nameKey, domain, rc.config.DomainTTL); err != nil {
		return err
	}

	// Cache by API key
	apiKeyKey := fmt.Sprintf("domain:apikey:%s", domain.APIKey)
	return rc.cache.Set(ctx, apiKeyKey, domain, rc.config.DomainTTL)
}

func (rc *RepositoryCache) GetCachedDomain(ctx context.Context, domainID uuid.UUID) (*entities.Domain, error) {
	if !rc.config.CacheDomains || rc.cache == nil {
		return nil, ErrCacheMiss
	}

	key := fmt.Sprintf("domain:id:%s", domainID.String())
	var domain entities.Domain
	err := rc.cache.Get(ctx, key, &domain)
	if err != nil {
		return nil, err
	}

	return &domain, nil
}

func (rc *RepositoryCache) GetCachedDomainByName(ctx context.Context, domainName string) (*entities.Domain, error) {
	if !rc.config.CacheDomains || rc.cache == nil {
		return nil, ErrCacheMiss
	}

	key := fmt.Sprintf("domain:name:%s", domainName)
	var domain entities.Domain
	err := rc.cache.Get(ctx, key, &domain)
	if err != nil {
		return nil, err
	}

	return &domain, nil
}

func (rc *RepositoryCache) GetCachedDomainByAPIKey(ctx context.Context, apiKey string) (*entities.Domain, error) {
	if !rc.config.CacheDomains || rc.cache == nil {
		return nil, ErrCacheMiss
	}

	key := fmt.Sprintf("domain:apikey:%s", apiKey)
	var domain entities.Domain
	err := rc.cache.Get(ctx, key, &domain)
	if err != nil {
		return nil, err
	}

	return &domain, nil
}

func (rc *RepositoryCache) CacheUserDomains(ctx context.Context, userID uuid.UUID, domains []*entities.Domain) error {
	if !rc.config.CacheDomains || rc.cache == nil {
		return nil
	}

	key := fmt.Sprintf("domains:user:%s", userID.String())
	return rc.cache.Set(ctx, key, domains, rc.config.DomainTTL)
}

func (rc *RepositoryCache) GetCachedUserDomains(ctx context.Context, userID uuid.UUID) ([]*entities.Domain, error) {
	if !rc.config.CacheDomains || rc.cache == nil {
		return nil, ErrCacheMiss
	}

	key := fmt.Sprintf("domains:user:%s", userID.String())
	var domains []*entities.Domain
	err := rc.cache.Get(ctx, key, &domains)
	if err != nil {
		return nil, err
	}

	return domains, nil
}

func (rc *RepositoryCache) InvalidateDomain(ctx context.Context, domain *entities.Domain, userID uuid.UUID) error {
	if !rc.config.CacheDomains || rc.cache == nil {
		return nil
	}

	// Delete domain cache entries
	rc.cache.Delete(ctx, fmt.Sprintf("domain:id:%s", domain.ID.String()))
	rc.cache.Delete(ctx, fmt.Sprintf("domain:name:%s", domain.Domain))
	rc.cache.Delete(ctx, fmt.Sprintf("domain:apikey:%s", domain.APIKey))

	// Invalidate user domains list
	if userID != uuid.Nil {
		rc.cache.Delete(ctx, fmt.Sprintf("domains:user:%s", userID.String()))
	}

	return nil
}

// Email Address caching methods

func (rc *RepositoryCache) CacheEmailAddress(ctx context.Context, emailAddr *entities.EmailAddress) error {
	if !rc.config.CacheEmailAddresses || rc.cache == nil {
		return nil
	}

	// Cache by ID
	idKey := fmt.Sprintf("email:id:%s", emailAddr.ID.String())
	return rc.cache.Set(ctx, idKey, emailAddr, rc.config.EmailAddressTTL)
}

func (rc *RepositoryCache) CacheEmailAddressByAddress(ctx context.Context, address string, emailAddr *entities.EmailAddress) error {
	if !rc.config.CacheEmailAddresses || rc.cache == nil {
		return nil
	}

	key := fmt.Sprintf("email:address:%s", address)
	return rc.cache.Set(ctx, key, emailAddr, rc.config.EmailAddressTTL)
}

func (rc *RepositoryCache) GetCachedEmailAddress(ctx context.Context, emailID uuid.UUID) (*entities.EmailAddress, error) {
	if !rc.config.CacheEmailAddresses || rc.cache == nil {
		return nil, ErrCacheMiss
	}

	key := fmt.Sprintf("email:id:%s", emailID.String())
	var emailAddr entities.EmailAddress
	err := rc.cache.Get(ctx, key, &emailAddr)
	if err != nil {
		return nil, err
	}

	return &emailAddr, nil
}

func (rc *RepositoryCache) GetCachedEmailAddressByAddress(ctx context.Context, address string) (*entities.EmailAddress, error) {
	if !rc.config.CacheEmailAddresses || rc.cache == nil {
		return nil, ErrCacheMiss
	}

	key := fmt.Sprintf("email:address:%s", address)
	var emailAddr entities.EmailAddress
	err := rc.cache.Get(ctx, key, &emailAddr)
	if err != nil {
		return nil, err
	}

	return &emailAddr, nil
}

func (rc *RepositoryCache) CacheDomainEmailAddresses(ctx context.Context, domainID uuid.UUID, emailAddresses []*entities.EmailAddress) error {
	if !rc.config.CacheEmailAddresses || rc.cache == nil {
		return nil
	}

	key := fmt.Sprintf("emails:domain:%s", domainID.String())
	return rc.cache.Set(ctx, key, emailAddresses, rc.config.EmailAddressTTL)
}

func (rc *RepositoryCache) GetCachedDomainEmailAddresses(ctx context.Context, domainID uuid.UUID) ([]*entities.EmailAddress, error) {
	if !rc.config.CacheEmailAddresses || rc.cache == nil {
		return nil, ErrCacheMiss
	}

	key := fmt.Sprintf("emails:domain:%s", domainID.String())
	var emailAddresses []*entities.EmailAddress
	err := rc.cache.Get(ctx, key, &emailAddresses)
	if err != nil {
		return nil, err
	}

	return emailAddresses, nil
}

func (rc *RepositoryCache) InvalidateEmailAddress(ctx context.Context, emailAddr *entities.EmailAddress, address string) error {
	if !rc.config.CacheEmailAddresses || rc.cache == nil {
		return nil
	}

	// Delete email address cache entries
	rc.cache.Delete(ctx, fmt.Sprintf("email:id:%s", emailAddr.ID.String()))
	if address != "" {
		rc.cache.Delete(ctx, fmt.Sprintf("email:address:%s", address))
	}

	// Invalidate domain email addresses list
	rc.cache.Delete(ctx, fmt.Sprintf("emails:domain:%s", emailAddr.DomainID.String()))

	return nil
}

// Received Email caching methods (optional, for recent emails)

func (rc *RepositoryCache) CacheRecentReceivedEmails(ctx context.Context, emailAddressID uuid.UUID, emails []*entities.ReceivedEmail) error {
	if !rc.config.CacheReceivedEmails || rc.cache == nil {
		return nil
	}

	key := fmt.Sprintf("received:email:%s", emailAddressID.String())
	return rc.cache.Set(ctx, key, emails, rc.config.ReceivedEmailTTL)
}

func (rc *RepositoryCache) GetCachedRecentReceivedEmails(ctx context.Context, emailAddressID uuid.UUID) ([]*entities.ReceivedEmail, error) {
	if !rc.config.CacheReceivedEmails || rc.cache == nil {
		return nil, ErrCacheMiss
	}

	key := fmt.Sprintf("received:email:%s", emailAddressID.String())
	var emails []*entities.ReceivedEmail
	err := rc.cache.Get(ctx, key, &emails)
	if err != nil {
		return nil, err
	}

	return emails, nil
}

func (rc *RepositoryCache) InvalidateReceivedEmails(ctx context.Context, emailAddressID uuid.UUID) error {
	if !rc.config.CacheReceivedEmails || rc.cache == nil {
		return nil
	}

	key := fmt.Sprintf("received:email:%s", emailAddressID.String())
	return rc.cache.Delete(ctx, key)
}

// Utility methods

func (rc *RepositoryCache) InvalidateUserData(ctx context.Context, userID uuid.UUID) error {
	if rc.cache == nil {
		return nil
	}

	// Use pattern matching to invalidate all user-related cache entries
	pattern := fmt.Sprintf("*user:%s*", userID.String())
	return rc.cache.DeletePattern(ctx, pattern)
}

func (rc *RepositoryCache) InvalidateAll(ctx context.Context) error {
	if rc.cache == nil {
		return nil
	}

	return rc.cache.FlushDB(ctx)
}

func (rc *RepositoryCache) GetCacheStats() map[string]interface{} {
	if rc.cache == nil {
		return map[string]interface{}{"enabled": false}
	}

	return rc.cache.GetStats()
}
