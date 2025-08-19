package encryption

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSerializeDeserialize(t *testing.T) {
	recipientKeyPair, err := GenerateKeyPair()
	require.NoError(t, err)

	testData := []byte("Test message for serialization")

	// Encrypt the data
	encrypted, err := Encrypt(testData, recipientKeyPair.PublicKey)
	require.NoError(t, err)

	// Serialize
	serialized, err := encrypted.Serialize()
	require.NoError(t, err)
	assert.NotEmpty(t, serialized)

	// Check that it's valid JSON
	var jsonCheck map[string]interface{}
	err = json.Unmarshal([]byte(serialized), &jsonCheck)
	require.NoError(t, err)

	// Deserialize
	deserialized, err := DeserializeEncryptedData(serialized)
	require.NoError(t, err)

	// Compare fields
	assert.Equal(t, encrypted.EphemeralPublicKey, deserialized.EphemeralPublicKey)
	assert.Equal(t, encrypted.Nonce, deserialized.Nonce)
	assert.Equal(t, encrypted.Ciphertext, deserialized.Ciphertext)

	// Decrypt and verify
	decrypted, err := Decrypt(deserialized, recipientKeyPair.PrivateKey)
	require.NoError(t, err)
	assert.Equal(t, testData, decrypted)
}

func TestSerializedDataStructure(t *testing.T) {
	recipientKeyPair, err := GenerateKeyPair()
	require.NoError(t, err)

	testData := []byte("Test data")
	encrypted, err := Encrypt(testData, recipientKeyPair.PublicKey)
	require.NoError(t, err)

	serialized, err := encrypted.Serialize()
	require.NoError(t, err)

	// Parse the JSON to check structure
	var parsed SerializedEncryptedData
	err = json.Unmarshal([]byte(serialized), &parsed)
	require.NoError(t, err)

	// Check version
	assert.Equal(t, CurrentVersion, parsed.Version)

	// Check all fields are present and not empty
	assert.NotEmpty(t, parsed.EphemeralPublicKey)
	assert.NotEmpty(t, parsed.Nonce)
	assert.NotEmpty(t, parsed.Ciphertext)
}

func TestEncryptAndSerialize(t *testing.T) {
	recipientKeyPair, err := GenerateKeyPair()
	require.NoError(t, err)

	testData := []byte("Test message for convenience function")

	// Use convenience function
	serialized, err := EncryptAndSerialize(testData, recipientKeyPair.PublicKey)
	require.NoError(t, err)
	assert.NotEmpty(t, serialized)

	// Deserialize and decrypt
	decrypted, err := DeserializeAndDecrypt(serialized, recipientKeyPair.PrivateKey)
	require.NoError(t, err)
	assert.Equal(t, testData, decrypted)
}

func TestDeserializeAndDecrypt(t *testing.T) {
	recipientKeyPair, err := GenerateKeyPair()
	require.NoError(t, err)

	testData := []byte("Another test message")

	// Encrypt and serialize manually
	encrypted, err := Encrypt(testData, recipientKeyPair.PublicKey)
	require.NoError(t, err)

	serialized, err := encrypted.Serialize()
	require.NoError(t, err)

	// Use convenience function to deserialize and decrypt
	decrypted, err := DeserializeAndDecrypt(serialized, recipientKeyPair.PrivateKey)
	require.NoError(t, err)
	assert.Equal(t, testData, decrypted)
}

func TestDeserializeInvalidJSON(t *testing.T) {
	invalidJSON := `{"invalid": "json", "missing": "fields"}`

	_, err := DeserializeEncryptedData(invalidJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported serialization version")
}

func TestDeserializeInvalidBase64(t *testing.T) {
	// Create JSON with invalid base64
	invalidData := SerializedEncryptedData{
		Version:            CurrentVersion,
		EphemeralPublicKey: "invalid-base64!",
		Nonce:              "dGVzdA==", // valid base64
		Ciphertext:         "dGVzdA==", // valid base64
	}

	jsonData, err := json.Marshal(invalidData)
	require.NoError(t, err)

	_, err = DeserializeEncryptedData(string(jsonData))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode ephemeral public key")
}

func TestDeserializeUnsupportedVersion(t *testing.T) {
	unsupportedData := SerializedEncryptedData{
		Version:            999, // unsupported version
		EphemeralPublicKey: "dGVzdA==",
		Nonce:              "dGVzdA==",
		Ciphertext:         "dGVzdA==",
	}

	jsonData, err := json.Marshal(unsupportedData)
	require.NoError(t, err)

	_, err = DeserializeEncryptedData(string(jsonData))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported serialization version")
}

func TestRoundTripWithEmptyData(t *testing.T) {
	recipientKeyPair, err := GenerateKeyPair()
	require.NoError(t, err)

	testData := []byte("")

	// Full round trip
	serialized, err := EncryptAndSerialize(testData, recipientKeyPair.PublicKey)
	require.NoError(t, err)

	decrypted, err := DeserializeAndDecrypt(serialized, recipientKeyPair.PrivateKey)
	require.NoError(t, err)
	assert.Len(t, decrypted, 0) // Check length instead of exact equality for empty data
}

func TestRoundTripWithLargeData(t *testing.T) {
	recipientKeyPair, err := GenerateKeyPair()
	require.NoError(t, err)

	// Create 1MB of test data
	testData := make([]byte, 1024*1024)
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	// Full round trip
	serialized, err := EncryptAndSerialize(testData, recipientKeyPair.PublicKey)
	require.NoError(t, err)

	decrypted, err := DeserializeAndDecrypt(serialized, recipientKeyPair.PrivateKey)
	require.NoError(t, err)
	assert.Equal(t, testData, decrypted)
}