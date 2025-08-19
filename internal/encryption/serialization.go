package encryption

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// SerializedEncryptedData represents encrypted data in a serializable format
type SerializedEncryptedData struct {
	Version            int    `json:"version"`
	EphemeralPublicKey string `json:"ephemeral_public_key"`
	Nonce              string `json:"nonce"`
	Ciphertext         string `json:"ciphertext"`
}

const (
	// CurrentVersion is the current serialization format version
	CurrentVersion = 1
)

// Serialize converts EncryptedData to a JSON string for storage
func (ed *EncryptedData) Serialize() (string, error) {
	serialized := SerializedEncryptedData{
		Version:            CurrentVersion,
		EphemeralPublicKey: base64.StdEncoding.EncodeToString(ed.EphemeralPublicKey),
		Nonce:              base64.StdEncoding.EncodeToString(ed.Nonce),
		Ciphertext:         base64.StdEncoding.EncodeToString(ed.Ciphertext),
	}

	jsonData, err := json.Marshal(serialized)
	if err != nil {
		return "", fmt.Errorf("failed to serialize encrypted data: %w", err)
	}

	return string(jsonData), nil
}

// DeserializeEncryptedData converts a JSON string back to EncryptedData
func DeserializeEncryptedData(data string) (*EncryptedData, error) {
	var serialized SerializedEncryptedData
	if err := json.Unmarshal([]byte(data), &serialized); err != nil {
		return nil, fmt.Errorf("failed to deserialize encrypted data: %w", err)
	}

	// Check version compatibility
	if serialized.Version != CurrentVersion {
		return nil, fmt.Errorf("unsupported serialization version: %d", serialized.Version)
	}

	ephemeralPublicKey, err := base64.StdEncoding.DecodeString(serialized.EphemeralPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ephemeral public key: %w", err)
	}

	nonce, err := base64.StdEncoding.DecodeString(serialized.Nonce)
	if err != nil {
		return nil, fmt.Errorf("failed to decode nonce: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(serialized.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	return &EncryptedData{
		EphemeralPublicKey: ephemeralPublicKey,
		Nonce:              nonce,
		Ciphertext:         ciphertext,
	}, nil
}

// EncryptAndSerialize is a convenience function that encrypts data and returns serialized result
func EncryptAndSerialize(data []byte, recipientPublicKey []byte) (string, error) {
	encrypted, err := Encrypt(data, recipientPublicKey)
	if err != nil {
		return "", err
	}

	return encrypted.Serialize()
}

// DeserializeAndDecrypt is a convenience function that deserializes and decrypts data
func DeserializeAndDecrypt(serializedData string, recipientPrivateKey []byte) ([]byte, error) {
	encrypted, err := DeserializeEncryptedData(serializedData)
	if err != nil {
		return nil, err
	}

	return Decrypt(encrypted, recipientPrivateKey)
}