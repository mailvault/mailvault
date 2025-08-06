package auth

import "fmt"

type Config struct {
	Provider        string `yaml:"provider"`
	SupabaseURL     string `yaml:"supabase_url"`
	SupabaseAPIKey  string `yaml:"supabase_api_key"`
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
	default:
		return nil, fmt.Errorf("unsupported auth provider: %s", config.Provider)
	}
}