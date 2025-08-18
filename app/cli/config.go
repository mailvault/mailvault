package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/term"
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

	// Validate token by testing it with the API
	client := NewClient(config.ServerURL)
	client.SetToken(config.AccessToken)
	
	_, err = client.GetMe()
	if err != nil {
		// Token is invalid/expired, attempt re-authentication
		if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "unauthorized") || strings.Contains(err.Error(), "token") {
			fmt.Printf("⚠️  Your authentication token has expired.\n")
			
			// Attempt automatic re-authentication
			newConfig, reAuthErr := attemptReAuth(config)
			if reAuthErr != nil {
				// Clear invalid token and ask user to login manually
				config.AccessToken = ""
				config.UserEmail = ""
				config.UserID = ""
				saveConfig(config)
				return nil, fmt.Errorf("authentication expired. Please run 'mailvault login' to sign in again")
			}
			
			return newConfig, nil
		}
		
		// If it's not an auth error, return the original error
		return nil, fmt.Errorf("failed to validate authentication: %w", err)
	}

	return config, nil
}

// attemptReAuth attempts to re-authenticate the user automatically
func attemptReAuth(config *Config) (*Config, error) {
	if config.UserEmail == "" {
		return nil, fmt.Errorf("no stored email for re-authentication")
	}

	fmt.Printf("Attempting to re-authenticate as %s...\n", config.UserEmail)
	fmt.Print("Password: ")
	
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return nil, fmt.Errorf("failed to read password: %w", err)
	}
	password := string(passwordBytes)
	fmt.Println() // New line after password input

	client := NewClient(config.ServerURL)
	resp, err := client.Login(config.UserEmail, password)
	if err != nil {
		return nil, fmt.Errorf("re-authentication failed: %w", err)
	}

	// Update config with new token
	config.AccessToken = resp.Token
	config.UserEmail = resp.User.Email
	config.UserID = resp.User.ID

	if err := saveConfig(config); err != nil {
		return nil, fmt.Errorf("failed to save updated config: %w", err)
	}

	fmt.Printf("✓ Successfully re-authenticated as %s\n", resp.User.Email)
	return config, nil
}

// validateAndRefreshAuth validates the current token and refreshes if needed
func validateAndRefreshAuth(config *Config) (*Config, error) {
	if config.AccessToken == "" {
		return nil, fmt.Errorf("not logged in")
	}

	// Test current token
	client := NewClient(config.ServerURL)
	client.SetToken(config.AccessToken)
	
	_, err := client.GetMe()
	if err == nil {
		// Token is still valid
		return config, nil
	}

	// Token is invalid, try to refresh
	if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "unauthorized") {
		return attemptReAuth(config)
	}

	return nil, err
}
