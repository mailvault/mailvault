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

var emailCmd = &cobra.Command{
	Use:   "email",
	Short: "Email address management",
	Long:  "Manage email addresses for your domains.",
}

var emailListCmd = &cobra.Command{
	Use:   "list [domain]",
	Short: "List email addresses",
	Long:  "List email addresses for a specific domain or all domains.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runEmailList,
}

var emailCreateCmd = &cobra.Command{
	Use:   "create <domain> <local-part>",
	Short: "Create a new email address",
	Long:  "Create a new email address for a domain.",
	Args:  cobra.ExactArgs(2),
	RunE:  runEmailCreate,
}

var emailDeleteCmd = &cobra.Command{
	Use:   "delete <domain> <email-id>",
	Short: "Delete an email address",
	Long:  "Delete an email address from a domain.",
	Args:  cobra.ExactArgs(2),
	RunE:  runEmailDelete,
}

var (
	emailLocalPart   string
	emailForwardTo   []string
	emailForceDelete bool
	emailOutput      string
)

func init() {
	emailCmd.AddCommand(emailListCmd)
	emailCmd.AddCommand(emailCreateCmd)
	emailCmd.AddCommand(emailDeleteCmd)

	// List flags
	emailListCmd.Flags().StringVarP(&emailOutput, "output", "o", "table", "output format: table|json|yaml")

	// Create flags (positional args preferred now)
	emailCreateCmd.Flags().StringSliceVarP(&emailForwardTo, "forward", "f", []string{}, "forward emails to these addresses (comma-separated)")

	// Delete flags
	emailDeleteCmd.Flags().BoolVarP(&emailForceDelete, "force", "f", false, "force deletion without confirmation")
}

func runEmailList(cmd *cobra.Command, args []string) error {
	config, err := requireAuth()
	if err != nil {
		return err
	}

	client := NewClient(config.ServerURL)
	client.SetToken(config.AccessToken)

	// If domain ref is provided, list emails for that domain only
	if len(args) > 0 {
		d, err := client.ResolveDomainReference(args[0])
		if err != nil {
			return fmt.Errorf("failed to resolve domain: %w", err)
		}
		emails, err := client.ListEmailAddresses(d.ID)
		if err != nil {
			return fmt.Errorf("failed to list email addresses: %w", err)
		}
		return outputEmails(emails, d.Domain, emailOutput)
	}

	// Otherwise, list emails for all domains
	domains, err := client.ListDomains()
	if err != nil {
		return fmt.Errorf("failed to list domains: %w", err)
	}

	if len(domains) == 0 {
		if emailOutput == "json" || emailOutput == "yaml" {
			fmt.Println("[]")
			return nil
		}
		fmt.Println("No domains found. Create one with 'mailvault domain create'")
		return nil
	}

	totalEmails := 0
	for _, d := range domains {
		emails, err := client.ListEmailAddresses(d.ID)
		if err != nil {
			fmt.Printf("Warning: failed to list emails for domain %s: %v\n", d.Domain, err)
			continue
		}
		if len(emails) == 0 {
			continue
		}
		if emailOutput == "table" {
			fmt.Printf("\n--- %s ---\n", d.Domain)
		}
		if err := outputEmails(emails, d.Domain, emailOutput); err != nil {
			return err
		}
		totalEmails += len(emails)
	}

	if totalEmails == 0 && emailOutput == "table" {
		fmt.Println("No email addresses found. Create one with 'mailvault email create'")
	}
	return nil
}

func outputEmails(emails []*EmailAddress, domain string, format string) error {
	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(emails)
	case "yaml":
		data, err := yaml.Marshal(emails)
		if err != nil {
			return fmt.Errorf("failed to marshal yaml: %w", err)
		}
		fmt.Print(string(data))
		return nil
	case "table":
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tADDRESS\tFORWARD\tCREATED")
		fmt.Fprintln(w, "--\t-------\t-------\t-------")
		for _, email := range emails {
			forward := "None"
			if len(email.ForwardAddresses) > 0 {
				if len(email.ForwardAddresses) == 1 {
					forward = email.ForwardAddresses[0]
				} else {
					forward = fmt.Sprintf("%d addresses", len(email.ForwardAddresses))
				}
			}
			address := email.LocalPart
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				email.ID[:8]+"...",
				address,
				forward,
				formatDate(email.CreatedAt))
		}
		w.Flush()
		return nil
	default:
		return fmt.Errorf("unknown output format: %s (use table|json|yaml)", format)
	}
}

func runEmailCreate(cmd *cobra.Command, args []string) error {
	config, err := requireAuth()
	if err != nil {
		return err
	}

	client := NewClient(config.ServerURL)
	client.SetToken(config.AccessToken)

	// Positional args enforced
	d, err := client.ResolveDomainReference(args[0])
	if err != nil {
		return fmt.Errorf("failed to resolve domain: %w", err)
	}
	emailLocalPart = args[1]

	req := CreateEmailRequest{
		LocalPart:        emailLocalPart,
		ForwardAddresses: emailForwardTo,
	}

	// Display what we're creating
	fmt.Printf("Creating email address '%s@%s'...\n", emailLocalPart, d.Domain)

	email, err := client.CreateEmailAddress(d.ID, req)
	if err != nil {
		return fmt.Errorf("failed to create email address: %w", err)
	}

	fmt.Printf("✓ Email address created successfully!\n\n")
	fmt.Printf("Email Details:\n")
	fmt.Printf("  ID:          %s\n", email.ID)
	fmt.Printf("  Address:     %s@%s\n", emailLocalPart, d.Domain)

	if len(email.ForwardAddresses) > 0 {
		fmt.Printf("  Forward to:  %s\n", strings.Join(email.ForwardAddresses, ", "))
	}

	return nil
}

func runEmailDelete(cmd *cobra.Command, args []string) error {
	config, err := requireAuth()
	if err != nil {
		return err
	}

	domainRef := args[0]
	emailID := args[1]

	client := NewClient(config.ServerURL)
	client.SetToken(config.AccessToken)

	d, err := client.ResolveDomainReference(domainRef)
	if err != nil {
		return fmt.Errorf("failed to resolve domain: %w", err)
	}

	// Confirmation prompt unless force flag is used
	if !emailForceDelete {
		addressDisplay := emailID
		fmt.Printf("Are you sure you want to delete email address '%s' from domain '%s'?\n", addressDisplay, d.Domain)
		fmt.Print("Type 'yes' to confirm: ")

		var confirmation string
		if _, err := fmt.Scanln(&confirmation); err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}

		if strings.ToLower(confirmation) != "yes" {
			fmt.Println("Email deletion cancelled")
			return nil
		}
	}

	if err := client.DeleteEmailAddress(d.ID, emailID); err != nil {
		return fmt.Errorf("failed to delete email address: %w", err)
	}

	fmt.Printf("✓ Email address deleted successfully\n")
	return nil
}
