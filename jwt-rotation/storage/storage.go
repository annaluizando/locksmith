package storage

import (
	"context"
	"time"
)

// represents a secret stored in the backend.
type StoredSecret struct {
	ID        string
	Value     []byte
	CreatedAt time.Time
}

// defines the interface for storing and retrieving secrets.
type SecretStorage interface {
	// configures the storage provider.
	Setup(ctx context.Context, config map[string]string) error
	// stores a new secret.
	Store(ctx context.Context, id string, value []byte, createdAt time.Time) error
	// retrieves a secret by its ID.
	Get(ctx context.Context, id string) (*StoredSecret, error)
	// retrieves the most recently stored secret.
	GetLatest(ctx context.Context) (*StoredSecret, error)
	// retrieves all secrets for token validation.
	GetAll(ctx context.Context) ([]*StoredSecret, error)
}
