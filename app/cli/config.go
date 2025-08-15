package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	ServerURL   string `json:"server_url"`
	AccessToken string `json:"access_token,omitempty"`
	UserEmail   string `json:"user_email,omitempty"`
	UserID      string `json:"user_id,omitempty"`
}

func getConfigPath() string {
	return filepath.Join(configDir, "config.json")
}

func loadConfig() (*Config, error) {
	configPath := getConfigPath()

	// If config file doesn't exist, return default config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &Config{
			ServerURL: serverURL,
		}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Override with command line flags if provided
	if serverURL != "http://localhost:8080" {
		config.ServerURL = serverURL
	}

	return &config, nil
}

func saveConfig(config *Config) error {
	configPath := getConfigPath()

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func requireAuth() (*Config, error) {
	config, err := loadConfig()
	if err != nil {
		return nil, err
	}

	if config.AccessToken == "" {
		return nil, fmt.Errorf("not logged in. Please run 'mailvault login' first")
	}

	return config, nil
}
