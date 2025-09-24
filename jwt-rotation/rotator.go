package secrets

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// defines the timing for secret rotation.
type RotationPolicy struct {
	RotationInterval time.Duration `json:"rotationInterval"`
	GracePeriod      time.Duration `json:"gracePeriod"`
}

// represents the raw value of a secret.
type SecretValue []byte

// represents a secret with its metadata.
type Secret struct {
	ID        string      `json:"id"`
	Value     SecretValue `json:"value"`
	CreatedAt time.Time   `json:"createdAt"`
	Active    bool        `json:"active"`
}

// defines the interface for generating new secret values.
type SecretGenerator interface {
	Generate() (SecretValue, error)
}

// generates a random byte slice as a secret.
type RandomSecretGenerator struct {
	secretSizeBytes int
}

// creates a new RandomSecretGenerator.
func NewRandomSecretGenerator(sizeBytes int) (*RandomSecretGenerator, error) {
	if sizeBytes < 32 {
		return nil, errors.New("secret size must be at least 32 bytes")
	}
	return &RandomSecretGenerator{secretSizeBytes: sizeBytes}, nil
}

// Generate creates a new random secret.
func (g *RandomSecretGenerator) Generate() (SecretValue, error) {
	secret := make([]byte, g.secretSizeBytes)
	_, err := rand.Read(secret)
	if err != nil {
		return nil, fmt.Errorf("error generating random secret: %w", err)
	}
	return secret, nil
}

// creates a unique ID for a secret value.
func generateSecretId(secret []byte) string {
	h := hmac.New(sha256.New, []byte(""))
	h.Write(secret)
	return hex.EncodeToString(h.Sum(nil))[:12] // uses first 12 chars for id
}

// Notifier defines the interface for sending notifications about secret rotation events.
type Notifier interface {
	NotifyRotation(secret *Secret)
	NotifyError(err error)
}
