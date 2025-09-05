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
	default:
		return nil, fmt.Errorf("unsupported auth provider: %s (supported: supabase)", config.Provider)
	}
}
