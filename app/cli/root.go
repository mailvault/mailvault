package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	configDir  string
	serverURL  string
	verbose    bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mailvault",
	Short: "MailVault CLI - Manage your private email service",
	Long: `MailVault CLI allows you to interact with the MailVault email service.

You can manage users, domains, email addresses, and view received emails
through this command-line interface.

Examples:
  mailvault login                    # Login to your account
  mailvault keys generate example.com # Generate encryption keys
  mailvault domain create           # Create a new domain
  mailvault email list              # List your email addresses
  mailvault inbox                   # View received emails`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	homeDir, _ := os.UserHomeDir()
	defaultConfigDir := filepath.Join(homeDir, ".mailvault")
	
	rootCmd.PersistentFlags().StringVar(&configDir, "config", defaultConfigDir, "config directory")
	rootCmd.PersistentFlags().StringVar(&serverURL, "server", "http://localhost:8080", "MailVault server URL")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Add subcommands
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(userCmd)
	rootCmd.AddCommand(domainCmd)
	rootCmd.AddCommand(emailCmd)
	rootCmd.AddCommand(inboxCmd)
	rootCmd.AddCommand(keysCmd)
}

func initConfig() {
	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create config directory: %v\n", err)
	}
}