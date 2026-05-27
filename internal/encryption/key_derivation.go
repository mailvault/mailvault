package encryption

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

const (
	// Argon2id parameters
	ArgonTime    = 1         // Number of iterations
	ArgonMemory  = 64 * 1024 // Memory usage in KB (64MB)
	ArgonThreads = 4         // Number of threads
	ArgonKeyLen  = 32        // Length of derived key
	SaltLength   = 16        // Length of salt in bytes
)

// DerivedKey represents a key derived from a password
type DerivedKey struct {
	Key  []byte
	Salt []byte
}

// DeriveKeyFromPassword derives a cryptographic key from a password using Argon2id
// This is used for client-side key derivation to encrypt domain private keys
func DeriveKeyFromPassword(password string, salt []byte) (*DerivedKey, error) {
	if password == "" {
		return nil, fmt.Errorf("password cannot be empty")
	}

	// Generate salt if not provided
	if salt == nil {
		salt = make([]byte, SaltLength)
		if _, err := rand.Read(salt); err != nil {
			return nil, fmt.Errorf("failed to generate salt: %w", err)
		}
	}

	// Derive key using Argon2id
	key := argon2.IDKey(
		[]byte(password),
		salt,
		ArgonTime,
		ArgonMemory,
		ArgonThreads,
		ArgonKeyLen,
	)

	return &DerivedKey{
		Key:  key,
		Salt: salt,
	}, nil
}

// EncryptPrivateKeyWithPassword encrypts a private key using a password-derived key
func EncryptPrivateKeyWithPassword(privateKey []byte, password string) (string, error) {
	if len(privateKey) != KeySize {
		return "", ErrInvalidKeySize
	}

	// Derive key from password (fresh salt)
	derivedKey, err := DeriveKeyFromPassword(password, nil)
	if err != nil {
		return "", fmt.Errorf("failed to derive key: %w", err)
	}

	// Encrypt with ChaCha20-Poly1305 using derived key
	aead, err := chacha20poly1305.New(derivedKey.Key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	nonce := make([]byte, NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := aead.Seal(nil, nonce, privateKey, nil) // #nosec G407 -- nonce is generated via crypto/rand above, not hardcoded

	// Encode fields for storage
	result := EncryptedPrivateKey{
		Salt:                base64.StdEncoding.EncodeToString(derivedKey.Salt),
		Nonce:               base64.StdEncoding.EncodeToString(nonce),
		EncryptedPrivateKey: base64.StdEncoding.EncodeToString(ciphertext),
	}

	return result.Serialize()
}

// DecryptPrivateKeyWithPassword decrypts a private key using a password-derived key
func DecryptPrivateKeyWithPassword(encryptedData, password string) ([]byte, error) {
	// Deserialize the encrypted private key
	epK, err := DeserializeEncryptedPrivateKey(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize encrypted private key: %w", err)
	}

	// Decode salt
	salt, err := base64.StdEncoding.DecodeString(epK.Salt)
	if err != nil {
		return nil, fmt.Errorf("failed to decode salt: %w", err)
	}

	// Derive key from password using the stored salt
	derivedKey, err := DeriveKeyFromPassword(password, salt)
	if err != nil {
		return nil, fmt.Errorf("failed to derive key: %w", err)
	}

	// Require nonce (new format); legacy format cannot be decrypted
	if epK.Nonce == "" {
		return nil, fmt.Errorf("encrypted private key uses an unsupported legacy format; please regenerate your keys")
	}

	nonce, err := base64.StdEncoding.DecodeString(epK.Nonce)
	if err != nil {
		return nil, fmt.Errorf("failed to decode nonce: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(epK.EncryptedPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	aead, err := chacha20poly1305.New(derivedKey.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt private key: %w", err)
	}

	if len(plaintext) != KeySize {
		return nil, fmt.Errorf("decrypted private key has invalid size")
	}

	return plaintext, nil
}

// EncryptedPrivateKey represents an encrypted private key with its salt
type EncryptedPrivateKey struct {
	Salt                string `json:"salt"`
	Nonce               string `json:"nonce,omitempty"`
	EncryptedPrivateKey string `json:"encrypted_private_key"`
}

// Serialize converts EncryptedPrivateKey to JSON string
func (epk *EncryptedPrivateKey) Serialize() (string, error) {
	jsonData, err := json.Marshal(epk)
	if err != nil {
		return "", fmt.Errorf("failed to serialize encrypted private key: %w", err)
	}
	return string(jsonData), nil
}

// DeserializeEncryptedPrivateKey converts JSON string to EncryptedPrivateKey
func DeserializeEncryptedPrivateKey(data string) (*EncryptedPrivateKey, error) {
	// Simple JSON parsing for the specific format
	// This is a simplified parser for our specific use case
	if len(data) < 50 { // Minimum reasonable length
		return nil, fmt.Errorf("encrypted private key data too short")
	}

	var epk EncryptedPrivateKey
	if err := json.Unmarshal([]byte(data), &epk); err != nil {
		return nil, fmt.Errorf("failed to deserialize encrypted private key: %w", err)
	}

	return &epk, nil
}

// GenerateDomainKeyPair generates a key pair for a domain and encrypts the private key with a password
func GenerateDomainKeyPair(userPassword string) (*KeyPair, string, error) {
	// Generate the actual key pair
	keyPair, err := GenerateKeyPair()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Encrypt the private key with the user's password
	encryptedPrivateKey, err := EncryptPrivateKeyWithPassword(keyPair.PrivateKey, userPassword)
	if err != nil {
		return nil, "", fmt.Errorf("failed to encrypt private key: %w", err)
	}

	// Return only public key in the KeyPair (for security)
	publicKeyPair := &KeyPair{
		PublicKey:  keyPair.PublicKey,
		PrivateKey: nil, // Don't return the private key in plaintext
	}

	return publicKeyPair, encryptedPrivateKey, nil
}
