package secrets

import (
	"context"
	"fmt"
	"sync"
	"time"

	"token-toolkit/jwt-rotation/storage"
)

// RotationManager provides a generic mechanism for rotating secrets.
type RotationManager struct {
	activeSecret    *Secret
	previousSecrets []*Secret
	policy          RotationPolicy
	mutex           sync.RWMutex
	autoRotate      bool
	notifier        Notifier
	storage         storage.SecretStorage
	generator       SecretGenerator
}

// NewRotationManager creates a new RotationManager.
func NewRotationManager(policy RotationPolicy, store storage.SecretStorage, gen SecretGenerator, notifier Notifier) (*RotationManager, error) {
	rm := &RotationManager{
		policy:          policy,
		previousSecrets: make([]*Secret, 0),
		storage:         store,
		generator:       gen,
		notifier:        notifier,
	}

	// Try to load secrets from storage
	allStoredSecrets, err := store.GetAll(context.Background())
	if err == nil && len(allStoredSecrets) > 0 {
		// Found secrets in storage, reconstruct state
		for _, s := range allStoredSecrets {
			secret := &Secret{
				ID:        s.ID,
				Value:     s.Value,
				CreatedAt: s.CreatedAt,
				Active:    false, // Mark all as inactive initially
			}
			// This logic assumes the latest secret is the first one.
			// A more robust implementation might sort by CreatedAt.
			if rm.activeSecret == nil {
				secret.Active = true
				rm.activeSecret = secret
			} else {
				rm.previousSecrets = append(rm.previousSecrets, secret)
			}
		}
	} else {
		// If no secrets in storage
		secret, err := rm.generateAndStoreSecret()
		if err != nil {
			if rm.notifier != nil {
				rm.notifier.NotifyError(err)
			}
			return nil, fmt.Errorf("failed to generate initial secret: %w", err)
		}
		rm.activeSecret = secret
	}

	return rm, nil
}

// generateAndStoreSecret creates a new secret using the generator and stores it.
func (rm *RotationManager) generateAndStoreSecret() (*Secret, error) {
	value, err := rm.generator.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate secret value: %w", err)
	}

	secret := &Secret{
		ID:        generateSecretId(value),
		Value:     value,
		CreatedAt: time.Now(),
		Active:    true,
	}

	if err := rm.storage.Store(context.Background(), secret.ID, secret.Value, secret.CreatedAt); err != nil {
		return nil, fmt.Errorf("failed to store new secret: %w", err)
	}
	return secret, nil
}

// RotateSecret performs a manual secret rotation.
func (rm *RotationManager) RotateSecret() (*Secret, error) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	newSecret, err := rm.generateAndStoreSecret()
	if err != nil {
		if rm.notifier != nil {
			rm.notifier.NotifyError(err)
		}
		return nil, err
	}

	if rm.activeSecret != nil {
		rm.activeSecret.Active = false // current secret goes inactive
		rm.previousSecrets = append([]*Secret{rm.activeSecret}, rm.previousSecrets...)
		rm.cleanupOldSecrets()
	}

	rm.activeSecret = newSecret

	if rm.notifier != nil {
		go rm.notifier.NotifyRotation(newSecret)
	}

	return newSecret, nil
}

// cleanupOldSecrets removes secrets that are past their grace period.
func (rm *RotationManager) cleanupOldSecrets() {
	if rm.policy.GracePeriod <= 0 {
		return
	}

	cutOffTime := time.Now().Add(-rm.policy.GracePeriod)
	validSecrets := make([]*Secret, 0, len(rm.previousSecrets))

	for _, secret := range rm.previousSecrets {
		if secret.CreatedAt.After(cutOffTime) {
			validSecrets = append(validSecrets, secret)
		}
	}

	rm.previousSecrets = validSecrets
}

// StartAutoRotation starts a background goroutine to rotate secrets periodically.
func (rm *RotationManager) StartAutoRotation() error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if rm.policy.RotationInterval <= 0 {
		return fmt.Errorf("rotation interval must be greater than zero")
	}

	rm.autoRotate = true

	go func() {
		ticker := time.NewTicker(rm.policy.RotationInterval)
		defer ticker.Stop()

		for range ticker.C {
			rm.mutex.RLock()
			shouldRotate := rm.autoRotate
			rm.mutex.RUnlock()

			if !shouldRotate {
				return
			}

			if _, err := rm.RotateSecret(); err != nil {
				if rm.notifier != nil {
					rm.notifier.NotifyError(err)
				}
				// It's better to log this than to panic
				fmt.Printf("Error during automatic rotation: %v\n", err)
			}
		}
	}()

	return nil
}

func (rm *RotationManager) StopAutoRotation() {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	rm.autoRotate = false
}

// returns all the secrets currently managed by the rotator.
func (rm *RotationManager) GetSecrets() []*Secret {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	secrets := make([]*Secret, 0, len(rm.previousSecrets)+1)
	if rm.activeSecret != nil {
		secrets = append(secrets, rm.activeSecret)
	}
	secrets = append(secrets, rm.previousSecrets...)

	return secrets
}
