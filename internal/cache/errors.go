package cache

import "errors"

var (
	// ErrCacheMiss indicates that a requested key was not found in the cache
	ErrCacheMiss = errors.New("cache miss")

	// ErrCacheUnavailable indicates that the cache service is unavailable
	ErrCacheUnavailable = errors.New("cache unavailable")

	// ErrInvalidKey indicates that the provided cache key is invalid
	ErrInvalidKey = errors.New("invalid cache key")

	// ErrSerializationFailed indicates that serialization/deserialization failed
	ErrSerializationFailed = errors.New("serialization failed")
)