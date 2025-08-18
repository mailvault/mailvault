package cli

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"mailvault/internal/encryption"

	"github.com/spf13/cobra"
)

var inboxCmd = &cobra.Command{
	Use:   "inbox",
	Short: "View received emails",
	Long:  "View and manage received emails in your mailboxes.",
}

var inboxListCmd = &cobra.Command{
	Use:   "list [domain-name|email@domain.com] [email-name]",
	Short: "List received emails",
	Long: `List received emails for a specific email address or all addresses.

Examples:
  mailvault inbox list                    # List all emails from all domains
  mailvault inbox list example.com       # List all emails from domain
  mailvault inbox list example.com hello # List emails from hello@example.com
  mailvault inbox list hello@example.com # List emails from hello@example.com`,
	Args: cobra.MaximumNArgs(2),
	RunE: runInboxList,
}

var inboxShowCmd = &cobra.Command{
	Use:   "show <domain-name|email@domain.com> [email-name] <email-reference>",
	Short: "Show email details",
	Long: `Display detailed information about a specific received email.

Email reference can be:
  - Sequence number (e.g., 1, 2, 3)
  - Short ID (e.g., a1b2c3d4)
  - Full UUID

If email reference not provided, shows interactive selection.

Examples:
  mailvault inbox show example.com hello 1        # Show email #1 from hello@example.com
  mailvault inbox show hello@example.com 1        # Same as above
  mailvault inbox show example.com hello a1b2c3d4 # Show email by short ID  
  mailvault inbox show example.com hello          # Interactive selection
  mailvault inbox show example.com *              # Interactive selection for catch-all
  mailvault inbox show example.com hello 1 --decrypt # Show decrypted content`,
	Args: cobra.RangeArgs(1, 3),
	RunE: runInboxShow,
}

var inboxDecryptCmd = &cobra.Command{
	Use:   "decrypt <domain-name|email@domain.com> [email-name] <email-reference>|<email-id>",
	Short: "Decrypt and show email content",
	Long: `Decrypt and display the full content of a received email using local private keys.

Email reference can be:
  - Sequence number (e.g., 1, 2, 3)
  - Short ID (e.g., a1b2c3d4)
  - Full UUID

You can also pass a single argument that is the received email UUID to decrypt directly.

Examples:
  mailvault inbox decrypt example.com hello 1        # Decrypt email #1
  mailvault inbox decrypt hello@example.com 1        # Same as above
  mailvault inbox decrypt example.com hello a1b2c3d4 # Decrypt by short ID
  mailvault inbox decrypt 123e4567-e89b-12d3-a456-426614174000 # Decrypt by email ID`,
	Args: cobra.RangeArgs(1, 3),
	RunE: runInboxDecrypt,
}

var (
	inboxLimit      int
	inboxOffset     int
	decryptFlag     bool
	decryptPassword string
	outputPath      string
)

func init() {
	inboxCmd.AddCommand(inboxListCmd)
	inboxCmd.AddCommand(inboxShowCmd)
	inboxCmd.AddCommand(inboxDecryptCmd)

	// List command flags
	inboxListCmd.Flags().IntVarP(&inboxLimit, "limit", "l", 20, "maximum number of emails to show")
	inboxListCmd.Flags().IntVarP(&inboxOffset, "offset", "o", 0, "number of emails to skip")

	// Show command flags
	inboxShowCmd.Flags().BoolVar(&decryptFlag, "decrypt", false, "decrypt email content using local private key")
	inboxShowCmd.Flags().StringVar(&decryptPassword, "password", "", "private key password to use for decryption (less secure)")
	inboxShowCmd.Flags().StringVarP(&outputPath, "output", "O", "", "write decrypted content to file instead of terminal")

	// Decrypt command flags
	inboxDecryptCmd.Flags().StringVar(&decryptPassword, "password", "", "private key password to use for decryption (less secure)")
	inboxDecryptCmd.Flags().StringVarP(&outputPath, "output", "O", "", "write decrypted content to file instead of terminal")
}

func runInboxList(cmd *cobra.Command, args []string) error {
	config, err := requireAuth()
	if err != nil {
		return err
	}

	client := NewClient(config.ServerURL)
	client.SetToken(config.AccessToken)

	// Case 1: Show emails for specific email address
	if len(args) == 2 {
		domain, emailAddr, err := client.ResolveEmailReference(args[0], args[1])
		if err != nil {
			return fmt.Errorf("failed to resolve email address: %w", err)
		}
		return listEmailsForAddressResolved(client, domain, emailAddr)
	}

	// Case 2: Show emails for specific domain OR full email address
	if len(args) == 1 {
		// Check if it's a full email address
		if strings.Contains(args[0], "@") {
			domain, emailAddr, err := client.ResolveEmailReference(args[0], "")
			if err != nil {
				return fmt.Errorf("failed to resolve email address: %w", err)
			}
			return listEmailsForAddressResolved(client, domain, emailAddr)
		}

		// Otherwise treat as domain
		domain, err := client.ResolveDomainReference(args[0])
		if err != nil {
			return fmt.Errorf("failed to resolve domain: %w", err)
		}
		return listEmailsForDomainResolved(client, domain)
	}

	// Case 3: Show emails for all domains and addresses
	return listAllEmails(client)
}

// New resolved functions using Domain and EmailAddress objects
func listEmailsForAddressResolved(client *Client, domain *Domain, emailAddr *EmailAddress) error {
	receivedEmails, err := client.ListReceivedEmails(domain.ID, emailAddr.ID, inboxLimit, inboxOffset)
	if err != nil {
		return fmt.Errorf("failed to list received emails: %w", err)
	}

	addressDisplay := emailAddr.LocalPart + "@" + domain.Domain
	if emailAddr.IsCatchAll {
		addressDisplay = "*@" + domain.Domain + " (catch-all)"
	}

	fmt.Printf("Received emails for %s:\n", addressDisplay)

	if len(receivedEmails) == 0 {
		fmt.Println("No emails received yet")
		return nil
	}

	printReceivedEmailsTable(receivedEmails)

	if len(receivedEmails) == inboxLimit {
		fmt.Printf("\nShowing %d emails (use --offset to see more)\n", inboxLimit)
	}

	return nil
}

func listEmailsForDomainResolved(client *Client, domain *Domain) error {
	emailAddresses, err := client.ListEmailAddresses(domain.ID)
	if err != nil {
		return fmt.Errorf("failed to get email addresses: %w", err)
	}

	if len(emailAddresses) == 0 {
		fmt.Printf("No email addresses found for domain %s\n", domain.Domain)
		return nil
	}

	fmt.Printf("Received emails for domain %s:\n", domain.Domain)

	totalEmails := 0
	for _, email := range emailAddresses {
		receivedEmails, err := client.ListReceivedEmails(domain.ID, email.ID, inboxLimit, inboxOffset)
		if err != nil {
			fmt.Printf("Warning: failed to get emails for %s: %v\n", email.LocalPart, err)
			continue
		}

		if len(receivedEmails) > 0 {
			addressDisplay := email.LocalPart + "@" + domain.Domain
			if email.IsCatchAll {
				addressDisplay = "*@" + domain.Domain + " (catch-all)"
			}
			fmt.Printf("\n--- %s ---\n", addressDisplay)
			printReceivedEmailsTable(receivedEmails)
			totalEmails += len(receivedEmails)
		}
	}

	if totalEmails == 0 {
		fmt.Println("No emails received yet")
	} else {
		fmt.Printf("\nTotal: %d emails\n", totalEmails)
	}

	return nil
}

// Keep old functions for backwards compatibility
func listEmailsForAddress(client *Client, domainID, emailID string) error {
	// Get domain and email details for display
	domain, err := client.GetDomain(domainID)
	if err != nil {
		return fmt.Errorf("failed to get domain details: %w", err)
	}

	emails, err := client.ListEmailAddresses(domainID)
	if err != nil {
		return fmt.Errorf("failed to get email addresses: %w", err)
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

	receivedEmails, err := client.ListReceivedEmails(domainID, emailID, inboxLimit, inboxOffset)
	if err != nil {
		return fmt.Errorf("failed to list received emails: %w", err)
	}

	addressDisplay := targetEmail.LocalPart + "@" + domain.Domain
	if targetEmail.IsCatchAll {
		addressDisplay = "*@" + domain.Domain + " (catch-all)"
	}

	fmt.Printf("Received emails for %s:\n", addressDisplay)

	if len(receivedEmails) == 0 {
		fmt.Println("No emails received yet")
		return nil
	}

	printReceivedEmailsTable(receivedEmails)

	if len(receivedEmails) == inboxLimit {
		fmt.Printf("\nShowing %d emails (use --offset to see more)\n", inboxLimit)
	}

	return nil
}

func listEmailsForDomainInbox(client *Client, domainID string) error {
	domain, err := client.GetDomain(domainID)
	if err != nil {
		return fmt.Errorf("failed to get domain details: %w", err)
	}

	emailAddresses, err := client.ListEmailAddresses(domainID)
	if err != nil {
		return fmt.Errorf("failed to get email addresses: %w", err)
	}

	if len(emailAddresses) == 0 {
		fmt.Printf("No email addresses found for domain %s\n", domain.Domain)
		return nil
	}

	fmt.Printf("Received emails for domain %s:\n", domain.Domain)

	totalEmails := 0
	for _, email := range emailAddresses {
		receivedEmails, err := client.ListReceivedEmails(domainID, email.ID, inboxLimit, inboxOffset)
		if err != nil {
			fmt.Printf("Warning: failed to get emails for %s: %v\n", email.LocalPart, err)
			continue
		}

		if len(receivedEmails) > 0 {
			addressDisplay := email.LocalPart + "@" + domain.Domain
			if email.IsCatchAll {
				addressDisplay = "*@" + domain.Domain + " (catch-all)"
			}
			fmt.Printf("\n--- %s ---\n", addressDisplay)
			printReceivedEmailsTable(receivedEmails)
			totalEmails += len(receivedEmails)
		}
	}

	if totalEmails == 0 {
		fmt.Println("No emails received yet")
	} else {
		fmt.Printf("\nTotal: %d emails\n", totalEmails)
	}

	return nil
}

func listAllEmails(client *Client) error {
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
		emailAddresses, err := client.ListEmailAddresses(domain.ID)
		if err != nil {
			fmt.Printf("Warning: failed to get emails for domain %s: %v\n", domain.Domain, err)
			continue
		}

		for _, email := range emailAddresses {
			receivedEmails, err := client.ListReceivedEmails(domain.ID, email.ID, inboxLimit, inboxOffset)
			if err != nil {
				fmt.Printf("Warning: failed to get received emails: %v\n", err)
				continue
			}

			if len(receivedEmails) > 0 {
				addressDisplay := email.LocalPart + "@" + domain.Domain
				if email.IsCatchAll {
					addressDisplay = "*@" + domain.Domain + " (catch-all)"
				}
				fmt.Printf("\n--- %s ---\n", addressDisplay)
				printReceivedEmailsTable(receivedEmails)
				totalEmails += len(receivedEmails)
			}
		}
	}

	if totalEmails == 0 {
		fmt.Println("No emails received yet")
	} else {
		fmt.Printf("\nTotal: %d emails\n", totalEmails)
		if totalEmails == inboxLimit {
			fmt.Printf("(use --offset to see more)\n")
		}
	}

	return nil
}

func printReceivedEmailsTable(emails []*ReceivedEmail) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "#\tID\tFROM\tSUBJECT\tRECEIVED")
	fmt.Fprintln(w, "-\t--\t----\t-------\t--------")

	for _, email := range emails {
		subject := "No Subject"
		if email.Subject != "" {
			subject = email.Subject
			// Truncate long subjects
			if len(subject) > 50 {
				subject = subject[:47] + "..."
			}
		}

		fromAddr := email.FromAddress
		if len(fromAddr) > 30 {
			fromAddr = fromAddr[:27] + "..."
		}

		// Generate short ID for display
		shortIDStr := shortID(parseUUIDString(email.ID))

		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n",
			email.SequenceNumber,
			shortIDStr,
			fromAddr,
			subject,
			formatDate(email.ReceivedAt))
	}

	w.Flush()
}

func runInboxShow(cmd *cobra.Command, args []string) error {
	config, err := requireAuth()
	if err != nil {
		return err
	}

	client := NewClient(config.ServerURL)
	client.SetToken(config.AccessToken)

	var domain *Domain
	var emailAddr *EmailAddress
	var emailReference string

	// Parse arguments with smart resolution
	switch len(args) {
	case 1:
		// Full email address format: hello@example.com
		if strings.Contains(args[0], "@") {
			domain, emailAddr, err = client.ResolveEmailReference(args[0], "")
			if err != nil {
				return fmt.Errorf("failed to resolve email address: %w", err)
			}
			// Interactive mode - no email reference provided
		} else {
			return fmt.Errorf("invalid format. Use: mailvault inbox show <domain> <email> [reference] or <email@domain> [reference]")
		}
	case 2:
		// Two args: domain + email OR email@domain + reference
		if strings.Contains(args[0], "@") {
			// email@domain + reference
			domain, emailAddr, err = client.ResolveEmailReference(args[0], "")
			if err != nil {
				return fmt.Errorf("failed to resolve email address: %w", err)
			}
			emailReference = args[1]
		} else {
			// domain + email (interactive mode)
			domain, emailAddr, err = client.ResolveEmailReference(args[0], args[1])
			if err != nil {
				return fmt.Errorf("failed to resolve email address: %w", err)
			}
			// Interactive mode - no email reference provided
		}
	case 3:
		// Three args: domain + email + reference
		domain, emailAddr, err = client.ResolveEmailReference(args[0], args[1])
		if err != nil {
			return fmt.Errorf("failed to resolve email address: %w", err)
		}
		emailReference = args[2]
	default:
		return fmt.Errorf("invalid number of arguments")
	}

	var targetEmail *ReceivedEmail

	// If email reference is provided, use it to find the email
	if emailReference != "" {
		targetEmail, err = client.FindReceivedEmailByReference(domain.ID, emailAddr.ID, emailReference)
		if err != nil {
			return fmt.Errorf("failed to find email: %w", err)
		}
	} else {
		// Interactive mode - show list and let user select
		targetEmail, err = interactiveEmailSelectionResolved(client, domain, emailAddr)
		if err != nil {
			return err
		}
	}

	// Display email details
	shortIDStr := shortID(parseUUIDString(targetEmail.ID))

	fmt.Printf("Email Details:\n")
	fmt.Printf("  Sequence:     #%d\n", targetEmail.SequenceNumber)
	fmt.Printf("  Short ID:     %s\n", shortIDStr)
	fmt.Printf("  Full ID:      %s\n", targetEmail.ID)
	fmt.Printf("  From:         %s\n", targetEmail.FromAddress)

	if targetEmail.Subject != "" {
		fmt.Printf("  Subject:      %s\n", targetEmail.Subject)
	} else {
		fmt.Printf("  Subject:      (No Subject)\n")
	}

	fmt.Printf("  Received At:  %s\n", targetEmail.ReceivedAt)

	addressDisplay := emailAddr.LocalPart + "@" + domain.Domain
	if emailAddr.IsCatchAll {
		addressDisplay = "*@" + domain.Domain + " (catch-all)"
	}
	fmt.Printf("  Delivered To: %s\n", addressDisplay)

	fmt.Printf("\n")

	// Display email body with optional decryption
	if decryptFlag {
		err = displayDecryptedEmailBody(targetEmail.EncryptedBody, domain.Domain, decryptPassword, outputPath)
		if err != nil {
			fmt.Printf("❌ Failed to decrypt email: %v\n", err)
			fmt.Printf("📧 Showing encrypted content instead:\n")
			displayEmailBody(targetEmail.EncryptedBody)
		}
	} else {
		displayEmailBody(targetEmail.EncryptedBody)
	}

	return nil
}

func interactiveEmailSelectionResolved(client *Client, domain *Domain, emailAddr *EmailAddress) (*ReceivedEmail, error) {
	// Get received emails
	emails, err := client.ListReceivedEmails(domain.ID, emailAddr.ID, 20, 0) // Show first 20
	if err != nil {
		return nil, fmt.Errorf("failed to list received emails: %w", err)
	}

	if len(emails) == 0 {
		return nil, fmt.Errorf("no emails found")
	}

	addressDisplay := emailAddr.LocalPart + "@" + domain.Domain
	if emailAddr.IsCatchAll {
		addressDisplay = "*@" + domain.Domain + " (catch-all)"
	}

	fmt.Printf("Select email to view from %s:\n\n", addressDisplay)

	// Display numbered list
	for i, email := range emails {
		subject := email.Subject
		if subject == "" {
			subject = "(No Subject)"
		}
		if len(subject) > 50 {
			subject = subject[:47] + "..."
		}

		shortIDStr := shortID(parseUUIDString(email.ID))

		fmt.Printf("%2d) #%-3d %s - %s - %s\n",
			i+1,
			email.SequenceNumber,
			shortIDStr,
			email.FromAddress,
			subject)
	}

	fmt.Printf("\nEnter selection (1-%d): ", len(emails))

	var selection int
	if _, err := fmt.Scanln(&selection); err != nil {
		return nil, fmt.Errorf("failed to read selection: %w", err)
	}

	if selection < 1 || selection > len(emails) {
		return nil, fmt.Errorf("invalid selection")
	}

	return emails[selection-1], nil
}

func runInboxDecrypt(cmd *cobra.Command, args []string) error {
	config, err := requireAuth()
	if err != nil {
		return err
	}

	client := NewClient(config.ServerURL)
	client.SetToken(config.AccessToken)

	var domain *Domain
	var emailAddr *EmailAddress
	var emailReference string

	// Parse arguments similar to runInboxShow but require email reference
	switch len(args) {
	case 1:
		if strings.Contains(args[0], "@") {
			return fmt.Errorf("invalid format. Use: mailvault inbox decrypt <email@domain> <reference> or <domain> <email> <reference>")
		}
		received, err := client.GetReceivedEmailByID(args[0])
		if err != nil {
			return fmt.Errorf("failed to fetch email by ID: %w", err)
		}
		domain, emailAddr, err = client.ResolveEmailReference(received.EmailAddress, "")
		if err != nil {
			return fmt.Errorf("failed to resolve email address: %w", err)
		}
		emailReference = args[0]

	case 2:
		// email@domain + reference
		if !strings.Contains(args[0], "@") {
			return fmt.Errorf("invalid format. Use: mailvault inbox decrypt <email@domain> <reference> or <domain> <email> <reference>")
		}
		domain, emailAddr, err = client.ResolveEmailReference(args[0], "")
		if err != nil {
			return fmt.Errorf("failed to resolve email address: %w", err)
		}
		emailReference = args[1]

	case 3:
		// domain + email + reference
		domain, emailAddr, err = client.ResolveEmailReference(args[0], args[1])
		if err != nil {
			return fmt.Errorf("failed to resolve email address: %w", err)
		}
		emailReference = args[2]
	default:
		return fmt.Errorf("email reference is required for decryption")
	}

	// Find the email
	targetEmail, err := client.FindReceivedEmailByReference(domain.ID, emailAddr.ID, emailReference)
	if err != nil {
		return fmt.Errorf("failed to find email: %w", err)
	}

	// Display email header info
	shortIDStr := shortID(parseUUIDString(targetEmail.ID))
	fmt.Printf("Decrypting Email #%d (%s)\n", targetEmail.SequenceNumber, shortIDStr)
	fmt.Printf("From: %s\n", targetEmail.FromAddress)
	if targetEmail.Subject != "" {
		fmt.Printf("Subject: %s\n", targetEmail.Subject)
	}
	fmt.Printf("Domain: %s\n", domain.Domain)
	fmt.Printf("\n")

	// Decrypt and display content
	return displayDecryptedEmailBody(targetEmail.EncryptedBody, domain.Domain, decryptPassword, outputPath)
}

// displayDecryptedEmailBody decrypts and displays email content
func displayDecryptedEmailBody(encryptedBody, domainName, providedPassword, outputPath string) error {
	// Load private key for the domain
	keyFile, err := loadKeyFile(domainName)
	if err != nil {
		return fmt.Errorf("no local private key found for domain %s. Generate keys with 'mailvault keys generate %s'", domainName, domainName)
	}

	// Check if content looks like our encrypted format
	if !strings.Contains(encryptedBody, `"version"`) || !strings.Contains(encryptedBody, `"ephemeral_public_key"`) {
		return fmt.Errorf("email content doesn't appear to be encrypted with MailVault encryption")
	}

	// Deserialize the encrypted data
	encryptedData, err := encryption.DeserializeEncryptedData(encryptedBody)
	if err != nil {
		return fmt.Errorf("failed to deserialize encrypted data: %w", err)
	}

	// Resolve private key: plaintext (preferred when present) or password-protected
	var privateKey []byte
	if keyFile.PrivateKeyPlain != "" {
		pk, err := base64.StdEncoding.DecodeString(keyFile.PrivateKeyPlain)
		if err != nil {
			return fmt.Errorf("failed to decode plaintext private key: %w", err)
		}
		privateKey = pk
	} else {
		var password string
		if providedPassword != "" {
			password = providedPassword
		} else {
			// Prompt for password to decrypt private key
			password, err = promptPassword(fmt.Sprintf("Enter password for %s private key: ", domainName))
			if err != nil {
				return fmt.Errorf("failed to read password: %w", err)
			}
		}
		pk, err := encryption.DecryptPrivateKeyWithPassword(keyFile.EncryptedPrivateKey, password)
		if err != nil {
			return fmt.Errorf("failed to decrypt private key (wrong password?): %w", err)
		}
		privateKey = pk
	}

	// Decrypt email content
	decryptedContent, err := encryption.Decrypt(encryptedData, privateKey)
	if err != nil {
		return fmt.Errorf("failed to decrypt email content: %w", err)
	}

	// If output path provided, write to file instead of printing
	if outputPath != "" {
		if err := os.WriteFile(outputPath, decryptedContent, 0600); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Printf("✅ Decrypted content saved to %s\n", outputPath)
		return nil
	}

	// Display decrypted content
	fmt.Printf("🔓 Decrypted Email Content:\n")
	fmt.Printf("┌─────────────────────────────────────────────────────┐\n")

	// Split into lines and display with proper formatting
	lines := strings.Split(string(decryptedContent), "\n")
	for _, line := range lines {
		if len(line) > 50 {
			// Wrap long lines
			for i := 0; i < len(line); i += 50 {
				end := i + 50
				if end > len(line) {
					end = len(line)
				}
				fmt.Printf("│ %-51s │\n", line[i:end])
			}
		} else {
			fmt.Printf("│ %-51s │\n", line)
		}
	}

	fmt.Printf("└─────────────────────────────────────────────────────┘\n")
	fmt.Printf("\n✅ Email successfully decrypted using local private key\n")

	return nil
}

// displayEmailBody formats and displays email body content
func displayEmailBody(body string) {
	// Check if it looks like encrypted content (base64, PGP, etc.)
	isEncrypted := isLikelyEncrypted(body)

	if isEncrypted {
		fmt.Printf("📧 Email Content (Encrypted):\n")
		fmt.Printf("╭─────────────────────────────────────────────────────╮\n")
		fmt.Printf("│ This email content is encrypted and needs to be    │\n")
		fmt.Printf("│ decrypted with your domain's private key.          │\n")
		fmt.Printf("╰─────────────────────────────────────────────────────╯\n")
		fmt.Printf("\nEncrypted Content Preview:\n")

		// Show first and last few characters to indicate it's encrypted data
		if len(body) > 100 {
			fmt.Printf("%s...\n...[%d characters total]...\n...%s\n",
				body[:50], len(body), body[len(body)-50:])
		} else {
			fmt.Printf("%s\n", body)
		}
	} else {
		// Display as plain text (for development/testing)
		fmt.Printf("📧 Email Content (Plain Text):\n")
		fmt.Printf("┌─────────────────────────────────────────────────────┐\n")

		// Split into lines and display with proper formatting
		lines := strings.Split(body, "\n")
		for _, line := range lines {
			if len(line) > 50 {
				// Wrap long lines
				for i := 0; i < len(line); i += 50 {
					end := i + 50
					if end > len(line) {
						end = len(line)
					}
					fmt.Printf("│ %-51s │\n", line[i:end])
				}
			} else {
				fmt.Printf("│ %-51s │\n", line)
			}
		}

		fmt.Printf("└─────────────────────────────────────────────────────┘\n")
	}

	// Add usage instructions
	fmt.Printf("\n💡 Usage:\n")
	if isEncrypted {
		fmt.Printf("   • Use your domain's private key to decrypt this content\n")
		fmt.Printf("   • Email encryption ensures privacy and security\n")
	} else {
		fmt.Printf("   • This is plain text content (development mode)\n")
		fmt.Printf("   • In production, emails should be encrypted\n")
	}
}

// isLikelyEncrypted checks if content appears to be encrypted
func isLikelyEncrypted(content string) bool {
	// Simple heuristics to detect encrypted content
	content = strings.TrimSpace(content)

	// Check for our JSON-based encryption format
	if strings.Contains(content, `"version"`) &&
		strings.Contains(content, `"ephemeral_public_key"`) &&
		strings.Contains(content, `"ciphertext"`) {
		return true
	}

	// Check for PGP markers
	if strings.Contains(content, "-----BEGIN PGP MESSAGE-----") ||
		strings.Contains(content, "-----BEGIN ENCRYPTED MESSAGE-----") {
		return true
	}

	// Check if it looks like base64 (high ratio of alphanumeric chars)
	if len(content) > 50 {
		alphanumeric := 0
		for _, r := range content {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
				(r >= '0' && r <= '9') || r == '+' || r == '/' || r == '=' {
				alphanumeric++
			}
		}
		// If more than 90% is base64-like, consider it encrypted
		if float64(alphanumeric)/float64(len(content)) > 0.9 {
			return true
		}
	}

	// Default to plain text for now (development mode)
	return false
}
