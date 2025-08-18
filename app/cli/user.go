package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "User account management",
	Long:  "Manage your MailVault user account and view account information.",
}

var userInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show user information",
	Long:  "Display information about your current user account.",
	RunE:  runUserInfo,
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout from MailVault",
	Long:  "Logout from your current MailVault session.",
	RunE:  runLogout,
}

func init() {
	userCmd.AddCommand(userInfoCmd)
	userCmd.AddCommand(logoutCmd)
}

func runUserInfo(cmd *cobra.Command, args []string) error {
	config, err := requireAuth()
	if err != nil {
		return err
	}

	client := NewClient(config.ServerURL)
	client.SetToken(config.AccessToken)

	user, err := client.GetMe()
	if err != nil {
		return fmt.Errorf("failed to get user info: %w", err)
	}

	fmt.Printf("User Information:\n")
	fmt.Printf("  ID:       %s\n", user.ID)
	fmt.Printf("  Email:    %s\n", user.Email)
	fmt.Printf("  Provider: %s\n", user.AuthProvider)
	fmt.Printf("  Server:   %s\n", config.ServerURL)

	return nil
}

func runLogout(cmd *cobra.Command, args []string) error {
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if config.AccessToken == "" {
		fmt.Println("Not currently logged in")
		return nil
	}

	userEmail := config.UserEmail
	
	// Clear authentication data
	config.AccessToken = ""
	config.UserEmail = ""
	config.UserID = ""

	if err := saveConfig(config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if userEmail != "" {
		fmt.Printf("✓ Successfully logged out from %s\n", userEmail)
	} else {
		fmt.Println("✓ Successfully logged out")
	}
	fmt.Println("Use 'mailvault login' to sign in again")

	return nil
}
