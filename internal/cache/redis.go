package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	// Redis connection settings
	Addr     string // Redis server address (default: localhost:6379)
	Password string // Redis password (if required)
	DB       int    // Redis database number (default: 0)

	// Connection pool settings
	PoolSize     int           // Maximum number of connections (default: 10)
	MinIdleConns int           // Minimum idle connections (default: 2)
	MaxRetries   int           // Maximum retry attempts (default: 3)
	DialTimeout  time.Duration // Connection timeout (default: 5s)
	ReadTimeout  time.Duration // Read timeout (default: 3s)
	WriteTimeout time.Duration // Write timeout (default: 3s)
	IdleTimeout  time.Duration // Idle connection timeout (default: 300s)

	// Cache settings
	DefaultTTL time.Duration // Default expiration time (default: 5 minutes)
	KeyPrefix  string        // Key prefix for all cache entries (default: "mailvault:")

	// Enable/disable caching
	Enabled bool

	// Logger
	Logger *slog.Logger
}

// DefaultRedisConfig returns production-ready Redis configuration
func DefaultRedisConfig() RedisConfig {
	return RedisConfig{
		Addr:         "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 2,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		IdleTimeout:  300 * time.Second,
		DefaultTTL:   5 * time.Minute,
		KeyPrefix:    "mailvault:",
		Enabled:      true,
	}
}

// RedisCache provides Redis-based caching functionality
type RedisCache struct {
	client *redis.Client
	config RedisConfig
	logger *slog.Logger
}

// NewRedisCache creates a new Redis cache instance
func NewRedisCache(config RedisConfig) (*RedisCache, error) {
	if !config.Enabled {
		return &RedisCache{
			config: config,
			logger: config.Logger,
		}, nil
	}

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:            config.Addr,
		Password:        config.Password,
		DB:              config.DB,
		PoolSize:        config.PoolSize,
		MinIdleConns:    config.MinIdleConns,
		MaxRetries:      config.MaxRetries,
		DialTimeout:     config.DialTimeout,
		ReadTimeout:     config.ReadTimeout,
		WriteTimeout:    config.WriteTimeout,
		ConnMaxIdleTime: config.IdleTimeout,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	cache := &RedisCache{
		client: client,
		config: config,
		logger: config.Logger,
	}

	if cache.logger != nil {
		cache.logger.Info("Redis cache initialized",
			"addr", config.Addr,
			"db", config.DB,
			"pool_size", config.PoolSize,
		)
	}

	return cache, nil
}

// buildKey creates a cache key with the configured prefix
func (c *RedisCache) buildKey(key string) string {
	return c.config.KeyPrefix + key
}

// Set stores a value in the cache with the specified TTL
func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if !c.config.Enabled || c.client == nil {
		return nil // Silently skip if caching is disabled
	}

	// Use default TTL if not specified
	if ttl == 0 {
		ttl = c.config.DefaultTTL
	}

	// Serialize value to JSON
	data, err := json.Marshal(value)
	if err != nil {
		if c.logger != nil {
			c.logger.Error("Failed to marshal cache value",
				"key", key,
				"error", err,
			)
		}
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	// Store in Redis
	err = c.client.Set(ctx, c.buildKey(key), data, ttl).Err()
	if err != nil {
		if c.logger != nil {
			c.logger.Error("Failed to set cache value",
				"key", key,
				"error", err,
			)
		}
		return fmt.Errorf("failed to set cache value: %w", err)
	}

	return nil
}

// Get retrieves a value from the cache
func (c *RedisCache) Get(ctx context.Context, key string, dest interface{}) error {
	if !c.config.Enabled || c.client == nil {
		return ErrCacheMiss
	}

	// Get from Redis
	data, err := c.client.Get(ctx, c.buildKey(key)).Result()
	if err != nil {
		if err == redis.Nil {
			return ErrCacheMiss
		}
		if c.logger != nil {
			c.logger.Error("Failed to get cache value",
				"key", key,
				"error", err,
			)
		}
		return fmt.Errorf("failed to get cache value: %w", err)
	}

	// Deserialize from JSON
	err = json.Unmarshal([]byte(data), dest)
	if err != nil {
		if c.logger != nil {
			c.logger.Error("Failed to unmarshal cache value",
				"key", key,
				"error", err,
			)
		}
		return fmt.Errorf("failed to unmarshal value: %w", err)
	}

	return nil
}

// Delete removes a value from the cache
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	if !c.config.Enabled || c.client == nil {
		return nil // Silently skip if caching is disabled
	}

	err := c.client.Del(ctx, c.buildKey(key)).Err()
	if err != nil {
		if c.logger != nil {
			c.logger.Error("Failed to delete cache value",
				"key", key,
				"error", err,
			)
		}
		return fmt.Errorf("failed to delete cache value: %w", err)
	}

	return nil
}

// DeletePattern removes all keys matching a pattern
func (c *RedisCache) DeletePattern(ctx context.Context, pattern string) error {
	if !c.config.Enabled || c.client == nil {
		return nil // Silently skip if caching is disabled
	}

	// Get all keys matching the pattern
	keys, err := c.client.Keys(ctx, c.buildKey(pattern)).Result()
	if err != nil {
		return fmt.Errorf("failed to get keys: %w", err)
	}

	if len(keys) == 0 {
		return nil // No keys to delete
	}

	// Delete all matching keys
	err = c.client.Del(ctx, keys...).Err()
	if err != nil {
		if c.logger != nil {
			c.logger.Error("Failed to delete cache pattern",
				"pattern", pattern,
				"keys_count", len(keys),
				"error", err,
			)
		}
		return fmt.Errorf("failed to delete keys: %w", err)
	}

	if c.logger != nil {
		c.logger.Debug("Deleted cache pattern",
			"pattern", pattern,
			"keys_count", len(keys),
		)
	}

	return nil
}

// Exists checks if a key exists in the cache
func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	if !c.config.Enabled || c.client == nil {
		return false, nil
	}

	count, err := c.client.Exists(ctx, c.buildKey(key)).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check key existence: %w", err)
	}

	return count > 0, nil
}

// SetExpire updates the TTL of an existing key
func (c *RedisCache) SetExpire(ctx context.Context, key string, ttl time.Duration) error {
	if !c.config.Enabled || c.client == nil {
		return nil
	}

	err := c.client.Expire(ctx, c.buildKey(key), ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set expiration: %w", err)
	}

	return nil
}

// GetTTL returns the remaining TTL of a key
func (c *RedisCache) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	if !c.config.Enabled || c.client == nil {
		return 0, nil
	}

	ttl, err := c.client.TTL(ctx, c.buildKey(key)).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get TTL: %w", err)
	}

	return ttl, nil
}

// Increment atomically increments a counter
func (c *RedisCache) Increment(ctx context.Context, key string) (int64, error) {
	if !c.config.Enabled || c.client == nil {
		return 0, nil
	}

	result, err := c.client.Incr(ctx, c.buildKey(key)).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment: %w", err)
	}

	return result, nil
}

// IncrementBy atomically increments a counter by a specific value
func (c *RedisCache) IncrementBy(ctx context.Context, key string, value int64) (int64, error) {
	if !c.config.Enabled || c.client == nil {
		return 0, nil
	}

	result, err := c.client.IncrBy(ctx, c.buildKey(key), value).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment by: %w", err)
	}

	return result, nil
}

// SetNX sets a key only if it doesn't exist (useful for locks)
func (c *RedisCache) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	if !c.config.Enabled || c.client == nil {
		return false, nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return false, fmt.Errorf("failed to marshal value: %w", err)
	}

	result, err := c.client.SetNX(ctx, c.buildKey(key), data, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to set NX: %w", err)
	}

	return result, nil
}

// FlushDB clears all keys in the current database (use with caution)
func (c *RedisCache) FlushDB(ctx context.Context) error {
	if !c.config.Enabled || c.client == nil {
		return nil
	}

	err := c.client.FlushDB(ctx).Err()
	if err != nil {
		return fmt.Errorf("failed to flush database: %w", err)
	}

	if c.logger != nil {
		c.logger.Warn("Flushed Redis database", "db", c.config.DB)
	}

	return nil
}

// GetStats returns Redis connection statistics
func (c *RedisCache) GetStats() map[string]interface{} {
	if !c.config.Enabled || c.client == nil {
		return map[string]interface{}{
			"enabled": false,
		}
	}

	stats := c.client.PoolStats()
	return map[string]interface{}{
		"enabled":     true,
		"addr":        c.config.Addr,
		"db":          c.config.DB,
		"hits":        stats.Hits,
		"misses":      stats.Misses,
		"timeouts":    stats.Timeouts,
		"total_conns": stats.TotalConns,
		"idle_conns":  stats.IdleConns,
		"stale_conns": stats.StaleConns,
	}
}

// Close closes the Redis connection
func (c *RedisCache) Close() error {
	if !c.config.Enabled || c.client == nil {
		return nil
	}

	err := c.client.Close()
	if err != nil {
		return fmt.Errorf("failed to close Redis client: %w", err)
	}

	if c.logger != nil {
		c.logger.Info("Redis cache connection closed")
	}

	return nil
}

// Ping tests the Redis connection
func (c *RedisCache) Ping(ctx context.Context) error {
	if !c.config.Enabled || c.client == nil {
		return nil
	}

	return c.client.Ping(ctx).Err()
}

// Cache interface for dependency injection
type Cache interface {
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Get(ctx context.Context, key string, dest interface{}) error
	Delete(ctx context.Context, key string) error
	DeletePattern(ctx context.Context, pattern string) error
	Exists(ctx context.Context, key string) (bool, error)
	SetExpire(ctx context.Context, key string, ttl time.Duration) error
	GetTTL(ctx context.Context, key string) (time.Duration, error)
	Increment(ctx context.Context, key string) (int64, error)
	IncrementBy(ctx context.Context, key string, value int64) (int64, error)
	SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)
	FlushDB(ctx context.Context) error
	GetStats() map[string]interface{}
	Close() error
	Ping(ctx context.Context) error
}

// Ensure RedisCache implements Cache interface
var _ Cache = (*RedisCache)(nil)
