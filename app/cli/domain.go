package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var domainCmd = &cobra.Command{
	Use:   "domain",
	Short: "Domain management",
	Long:  "Manage your MailVault domains and their configurations.",
}

var domainListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all domains",
	Long:  "Display all domains associated with your account.",
	RunE:  runDomainList,
}

var domainCreateCmd = &cobra.Command{
	Use:   "create [domain]",
	Short: "Create a new domain",
	Long: `Create a new domain with encryption settings.

If you have locally generated keys for this domain, they will be used automatically.
Otherwise, you must provide a public key with --public-key.

Examples:
  mailvault domain create example.com           # Use local keys if available
  mailvault domain create example.com --public-key x25519:...  # Use specific key`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDomainCreate,
}

var domainDeleteCmd = &cobra.Command{
	Use:   "delete <domain>",
	Short: "Delete a domain",
	Long:  "Delete a domain and all its associated email addresses.",
	Args:  cobra.ExactArgs(1),
	RunE:  runDomainDelete,
}

var domainShowCmd = &cobra.Command{
	Use:   "show <domain>",
	Short: "Show domain details",
	Long:  "Display detailed information about a specific domain.",
	Args:  cobra.ExactArgs(1),
	RunE:  runDomainShow,
}

var (
	domainName        string
	publicKey         string
	webhookURL        string
	webhookSecret     string
	storageEnabled    bool
	autoCreateAddress bool
	force             bool
	// new common flags
	outputFormat string
	domainLimit  int
	domainOffset int
)

func init() {
	domainCmd.AddCommand(domainListCmd)
	domainCmd.AddCommand(domainCreateCmd)
	domainCmd.AddCommand(domainDeleteCmd)
	domainCmd.AddCommand(domainShowCmd)

	// Create command flags
	domainCreateCmd.Flags().StringVarP(&domainName, "domain", "d", "", "domain name (overrides positional argument)")
	domainCreateCmd.Flags().StringVarP(&publicKey, "public-key", "k", "", "public key for email encryption (auto-detected from local keys if available)")
	domainCreateCmd.Flags().StringVar(&webhookURL, "webhook-url", "", "webhook URL for email notifications")
	domainCreateCmd.Flags().StringVar(&webhookSecret, "webhook-secret", "", "webhook secret for authentication")
	domainCreateCmd.Flags().BoolVar(&storageEnabled, "storage", true, "enable email storage")
	domainCreateCmd.Flags().BoolVar(&autoCreateAddress, "auto-create", false, "automatically create email addresses when receiving emails to non-existent addresses")

	// Delete command flags
	domainDeleteCmd.Flags().BoolVarP(&force, "force", "f", false, "force deletion without confirmation")

	// List/Show output and pagination flags
	domainListCmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "output format: table|json|yaml")
	domainListCmd.Flags().IntVarP(&domainLimit, "limit", "l", 0, "maximum number of domains to show (0 = all)")
	domainListCmd.Flags().IntVarP(&domainOffset, "offset", "O", 0, "number of domains to skip")
	domainShowCmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "output format: table|json|yaml")
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

	// Apply offset/limit client-side
	start := domainOffset
	if start < 0 {
		start = 0
	}
	if start > len(domains) {
		start = len(domains)
	}
	end := len(domains)
	if domainLimit > 0 && start+domainLimit < end {
		end = start + domainLimit
	}
	domains = domains[start:end]

	if len(domains) == 0 {
		if outputFormat == "json" {
			fmt.Println("[]")
			return nil
		}
		if outputFormat == "yaml" {
			fmt.Println("[]")
			return nil
		}
		fmt.Println("No domains found. Create one with 'mailvault domain create'")
		return nil
	}

	switch outputFormat {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(domains)
	case "yaml":
		data, err := yaml.Marshal(domains)
		if err != nil {
			return fmt.Errorf("failed to marshal yaml: %w", err)
		}
		fmt.Print(string(data))
		return nil
	case "table":
		// Print in a table format
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tDOMAIN\tVERIFIED\tSTORAGE\tAUTO-CREATE\tCREATED")
		fmt.Fprintln(w, "--\t------\t--------\t-------\t-----------\t-------")
		for _, domain := range domains {
			verified := "No"
			if domain.Verified {
				verified = "Yes"
			}
			storage := "No"
			if domain.StorageEnabled {
				storage = "Yes"
			}
			autoCreate := "No" // TODO: enable when SDK exposes
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				domain.ID[:8]+"...",
				domain.Domain,
				verified,
				storage,
				autoCreate,
				formatDate(domain.CreatedAt))
		}
		w.Flush()
		return nil
	default:
		return fmt.Errorf("unknown output format: %s (use table|json|yaml)", outputFormat)
	}
}

func runDomainCreate(cmd *cobra.Command, args []string) error {
	config, err := requireAuth()
	if err != nil {
		return err
	}

	// Determine domain name from args or flag
	var targetDomain string
	if len(args) > 0 {
		targetDomain = strings.ToLower(args[0])
	} else if domainName != "" {
		targetDomain = strings.ToLower(domainName)
	} else {
		return fmt.Errorf("domain name is required (provide as argument or --domain flag)")
	}

	// Determine public key - check local keys first, then flag
	var targetPublicKey string
	if publicKey != "" {
		// Use provided public key
		targetPublicKey = publicKey
	} else {
		// Try to load from local keys
		keyFile, err := loadKeyFile(targetDomain)
		if err != nil {
			return fmt.Errorf("no public key provided and no local keys found for domain %s. Generate keys first with 'mailvault keys generate %s' or provide --public-key", targetDomain, targetDomain)
		}
		targetPublicKey = keyFile.PublicKey
		fmt.Printf("Using local public key for domain: %s\n", targetDomain)
	}

	client := NewClient(config.ServerURL)
	client.SetToken(config.AccessToken)

	req := CreateDomainRequest{
		Domain:         targetDomain,
		PublicKey:      targetPublicKey,
		StorageEnabled: &storageEnabled,
		// AutoCreateAddress: &autoCreateAddress, // TODO: Add when SDK is updated
	}

	// Add webhook configuration if provided
	if webhookURL != "" {
		req.WebhookConfig = &WebhookConfig{
			URL:     webhookURL,
			Secret:  webhookSecret,
			Enabled: true,
		}
	}

	fmt.Printf("Creating domain '%s'...\n", targetDomain)

	domain, err := client.CreateDomain(req)
	if err != nil {
		return fmt.Errorf("failed to create domain: %w", err)
	}

	fmt.Printf("✓ Domain created successfully!\n\n")
	fmt.Printf("Domain Details:\n")
	fmt.Printf("  ID:              %s\n", domain.ID)
	fmt.Printf("  Domain:          %s\n", domain.Domain)
	fmt.Printf("  API Key:         %s\n", domain.APIKey)
	fmt.Printf("  Verified:        %t\n", domain.Verified)
	fmt.Printf("  Storage:         %t\n", domain.StorageEnabled)
	// TODO: Add when SDK is updated
	// fmt.Printf("  Auto-create:     %t\n", domain.AutoCreateAddress)

	if domain.WebhookConfig != nil && domain.WebhookConfig.Enabled {
		fmt.Printf("  Webhook URL:   %s\n", domain.WebhookConfig.URL)
	}

	fmt.Printf("\nNext steps:\n")
	fmt.Printf("1. Add DNS records to verify domain ownership\n")
	fmt.Printf("2. Create email addresses with 'mailvault email create'\n")

	return nil
}

func runDomainDelete(cmd *cobra.Command, args []string) error {
	config, err := requireAuth()
	if err != nil {
		return err
	}

	domainRef := args[0]

	client := NewClient(config.ServerURL)
	client.SetToken(config.AccessToken)

	// Resolve domain by name or id for confirmation and deletion
	d, err := client.ResolveDomainReference(domainRef)
	if err != nil {
		return fmt.Errorf("failed to resolve domain: %w", err)
	}

	// Confirmation prompt unless force flag is used
	if !force {
		fmt.Printf("Are you sure you want to delete domain '%s'? This will also delete all associated email addresses.\n", d.Domain)
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

	fmt.Printf("Deleting domain '%s'...\n", d.Domain)

	if err := client.DeleteDomain(d.ID); err != nil {
		return fmt.Errorf("failed to delete domain: %w", err)
	}

	fmt.Printf("✓ Domain '%s' deleted successfully\n", d.Domain)
	return nil
}

func runDomainShow(cmd *cobra.Command, args []string) error {
	config, err := requireAuth()
	if err != nil {
		return err
	}

	domainRef := args[0]

	client := NewClient(config.ServerURL)
	client.SetToken(config.AccessToken)

	d, err := client.ResolveDomainReference(domainRef)
	if err != nil {
		return fmt.Errorf("failed to resolve domain: %w", err)
	}

	switch outputFormat {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	case "yaml":
		data, err := yaml.Marshal(d)
		if err != nil {
			return fmt.Errorf("failed to marshal yaml: %w", err)
		}
		fmt.Print(string(data))
		return nil
	case "table":
		fmt.Printf("Domain Details:\n")
		fmt.Printf("  ID:              %s\n", d.ID)
		fmt.Printf("  Domain:          %s\n", d.Domain)
		fmt.Printf("  API Key:         %s\n", d.APIKey)
		fmt.Printf("  Verified:        %t\n", d.Verified)
		fmt.Printf("  Storage:         %t\n", d.StorageEnabled)
		// TODO: Add when SDK is updated
		// fmt.Printf("  Auto-create:     %t\n", d.AutoCreateAddress)
		fmt.Printf("  Created:         %s\n", formatDate(d.CreatedAt))
		fmt.Printf("  Updated:         %s\n", formatDate(d.UpdatedAt))

		if d.WebhookConfig != nil {
			fmt.Printf("\nWebhook Configuration:\n")
			fmt.Printf("  Enabled:       %t\n", d.WebhookConfig.Enabled)
			if d.WebhookConfig.Enabled {
				fmt.Printf("  URL:           %s\n", d.WebhookConfig.URL)
				if len(d.WebhookConfig.Headers) > 0 {
					fmt.Printf("  Headers:       %d custom headers configured\n", len(d.WebhookConfig.Headers))
				}
			}
		}

		fmt.Printf("\nPublic Key (truncated):\n")
		if len(d.PublicKey) > 100 {
			fmt.Printf("  %s...\n", d.PublicKey[:100])
		} else {
			fmt.Printf("  %s\n", d.PublicKey)
		}
		return nil
	default:
		return fmt.Errorf("unknown output format: %s (use table|json|yaml)", outputFormat)
	}
}

// formatDate formats a date string for display
func formatDate(dateStr string) string {
	// Simple date formatting - could be enhanced with proper parsing
	if len(dateStr) > 10 {
		return dateStr[:10] // Return just the date part
	}
	return dateStr
}
