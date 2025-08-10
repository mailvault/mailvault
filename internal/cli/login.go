package cli

import (
	"fmt"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to MailSafe",
	Long:  "Login to your MailSafe account using email and password.",
	RunE:  runLogin,
}

var registerFlag bool

func init() {
	loginCmd.Flags().BoolVarP(&registerFlag, "register", "r", false, "register a new account instead of login")
}

func runLogin(cmd *cobra.Command, args []string) error {
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if registerFlag {
		return runRegister(config)
	}

	// Check if already logged in
	if config.AccessToken != "" {
		fmt.Printf("Already logged in as %s\n", config.UserEmail)
		fmt.Println("Use 'mailsafe user info' to view your account details")
		return nil
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
	fmt.Println("You can now use other MailSafe commands")

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
	fmt.Println("You can now use other MailSafe commands")

	return nil
}