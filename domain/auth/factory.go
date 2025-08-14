package auth

import "fmt"

type Config struct {
	Provider       string
	SupabaseURL    string
	SupabaseAPIKey string
}

func NewAuthProvider(config Config) (Provider, error) {
	switch config.Provider {
	case "supabase":
		if config.SupabaseURL == "" || config.SupabaseAPIKey == "" {
			return nil, fmt.Errorf("supabase configuration missing: url and api_key required")
		}
		return NewSupabaseProvider(config.SupabaseURL, config.SupabaseAPIKey), nil
	case "basic":
		return NewBasicProvider(), nil
	case "mock":
		return NewMockProvider(), nil
	default:
		return nil, fmt.Errorf("unsupported auth provider: %s", config.Provider)
	}
}
