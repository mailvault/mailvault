package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var emailCmd = &cobra.Command{
	Use:   "email",
	Short: "Email address management",
	Long:  "Manage email addresses for your domains.",
}

var emailListCmd = &cobra.Command{
	Use:   "list [domain-id]",
	Short: "List email addresses",
	Long:  "List email addresses for a specific domain or all domains.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runEmailList,
}

var emailCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new email address",
	Long:  "Create a new email address for a domain.",
	RunE:  runEmailCreate,
}

var emailDeleteCmd = &cobra.Command{
	Use:   "delete <domain-id> <email-id>",
	Short: "Delete an email address",
	Long:  "Delete an email address from a domain.",
	Args:  cobra.ExactArgs(2),
	RunE:  runEmailDelete,
}

var (
	emailDomainID       string
	emailLocalPart      string
	emailCatchAll       bool
	emailForwardTo      []string
	emailForceDelete    bool
)

func init() {
	emailCmd.AddCommand(emailListCmd)
	emailCmd.AddCommand(emailCreateCmd)
	emailCmd.AddCommand(emailDeleteCmd)

	// Create command flags
	emailCreateCmd.Flags().StringVarP(&emailDomainID, "domain", "d", "", "domain ID (required)")
	emailCreateCmd.Flags().StringVarP(&emailLocalPart, "address", "a", "", "local part of email address (e.g., 'hello' for hello@domain.com)")
	emailCreateCmd.Flags().BoolVar(&emailCatchAll, "catch-all", false, "make this a catch-all address")
	emailCreateCmd.Flags().StringSliceVarP(&emailForwardTo, "forward", "f", []string{}, "forward emails to these addresses (comma-separated)")
	
	emailCreateCmd.MarkFlagRequired("domain")

	// Delete command flags
	emailDeleteCmd.Flags().BoolVarP(&emailForceDelete, "force", "f", false, "force deletion without confirmation")
}

func runEmailList(cmd *cobra.Command, args []string) error {
	config, err := requireAuth()
	if err != nil {
		return err
	}

	client := NewClient(config.ServerURL)
	client.SetToken(config.AccessToken)

	// If domain ID is provided, list emails for that domain only
	if len(args) > 0 {
		domainID := args[0]
		return listEmailsForDomain(client, domainID)
	}

	// Otherwise, list emails for all domains
	domains, err := client.ListDomains()
	if err != nil {
		return fmt.Errorf("failed to list domains: %w", err)
	}

	if len(domains) == 0 {
		fmt.Println("No domains found. Create one with 'mailvault domain create'")
		return nil
	}

	totalEmails := 0
	for _, domain := range domains {
		emails, err := client.ListEmailAddresses(domain.ID)
		if err != nil {
			fmt.Printf("Warning: failed to list emails for domain %s: %v\n", domain.Domain, err)
			continue
		}

		if len(emails) > 0 {
			fmt.Printf("\n--- %s ---\n", domain.Domain)
			printEmailTable(emails)
			totalEmails += len(emails)
		}
	}

	if totalEmails == 0 {
		fmt.Println("No email addresses found. Create one with 'mailvault email create'")
	} else {
		fmt.Printf("\nTotal: %d email addresses\n", totalEmails)
	}

	return nil
}

func listEmailsForDomain(client *Client, domainID string) error {
	// Get domain details for display
	domain, err := client.GetDomain(domainID)
	if err != nil {
		return fmt.Errorf("failed to get domain details: %w", err)
	}

	emails, err := client.ListEmailAddresses(domainID)
	if err != nil {
		return fmt.Errorf("failed to list email addresses: %w", err)
	}

	if len(emails) == 0 {
		fmt.Printf("No email addresses found for domain %s\n", domain.Domain)
		fmt.Println("Create one with 'mailvault email create'")
		return nil
	}

	fmt.Printf("Email addresses for %s:\n", domain.Domain)
	printEmailTable(emails)

	return nil
}

func printEmailTable(emails []*EmailAddress) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tADDRESS\tCATCH-ALL\tFORWARD\tCREATED")
	fmt.Fprintln(w, "--\t-------\t---------\t-------\t-------")

	for _, email := range emails {
		catchAll := "No"
		if email.IsCatchAll {
			catchAll = "Yes"
		}

		forward := "None"
		if len(email.ForwardAddresses) > 0 {
			if len(email.ForwardAddresses) == 1 {
				forward = email.ForwardAddresses[0]
			} else {
				forward = fmt.Sprintf("%d addresses", len(email.ForwardAddresses))
			}
		}

		address := email.LocalPart
		if email.IsCatchAll {
			address = "*"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			email.ID[:8]+"...", // Truncate ID for display
			address,
			catchAll,
			forward,
			formatDate(email.CreatedAt))
	}

	w.Flush()
}

func runEmailCreate(cmd *cobra.Command, args []string) error {
	config, err := requireAuth()
	if err != nil {
		return err
	}

	client := NewClient(config.ServerURL)
	client.SetToken(config.AccessToken)

	// Get domain details for display
	domain, err := client.GetDomain(emailDomainID)
	if err != nil {
		return fmt.Errorf("failed to get domain details: %w", err)
	}

	// Validate input
	if !emailCatchAll && emailLocalPart == "" {
		fmt.Print("Local part (e.g., 'hello' for hello@" + domain.Domain + "): ")
		if _, err := fmt.Scanln(&emailLocalPart); err != nil {
			return fmt.Errorf("failed to read local part: %w", err)
		}
	}

	if emailCatchAll && emailLocalPart != "" {
		return fmt.Errorf("cannot specify both --catch-all and --address")
	}

	req := CreateEmailRequest{
		LocalPart:        emailLocalPart,
		IsCatchAll:       emailCatchAll,
		ForwardAddresses: emailForwardTo,
	}

	// Display what we're creating
	if emailCatchAll {
		fmt.Printf("Creating catch-all address for domain '%s'...\n", domain.Domain)
	} else {
		fmt.Printf("Creating email address '%s@%s'...\n", emailLocalPart, domain.Domain)
	}

	email, err := client.CreateEmailAddress(emailDomainID, req)
	if err != nil {
		return fmt.Errorf("failed to create email address: %w", err)
	}

	fmt.Printf("✓ Email address created successfully!\n\n")
	fmt.Printf("Email Details:\n")
	fmt.Printf("  ID:          %s\n", email.ID)
	
	if email.IsCatchAll {
		fmt.Printf("  Address:     *@%s (catch-all)\n", domain.Domain)
	} else {
		fmt.Printf("  Address:     %s@%s\n", email.LocalPart, domain.Domain)
	}
	
	fmt.Printf("  Catch-all:   %t\n", email.IsCatchAll)
	
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

	domainID := args[0]
	emailID := args[1]

	client := NewClient(config.ServerURL)
	client.SetToken(config.AccessToken)

	// Get domain and email details for confirmation
	domain, err := client.GetDomain(domainID)
	if err != nil {
		return fmt.Errorf("failed to get domain details: %w", err)
	}

	emails, err := client.ListEmailAddresses(domainID)
	if err != nil {
		return fmt.Errorf("failed to list email addresses: %w", err)
	}

	var targetEmail *EmailAddress
	for _, email := range emails {
		if email.ID == emailID {
			targetEmail = email
			break
		}
	}

	if targetEmail == nil {
		return fmt.Errorf("email address not found")
	}

	// Confirmation prompt unless force flag is used
	if !emailForceDelete {
		addressDisplay := targetEmail.LocalPart + "@" + domain.Domain
		if targetEmail.IsCatchAll {
			addressDisplay = "*@" + domain.Domain + " (catch-all)"
		}

		fmt.Printf("Are you sure you want to delete email address '%s'?\n", addressDisplay)
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

	if err := client.DeleteEmailAddress(domainID, emailID); err != nil {
		return fmt.Errorf("failed to delete email address: %w", err)
	}

	fmt.Printf("✓ Email address deleted successfully\n")
	return nil
}