package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var domainCmd = &cobra.Command{
	Use:   "domain",
	Short: "Domain management",
	Long:  "Manage your MailSafe domains and their configurations.",
}

var domainListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all domains",
	Long:  "Display all domains associated with your account.",
	RunE:  runDomainList,
}

var domainCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new domain",
	Long:  "Create a new domain with encryption settings.",
	RunE:  runDomainCreate,
}

var domainDeleteCmd = &cobra.Command{
	Use:   "delete <domain-id>",
	Short: "Delete a domain",
	Long:  "Delete a domain and all its associated email addresses.",
	Args:  cobra.ExactArgs(1),
	RunE:  runDomainDelete,
}

var domainShowCmd = &cobra.Command{
	Use:   "show <domain-id>",
	Short: "Show domain details",
	Long:  "Display detailed information about a specific domain.",
	Args:  cobra.ExactArgs(1),
	RunE:  runDomainShow,
}

var (
	domainName      string
	publicKey       string
	webhookURL      string
	webhookSecret   string
	storageEnabled  bool
	force           bool
)

func init() {
	domainCmd.AddCommand(domainListCmd)
	domainCmd.AddCommand(domainCreateCmd)
	domainCmd.AddCommand(domainDeleteCmd)
	domainCmd.AddCommand(domainShowCmd)

	// Create command flags
	domainCreateCmd.Flags().StringVarP(&domainName, "domain", "d", "", "domain name (required)")
	domainCreateCmd.Flags().StringVarP(&publicKey, "public-key", "k", "", "public key for email encryption (required)")
	domainCreateCmd.Flags().StringVar(&webhookURL, "webhook-url", "", "webhook URL for email notifications")
	domainCreateCmd.Flags().StringVar(&webhookSecret, "webhook-secret", "", "webhook secret for authentication")
	domainCreateCmd.Flags().BoolVar(&storageEnabled, "storage", true, "enable email storage")
	
	domainCreateCmd.MarkFlagRequired("domain")
	domainCreateCmd.MarkFlagRequired("public-key")

	// Delete command flags
	domainDeleteCmd.Flags().BoolVarP(&force, "force", "f", false, "force deletion without confirmation")
}

func runDomainList(cmd *cobra.Command, args []string) error {
	config, err := requireAuth()
	if err != nil {
		return err
	}

	client := NewClient(config.ServerURL)
	client.SetToken(config.AccessToken)

	domains, err := client.ListDomains()
	if err != nil {
		return fmt.Errorf("failed to list domains: %w", err)
	}

	if len(domains) == 0 {
		fmt.Println("No domains found. Create one with 'mailsafe domain create'")
		return nil
	}

	// Print domains in a table format
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tDOMAIN\tVERIFIED\tSTORAGE\tCREATED")
	fmt.Fprintln(w, "--\t------\t--------\t-------\t-------")

	for _, domain := range domains {
		verified := "No"
		if domain.Verified {
			verified = "Yes"
		}
		
		storage := "No"
		if domain.StorageEnabled {
			storage = "Yes"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			domain.ID[:8]+"...", // Truncate ID for display
			domain.Domain,
			verified,
			storage,
			formatDate(domain.CreatedAt))
	}

	w.Flush()
	return nil
}

func runDomainCreate(cmd *cobra.Command, args []string) error {
	config, err := requireAuth()
	if err != nil {
		return err
	}

	client := NewClient(config.ServerURL)
	client.SetToken(config.AccessToken)

	req := CreateDomainRequest{
		Domain:         domainName,
		PublicKey:      publicKey,
		StorageEnabled: &storageEnabled,
	}

	// Add webhook configuration if provided
	if webhookURL != "" {
		req.WebhookConfig = &WebhookConfig{
			URL:     webhookURL,
			Secret:  webhookSecret,
			Enabled: true,
		}
	}

	fmt.Printf("Creating domain '%s'...\n", domainName)

	domain, err := client.CreateDomain(req)
	if err != nil {
		return fmt.Errorf("failed to create domain: %w", err)
	}

	fmt.Printf("✓ Domain created successfully!\n\n")
	fmt.Printf("Domain Details:\n")
	fmt.Printf("  ID:            %s\n", domain.ID)
	fmt.Printf("  Domain:        %s\n", domain.Domain)
	fmt.Printf("  API Key:       %s\n", domain.APIKey)
	fmt.Printf("  Verified:      %t\n", domain.Verified)
	fmt.Printf("  Storage:       %t\n", domain.StorageEnabled)
	
	if domain.WebhookConfig != nil && domain.WebhookConfig.Enabled {
		fmt.Printf("  Webhook URL:   %s\n", domain.WebhookConfig.URL)
	}

	fmt.Printf("\nNext steps:\n")
	fmt.Printf("1. Add DNS records to verify domain ownership\n")
	fmt.Printf("2. Create email addresses with 'mailsafe email create'\n")

	return nil
}

func runDomainDelete(cmd *cobra.Command, args []string) error {
	config, err := requireAuth()
	if err != nil {
		return err
	}

	domainID := args[0]

	client := NewClient(config.ServerURL)
	client.SetToken(config.AccessToken)

	// Get domain details for confirmation
	domain, err := client.GetDomain(domainID)
	if err != nil {
		return fmt.Errorf("failed to get domain details: %w", err)
	}

	// Confirmation prompt unless force flag is used
	if !force {
		fmt.Printf("Are you sure you want to delete domain '%s'? This will also delete all associated email addresses.\n", domain.Domain)
		fmt.Print("Type 'yes' to confirm: ")
		
		var confirmation string
		if _, err := fmt.Scanln(&confirmation); err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}
		
		if strings.ToLower(confirmation) != "yes" {
			fmt.Println("Domain deletion cancelled")
			return nil
		}
	}

	fmt.Printf("Deleting domain '%s'...\n", domain.Domain)

	if err := client.DeleteDomain(domainID); err != nil {
		return fmt.Errorf("failed to delete domain: %w", err)
	}

	fmt.Printf("✓ Domain '%s' deleted successfully\n", domain.Domain)
	return nil
}

func runDomainShow(cmd *cobra.Command, args []string) error {
	config, err := requireAuth()
	if err != nil {
		return err
	}

	domainID := args[0]

	client := NewClient(config.ServerURL)
	client.SetToken(config.AccessToken)

	domain, err := client.GetDomain(domainID)
	if err != nil {
		return fmt.Errorf("failed to get domain details: %w", err)
	}

	fmt.Printf("Domain Details:\n")
	fmt.Printf("  ID:            %s\n", domain.ID)
	fmt.Printf("  Domain:        %s\n", domain.Domain)
	fmt.Printf("  API Key:       %s\n", domain.APIKey)
	fmt.Printf("  Verified:      %t\n", domain.Verified)
	fmt.Printf("  Storage:       %t\n", domain.StorageEnabled)
	fmt.Printf("  Created:       %s\n", formatDate(domain.CreatedAt))
	fmt.Printf("  Updated:       %s\n", formatDate(domain.UpdatedAt))
	
	if domain.WebhookConfig != nil {
		fmt.Printf("\nWebhook Configuration:\n")
		fmt.Printf("  Enabled:       %t\n", domain.WebhookConfig.Enabled)
		if domain.WebhookConfig.Enabled {
			fmt.Printf("  URL:           %s\n", domain.WebhookConfig.URL)
			if len(domain.WebhookConfig.Headers) > 0 {
				fmt.Printf("  Headers:       %d custom headers configured\n", len(domain.WebhookConfig.Headers))
			}
		}
	}

	fmt.Printf("\nPublic Key (truncated):\n")
	if len(domain.PublicKey) > 100 {
		fmt.Printf("  %s...\n", domain.PublicKey[:100])
	} else {
		fmt.Printf("  %s\n", domain.PublicKey)
	}

	return nil
}

// formatDate formats a date string for display
func formatDate(dateStr string) string {
	// Simple date formatting - could be enhanced with proper parsing
	if len(dateStr) > 10 {
		return dateStr[:10] // Return just the date part
	}
	return dateStr
}