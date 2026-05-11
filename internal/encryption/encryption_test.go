package encryption

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateKeyPair(t *testing.T) {
	keyPair, err := GenerateKeyPair()
	require.NoError(t, err)
	require.NotNil(t, keyPair)

	// Check key sizes
	assert.Len(t, keyPair.PublicKey, KeySize)
	assert.Len(t, keyPair.PrivateKey, KeySize)

	// Keys should not be all zeros
	assert.NotEqual(t, make([]byte, KeySize), keyPair.PublicKey)
	assert.NotEqual(t, make([]byte, KeySize), keyPair.PrivateKey)

	// Generate another key pair and ensure they're different
	keyPair2, err := GenerateKeyPair()
	require.NoError(t, err)
	assert.NotEqual(t, keyPair.PublicKey, keyPair2.PublicKey)
	assert.NotEqual(t, keyPair.PrivateKey, keyPair2.PrivateKey)
}

func TestEncryptDecrypt(t *testing.T) {
	// Generate recipient key pair
	recipientKeyPair, err := GenerateKeyPair()
	require.NoError(t, err)

	testData := []byte("Hello, MailVault! This is a test message for encryption.")

	// Encrypt the data
	encrypted, err := Encrypt(testData, recipientKeyPair.PublicKey)
	require.NoError(t, err)
	require.NotNil(t, encrypted)

	// Check encrypted data structure
	assert.Len(t, encrypted.EphemeralPublicKey, KeySize)
	assert.Len(t, encrypted.Nonce, NonceSize)
	assert.Greater(t, len(encrypted.Ciphertext), len(testData)) // Should be larger due to auth tag

	// Decrypt the data
	decrypted, err := Decrypt(encrypted, recipientKeyPair.PrivateKey)
	require.NoError(t, err)
	assert.Equal(t, testData, decrypted)
}

func TestEncryptDecryptLargeData(t *testing.T) {
	// Test with larger data (10KB)
	testData := make([]byte, 10*1024)
	_, err := rand.Read(testData)
	require.NoError(t, err)

	recipientKeyPair, err := GenerateKeyPair()
	require.NoError(t, err)

	encrypted, err := Encrypt(testData, recipientKeyPair.PublicKey)
	require.NoError(t, err)

	decrypted, err := Decrypt(encrypted, recipientKeyPair.PrivateKey)
	require.NoError(t, err)
	assert.Equal(t, testData, decrypted)
}

func TestEncryptDecryptEmptyData(t *testing.T) {
	recipientKeyPair, err := GenerateKeyPair()
	require.NoError(t, err)

	testData := []byte{}

	encrypted, err := Encrypt(testData, recipientKeyPair.PublicKey)
	require.NoError(t, err)

	decrypted, err := Decrypt(encrypted, recipientKeyPair.PrivateKey)
	require.NoError(t, err)
	assert.Len(t, decrypted, 0) // Check length instead of exact equality for empty data
}

func TestDecryptWithWrongKey(t *testing.T) {
	// Generate two different key pairs
	keyPair1, err := GenerateKeyPair()
	require.NoError(t, err)

	keyPair2, err := GenerateKeyPair()
	require.NoError(t, err)

	testData := []byte("Secret message")

	// Encrypt with keyPair1's public key
	encrypted, err := Encrypt(testData, keyPair1.PublicKey)
	require.NoError(t, err)

	// Try to decrypt with keyPair2's private key (should fail)
	_, err = Decrypt(encrypted, keyPair2.PrivateKey)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decryption failed")
}

func TestEncryptWithInvalidPublicKey(t *testing.T) {
	testData := []byte("test data")

	// Test with wrong key size
	invalidKey := make([]byte, KeySize-1)
	_, err := Encrypt(testData, invalidKey)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidKeySize, err)

	// Test with nil key
	_, err = Encrypt(testData, nil)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidKeySize, err)
}

func TestDecryptWithInvalidPrivateKey(t *testing.T) {
	recipientKeyPair, err := GenerateKeyPair()
	require.NoError(t, err)

	testData := []byte("test data")
	encrypted, err := Encrypt(testData, recipientKeyPair.PublicKey)
	require.NoError(t, err)

	// Test with wrong key size
	invalidKey := make([]byte, KeySize-1)
	_, err = Decrypt(encrypted, invalidKey)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidKeySize, err)
}

func TestDecryptWithInvalidEncryptedData(t *testing.T) {
	recipientKeyPair, err := GenerateKeyPair()
	require.NoError(t, err)

	// Test with invalid ephemeral public key size
	invalidEncrypted := &EncryptedData{
		EphemeralPublicKey: make([]byte, KeySize-1),
		Nonce:              make([]byte, NonceSize),
		Ciphertext:         []byte("test"),
	}
	_, err = Decrypt(invalidEncrypted, recipientKeyPair.PrivateKey)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidKeySize, err)

	// Test with invalid nonce size
	invalidEncrypted = &EncryptedData{
		EphemeralPublicKey: make([]byte, KeySize),
		Nonce:              make([]byte, NonceSize-1),
		Ciphertext:         []byte("test"),
	}
	_, err = Decrypt(invalidEncrypted, recipientKeyPair.PrivateKey)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidNonceSize, err)
}

func TestSecureRandom(t *testing.T) {
	size := 32
	random1, err := SecureRandom(size)
	require.NoError(t, err)
	assert.Len(t, random1, size)

	random2, err := SecureRandom(size)
	require.NoError(t, err)
	assert.Len(t, random2, size)

	// Should be different
	assert.NotEqual(t, random1, random2)

	// Should not be all zeros
	assert.NotEqual(t, make([]byte, size), random1)
	assert.NotEqual(t, make([]byte, size), random2)
}

func TestFormatParsePublicKey(t *testing.T) {
	keyPair, err := GenerateKeyPair()
	require.NoError(t, err)

	// Format public key
	formatted := FormatPublicKey(keyPair.PublicKey)
	assert.Contains(t, formatted, PublicKeyPrefix)
	assert.Len(t, formatted, len(PublicKeyPrefix)+KeySize*2)

	// Parse it back
	parsed, err := ParsePublicKey(formatted)
	require.NoError(t, err)
	assert.Equal(t, keyPair.PublicKey, parsed)
}

func TestParsePublicKeyInvalid(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
	}{
		{
			name:      "wrong prefix",
			input:     "rsa:" + "a1b2c3d4e5f6",
			expectErr: true,
		},
		{
			name:      "wrong length",
			input:     PublicKeyPrefix + "a1b2c3",
			expectErr: true,
		},
		{
			name:      "invalid hex",
			input:     PublicKeyPrefix + "invalid_hex_string_that_is_wrong_length_exactly_64_chars",
			expectErr: true,
		},
		{
			name:      "empty",
			input:     "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParsePublicKey(tt.input)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEncryptionDeterminism(t *testing.T) {
	// Encryption should not be deterministic (different each time)
	recipientKeyPair, err := GenerateKeyPair()
	require.NoError(t, err)

	testData := []byte("Same message")

	encrypted1, err := Encrypt(testData, recipientKeyPair.PublicKey)
	require.NoError(t, err)

	encrypted2, err := Encrypt(testData, recipientKeyPair.PublicKey)
	require.NoError(t, err)

	// Should have different ephemeral keys and nonces
	assert.NotEqual(t, encrypted1.EphemeralPublicKey, encrypted2.EphemeralPublicKey)
	assert.NotEqual(t, encrypted1.Nonce, encrypted2.Nonce)
	assert.NotEqual(t, encrypted1.Ciphertext, encrypted2.Ciphertext)

	// But both should decrypt to the same message
	decrypted1, err := Decrypt(encrypted1, recipientKeyPair.PrivateKey)
	require.NoError(t, err)

	decrypted2, err := Decrypt(encrypted2, recipientKeyPair.PrivateKey)
	require.NoError(t, err)

	assert.Equal(t, testData, decrypted1)
	assert.Equal(t, testData, decrypted2)
}
