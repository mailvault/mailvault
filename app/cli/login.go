package cli

import (
	"fmt"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to MailVault",
	Long:  "Login to your MailVault account using email and password.",
	RunE:  runLogin,
}


var (
	registerFlag bool
	forceFlag    bool
)

func init() {
	loginCmd.Flags().BoolVarP(&registerFlag, "register", "r", false, "register a new account instead of login")
	loginCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "force login even if already authenticated")
}

func runLogin(cmd *cobra.Command, args []string) error {
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if registerFlag {
		return runRegister(config)
	}

	// Check if already logged in (unless force flag is used)
	if config.AccessToken != "" && !forceFlag {
		// Validate current token
		validConfig, err := validateAndRefreshAuth(config)
		if err == nil {
			fmt.Printf("Already logged in as %s\n", validConfig.UserEmail)
			fmt.Println("Use 'mailvault user info' to view your account details")
			fmt.Println("Use 'mailvault login --force' to login with a different account")
			return nil
		}
		
		// Token is invalid, clear it and continue with login
		fmt.Println("⚠️  Current authentication is invalid, proceeding with login...")
		config.AccessToken = ""
		config.UserEmail = ""
		config.UserID = ""
	}

	client := NewClient(config.ServerURL)

	fmt.Print("Email: ")
	var email string
	if _, err := fmt.Scanln(&email); err != nil {
		return fmt.Errorf("failed to read email: %w", err)
	}

	fmt.Print("Password: ")
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}
	password := string(passwordBytes)
	fmt.Println() // New line after password input

	fmt.Println("Logging in...")

	resp, err := client.Login(email, password)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	// Save authentication info
	config.AccessToken = resp.Token
	config.UserEmail = resp.User.Email
	config.UserID = resp.User.ID

	if err := saveConfig(config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✓ Successfully logged in as %s\n", resp.User.Email)
	fmt.Println("You can now use other MailVault commands")

	return nil
}

func runRegister(config *Config) error {
	client := NewClient(config.ServerURL)

	fmt.Print("Email: ")
	var email string
	if _, err := fmt.Scanln(&email); err != nil {
		return fmt.Errorf("failed to read email: %w", err)
	}

	fmt.Print("Password: ")
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}
	password := string(passwordBytes)
	fmt.Println() // New line after password input

	fmt.Print("Confirm Password: ")
	confirmPasswordBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return fmt.Errorf("failed to read password confirmation: %w", err)
	}
	confirmPassword := string(confirmPasswordBytes)
	fmt.Println() // New line after password input

	if password != confirmPassword {
		return fmt.Errorf("passwords do not match")
	}

	fmt.Println("Creating account...")

	resp, err := client.Register(email, password)
	if err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	// Save authentication info
	config.AccessToken = resp.Token
	config.UserEmail = resp.User.Email
	config.UserID = resp.User.ID

	if err := saveConfig(config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✓ Successfully created account and logged in as %s\n", resp.User.Email)
	fmt.Println("You can now use other MailVault commands")

	return nil
}