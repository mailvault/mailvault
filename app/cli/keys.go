package cli

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"mailvault/internal/encryption"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// KeyMetadata stores non-sensitive information about keys
type KeyMetadata struct {
	Domain    string    `json:"domain"`
	CreatedAt time.Time `json:"created_at"`
	PublicKey string    `json:"public_key"`
}

// KeyStorage represents the local key storage structure
type KeyStorage struct {
	Version   int           `json:"version"`
	Keys      []KeyMetadata `json:"keys"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// EncryptedKeyFile represents a key file stored locally
type EncryptedKeyFile struct {
	Domain              string    `json:"domain"`
	EncryptedPrivateKey string    `json:"encrypted_private_key,omitempty"`
	PrivateKeyPlain     string    `json:"private_key_plain,omitempty"`
	PublicKey           string    `json:"public_key"`
	CreatedAt           time.Time `json:"created_at"`
}

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage encryption keys",
	Long: `Manage encryption keys for your domains.
	
Keys are stored locally. We strongly recommend protecting your private key with a password (default).
Use --no-password only for development or low-risk scenarios.`,
}

var keysGenerateCmd = &cobra.Command{
	Use:   "generate [domain]",
	Short: "Generate new encryption keys for a domain",
	Long: `Generate new X25519 encryption keys for a domain.
	
By default, the private key is encrypted with a password you choose and stored locally.
Use --no-password to store the private key unencrypted (base64) for development convenience.

Examples:
  mailvault keys generate example.com
  mailvault keys generate myapp.dev --no-password`,
	Args: cobra.ExactArgs(1),
	RunE: runKeysGenerate,
}

var keysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all managed keys",
	Long:  `List all locally stored encryption keys.`,
	RunE:  runKeysList,
}

var keysExportCmd = &cobra.Command{
	Use:   "export [domain]",
	Short: "Export public key for a domain",
	Long: `Export the public key for a domain.
	
This is useful when creating domains or sharing your public key.

Examples:
  mailvault keys export example.com
  mailvault keys export example.com --format hex`,
	Args: cobra.ExactArgs(1),
	RunE: runKeysExport,
}

var keysDeleteCmd = &cobra.Command{
	Use:   "delete [domain]",
	Short: "Delete keys for a domain",
	Long: `Delete the encryption keys for a domain.
	
WARNING: This will permanently delete your private key.
Make sure you have backups before proceeding.`,
	Args: cobra.ExactArgs(1),
	RunE: runKeysDelete,
}

var (
	noPasswordFlag bool
)

func init() {
	// Global flags specific to keys
	keysGenerateCmd.Flags().BoolVar(&noPasswordFlag, "no-password", false, "do not encrypt the private key (store in plaintext base64). Not recommended for production")
	keysExportCmd.Flags().String("format", "formatted", "output format (formatted, hex, base64)")

	// Add subcommands
	keysCmd.AddCommand(keysGenerateCmd)
	keysCmd.AddCommand(keysListCmd)
	keysCmd.AddCommand(keysExportCmd)
	keysCmd.AddCommand(keysDeleteCmd)
}

func runKeysGenerate(cmd *cobra.Command, args []string) error {
	domain := strings.ToLower(args[0])

	// Check if keys already exist
	if keyExists(domain) {
		return fmt.Errorf("keys for domain %s already exist. Use 'keys delete %s' first if you want to regenerate", domain, domain)
	}

	// Generate key pair
	keyPair, err := encryption.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Format public key
	formattedPublicKey := encryption.FormatPublicKey(keyPair.PublicKey)

	keyFile := EncryptedKeyFile{
		Domain:    domain,
		PublicKey: formattedPublicKey,
		CreatedAt: time.Now().UTC(),
	}

	if noPasswordFlag {
		// Store plaintext private key (base64)
		keyFile.PrivateKeyPlain = base64.StdEncoding.EncodeToString(keyPair.PrivateKey)
		if verbose {
			fmt.Println("Storing private key WITHOUT password (base64). Protect your keys directory (0600)!")
		}
	} else {
		// Prompt for password and confirmation
		password, err := promptPassword(fmt.Sprintf("Enter password for %s keys: ", domain))
		if err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}
		confirmPassword, err := promptPassword("Confirm password: ")
		if err != nil {
			return fmt.Errorf("failed to read password confirmation: %w", err)
		}
		if password != confirmPassword {
			return fmt.Errorf("passwords do not match")
		}

		encryptedPrivateKey, err := encryption.EncryptPrivateKeyWithPassword(keyPair.PrivateKey, password)
		if err != nil {
			return fmt.Errorf("failed to encrypt private key: %w", err)
		}
		keyFile.EncryptedPrivateKey = encryptedPrivateKey
	}

	if err := saveKeyFile(domain, keyFile); err != nil {
		return fmt.Errorf("failed to save key file: %w", err)
	}

	// Update metadata
	if err := updateKeyMetadata(domain, formattedPublicKey); err != nil {
		return fmt.Errorf("failed to update key metadata: %w", err)
	}

	fmt.Printf("Successfully generated keys for domain: %s\n", domain)
	fmt.Printf("Public key: %s\n", formattedPublicKey)
	if noPasswordFlag {
		fmt.Printf("\nWARNING: Private key stored without password. Protect your keys directory: %s\n", getKeysDir())
	} else {
		fmt.Printf("\nYour private key is protected with your password.\n")
	}

	return nil
}

func runKeysList(cmd *cobra.Command, args []string) error {
	storage, err := loadKeyStorage()
	if err != nil {
		return fmt.Errorf("failed to load key storage: %w", err)
	}

	if len(storage.Keys) == 0 {
		fmt.Println("No keys found. Use 'mailvault keys generate <domain>' to create keys.")
		return nil
	}

	fmt.Printf("%-20s %-12s %s\n", "DOMAIN", "CREATED", "PUBLIC KEY")
	fmt.Printf("%-20s %-12s %s\n", "------", "-------", "----------")

	for _, key := range storage.Keys {
		createdStr := key.CreatedAt.Format("2006-01-02")
		publicKeyShort := key.PublicKey
		if len(publicKeyShort) > 40 {
			publicKeyShort = publicKeyShort[:40] + "..."
		}
		fmt.Printf("%-20s %-12s %s\n", key.Domain, createdStr, publicKeyShort)
	}

	return nil
}

func runKeysExport(cmd *cobra.Command, args []string) error {
	domain := strings.ToLower(args[0])
	format, _ := cmd.Flags().GetString("format")

	keyFile, err := loadKeyFile(domain)
	if err != nil {
		return fmt.Errorf("failed to load key file for domain %s: %w", domain, err)
	}

	switch format {
	case "formatted":
		fmt.Println(keyFile.PublicKey)
	case "hex":
		publicKeyBytes, err := encryption.ParsePublicKey(keyFile.PublicKey)
		if err != nil {
			return fmt.Errorf("failed to parse public key: %w", err)
		}
		fmt.Printf("%x\n", publicKeyBytes)
	case "base64":
		publicKeyBytes, err := encryption.ParsePublicKey(keyFile.PublicKey)
		if err != nil {
			return fmt.Errorf("failed to parse public key: %w", err)
		}
		fmt.Printf("%s\n", publicKeyBytes)
	default:
		return fmt.Errorf("unknown format: %s (supported: formatted, hex, base64)", format)
	}

	return nil
}

func runKeysDelete(cmd *cobra.Command, args []string) error {
	domain := strings.ToLower(args[0])

	if !keyExists(domain) {
		return fmt.Errorf("no keys found for domain: %s", domain)
	}

	// Confirm deletion
	fmt.Printf("Are you sure you want to delete keys for %s? This cannot be undone.\n", domain)
	fmt.Print("Type 'yes' to confirm: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if strings.TrimSpace(response) != "yes" {
		fmt.Println("Deletion cancelled.")
		return nil
	}

	// Delete key file
	keyFilePath := filepath.Join(getKeysDir(), domain+".key")
	if err := os.Remove(keyFilePath); err != nil {
		return fmt.Errorf("failed to delete key file: %w", err)
	}

	// Remove from metadata
	if err := removeKeyFromMetadata(domain); err != nil {
		return fmt.Errorf("failed to update metadata: %w", err)
	}

	fmt.Printf("Successfully deleted keys for domain: %s\n", domain)
	return nil
}

// Helper functions

func getKeysDir() string {
	return filepath.Join(configDir, "keys")
}

func keyExists(domain string) bool {
	keyFilePath := filepath.Join(getKeysDir(), domain+".key")
	_, err := os.Stat(keyFilePath)
	return err == nil
}

func saveKeyFile(domain string, keyFile EncryptedKeyFile) error {
	keysDir := getKeysDir()
	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return fmt.Errorf("failed to create keys directory: %w", err)
	}

	keyFilePath := filepath.Join(keysDir, domain+".key")
	jsonData, err := json.MarshalIndent(keyFile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal key file: %w", err)
	}

	if err := os.WriteFile(keyFilePath, jsonData, 0600); err != nil {
		return fmt.Errorf("failed to write key file: %w", err)
	}

	return nil
}

func loadKeyFile(domain string) (*EncryptedKeyFile, error) {
	keyFilePath := filepath.Join(getKeysDir(), domain+".key")
	data, err := os.ReadFile(keyFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	var keyFile EncryptedKeyFile
	if err := json.Unmarshal(data, &keyFile); err != nil {
		return nil, fmt.Errorf("failed to unmarshal key file: %w", err)
	}

	return &keyFile, nil
}

func loadKeyStorage() (*KeyStorage, error) {
	storageFilePath := filepath.Join(getKeysDir(), "keyring.json")

	// Create empty storage if file doesn't exist
	if _, err := os.Stat(storageFilePath); os.IsNotExist(err) {
		return &KeyStorage{
			Version:   1,
			Keys:      []KeyMetadata{},
			UpdatedAt: time.Now().UTC(),
		}, nil
	}

	data, err := os.ReadFile(storageFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read storage file: %w", err)
	}

	var storage KeyStorage
	if err := json.Unmarshal(data, &storage); err != nil {
		return nil, fmt.Errorf("failed to unmarshal storage file: %w", err)
	}

	return &storage, nil
}

func saveKeyStorage(storage *KeyStorage) error {
	keysDir := getKeysDir()
	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return fmt.Errorf("failed to create keys directory: %w", err)
	}

	storage.UpdatedAt = time.Now().UTC()
	storageFilePath := filepath.Join(keysDir, "keyring.json")

	jsonData, err := json.MarshalIndent(storage, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal storage: %w", err)
	}

	if err := os.WriteFile(storageFilePath, jsonData, 0600); err != nil {
		return fmt.Errorf("failed to write storage file: %w", err)
	}

	return nil
}

func updateKeyMetadata(domain, publicKey string) error {
	storage, err := loadKeyStorage()
	if err != nil {
		return err
	}

	// Remove existing entry if present
	for i, key := range storage.Keys {
		if key.Domain == domain {
			storage.Keys = append(storage.Keys[:i], storage.Keys[i+1:]...)
			break
		}
	}

	// Add new entry
	storage.Keys = append(storage.Keys, KeyMetadata{
		Domain:    domain,
		CreatedAt: time.Now().UTC(),
		PublicKey: publicKey,
	})

	return saveKeyStorage(storage)
}

func removeKeyFromMetadata(domain string) error {
	storage, err := loadKeyStorage()
	if err != nil {
		return err
	}

	// Remove entry
	for i, key := range storage.Keys {
		if key.Domain == domain {
			storage.Keys = append(storage.Keys[:i], storage.Keys[i+1:]...)
			break
		}
	}

	return saveKeyStorage(storage)
}

func promptPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", err
	}
	fmt.Println() // Add newline after password input
	return string(bytePassword), nil
}
