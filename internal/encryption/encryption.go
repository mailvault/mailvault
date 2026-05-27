package encryption

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

const (
	// KeySize is the size of X25519 keys in bytes
	KeySize = 32
	// NonceSize is the size of ChaCha20-Poly1305 nonces in bytes
	NonceSize = 12
	// PublicKeyPrefix is used to identify public keys in storage
	PublicKeyPrefix = "x25519:"
)

var (
	ErrInvalidKeySize   = errors.New("invalid key size")
	ErrInvalidNonceSize = errors.New("invalid nonce size")
	ErrInvalidPublicKey = errors.New("invalid public key format")
	ErrDecryptionFailed = errors.New("decryption failed")
	ErrEncryptionFailed = errors.New("encryption failed")
)

// KeyPair represents an X25519 key pair
type KeyPair struct {
	PublicKey  []byte
	PrivateKey []byte
}

// GenerateKeyPair generates a new X25519 key pair
func GenerateKeyPair() (*KeyPair, error) {
	privateKey := make([]byte, KeySize)
	if _, err := rand.Read(privateKey); err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	publicKey, err := curve25519.X25519(privateKey, curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("failed to generate public key: %w", err)
	}

	return &KeyPair{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	}, nil
}

// EncryptedData represents encrypted data with metadata
type EncryptedData struct {
	// EphemeralPublicKey is the ephemeral public key used for this encryption
	EphemeralPublicKey []byte
	// Nonce is the ChaCha20-Poly1305 nonce
	Nonce []byte
	// Ciphertext is the encrypted data
	Ciphertext []byte
}

// Encrypt encrypts data using hybrid X25519 + ChaCha20-Poly1305 encryption
// This implements the following scheme:
// 1. Generate ephemeral X25519 key pair
// 2. Perform ECDH with recipient's public key to derive shared secret
// 3. Use shared secret as ChaCha20-Poly1305 key
// 4. Encrypt data with ChaCha20-Poly1305
// 5. Return ephemeral public key + nonce + ciphertext
func Encrypt(data []byte, recipientPublicKey []byte) (*EncryptedData, error) {
	if len(recipientPublicKey) != KeySize {
		return nil, ErrInvalidKeySize
	}

	// Generate ephemeral key pair
	ephemeralKeyPair, err := GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate ephemeral key pair: %w", err)
	}

	// Perform ECDH to derive shared secret
	sharedSecret, err := curve25519.X25519(ephemeralKeyPair.PrivateKey, recipientPublicKey)
	if err != nil {
		return nil, fmt.Errorf("ECDH failed: %w", err)
	}

	// Create ChaCha20-Poly1305 cipher
	cipher, err := chacha20poly1305.New(sharedSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt the data
	ciphertext := cipher.Seal(nil, nonce, data, nil) // #nosec G407 -- nonce is generated via crypto/rand above, not hardcoded

	return &EncryptedData{
		EphemeralPublicKey: ephemeralKeyPair.PublicKey,
		Nonce:              nonce,
		Ciphertext:         ciphertext,
	}, nil
}

// Decrypt decrypts data using the recipient's private key
func Decrypt(encryptedData *EncryptedData, recipientPrivateKey []byte) ([]byte, error) {
	if len(recipientPrivateKey) != KeySize {
		return nil, ErrInvalidKeySize
	}

	if len(encryptedData.EphemeralPublicKey) != KeySize {
		return nil, ErrInvalidKeySize
	}

	if len(encryptedData.Nonce) != NonceSize {
		return nil, ErrInvalidNonceSize
	}

	// Perform ECDH to derive shared secret
	sharedSecret, err := curve25519.X25519(recipientPrivateKey, encryptedData.EphemeralPublicKey)
	if err != nil {
		return nil, fmt.Errorf("ECDH failed: %w", err)
	}

	// Create ChaCha20-Poly1305 cipher
	cipher, err := chacha20poly1305.New(sharedSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Decrypt the data
	plaintext, err := cipher.Open(nil, encryptedData.Nonce, encryptedData.Ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	return plaintext, nil
}

// SecureRandom generates cryptographically secure random bytes
func SecureRandom(size int) ([]byte, error) {
	bytes := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		return nil, fmt.Errorf("failed to generate secure random bytes: %w", err)
	}
	return bytes, nil
}

// FormatPublicKey formats a public key for storage/display
func FormatPublicKey(publicKey []byte) string {
	return fmt.Sprintf("%s%x", PublicKeyPrefix, publicKey)
}

// ParsePublicKey parses a formatted public key
func ParsePublicKey(formatted string) ([]byte, error) {
	if len(formatted) != len(PublicKeyPrefix)+KeySize*2 {
		return nil, ErrInvalidPublicKey
	}

	if formatted[:len(PublicKeyPrefix)] != PublicKeyPrefix {
		return nil, ErrInvalidPublicKey
	}

	publicKey := make([]byte, KeySize)
	n, err := fmt.Sscanf(formatted[len(PublicKeyPrefix):], "%x", &publicKey)
	if err != nil || n != 1 {
		return nil, ErrInvalidPublicKey
	}

	return publicKey, nil
}
