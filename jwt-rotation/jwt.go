package secrets

import (
	"encoding/hex"
	"errors"
	"fmt"

	"token-toolkit/jwt-rotation/storage"

	"github.com/golang-jwt/jwt"
)

// handles JWT-specific operations on top of a generic secret rotator.
type JWTManager struct {
	*RotationManager
}

// creates a new manager for JWT secrets.
func NewJWTManager(policy RotationPolicy, secretSizeBytes int, store storage.SecretStorage, notifier Notifier) (*JWTManager, error) {
	generator, err := NewRandomSecretGenerator(secretSizeBytes)
	if err != nil {
		return nil, fmt.Errorf("could not create secret generator: %w", err)
	}

	rotator, err := NewRotationManager(policy, store, generator, notifier)
	if err != nil {
		return nil, fmt.Errorf("could not create rotation manager: %w", err)
	}

	return &JWTManager{RotationManager: rotator}, nil
}

// signs a set of claims with the active secret.
func (jm *JWTManager) SignToken(claims jwt.Claims) (string, error) {
	jm.mutex.RLock()
	activeSecret := jm.activeSecret
	jm.mutex.RUnlock()

	if activeSecret == nil {
		return "", errors.New("no active secret available to sign token")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = activeSecret.ID

	return token.SignedString(activeSecret.Value)
}

// ValidateToken parses and validates a JWT token string.
// It will try the active secret first, then any previous secrets within their grace period.
func (jm *JWTManager) ValidateToken(tokenString string) (*jwt.Token, error) {
	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		jm.mutex.RLock()
		defer jm.mutex.RUnlock()

		kid, ok := token.Header["kid"].(string)
		if ok {
			// Find the secret by key ID
			for _, secret := range jm.GetSecrets() {
				if secret.ID == kid {
					return secret.Value, nil
				}
			}
		}

		return nil, fmt.Errorf("token validation failed: secret with kid '%s' not found", kid)
	})
}

// returns the active secret value as a hex-encoded string.
func (jm *JWTManager) ExportActiveSecretHex() string {
	jm.mutex.RLock()
	defer jm.mutex.RUnlock()

	if jm.activeSecret == nil {
		return ""
	}

	return hex.EncodeToString(jm.activeSecret.Value)
}
