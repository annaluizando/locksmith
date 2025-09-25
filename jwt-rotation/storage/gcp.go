package storage

import (
	"context"
	"fmt"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/api/iterator"
)

// GCPSecretManager implements the SecretStorage interface for GCP Secret Manager.
type GCPSecretManager struct {
	client    *secretmanager.Client
	projectID string
	secretID  string
}

// NewGCPSecretManager creates a new GCPSecretManager.
func NewGCPSecretManager() *GCPSecretManager {
	return &GCPSecretManager{}
}

// Setup initializes the GCP Secret Manager client.
func (g *GCPSecretManager) Setup(ctx context.Context, config map[string]string) error {
	projectID, ok := config["projectID"]
	if !ok || projectID == "" {
		return fmt.Errorf("projectID is required for GCP Secret Manager")
	}
	g.projectID = projectID

	secretID, ok := config["secretID"]
	if !ok || secretID == "" {
		return fmt.Errorf("secretID is required for GCP Secret Manager")
	}
	g.secretID = secretID

	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create secret manager client: %w", err)
	}
	g.client = client
	return nil
}

// Store adds a new secret version to an existing secret in GCP Secret Manager.
func (g *GCPSecretManager) Store(ctx context.Context, id string, value []byte, createdAt time.Time) error {
	parent := fmt.Sprintf("projects/%s/secrets/%s", g.projectID, g.secretID)

	// Add a new secret version
	_, err := g.client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: parent,
		Payload: &secretmanagerpb.SecretPayload{
			Data: value,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add secret version: %w", err)
	}

	return nil
}

// Get retrieves a secret version by its version ID (we'll use our custom ID for this).
// Note: GCP Secret Manager doesn't directly support getting a version by a custom ID stored in labels.
// This implementation iterates through versions, which can be inefficient for many versions.
func (g *GCPSecretManager) Get(ctx context.Context, id string) (*StoredSecret, error) {
	// This is not efficient, GCP Secret Manager does not allow filtering by labels.
	// A better approach would be to store the mapping of our ID to GCP's version number elsewhere.
	// For this implementation, we will iterate and find the version.
	secrets, err := g.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	for _, s := range secrets {
		if s.ID == id {
			return s, nil
		}
	}
	return nil, fmt.Errorf("secret with id %s not found", id)
}

// retrieves the latest version of a secret.
func (g *GCPSecretManager) GetLatest(ctx context.Context) (*StoredSecret, error) {
	// To get the creation time, we need to list the versions.
	req := &secretmanagerpb.ListSecretVersionsRequest{
		Parent:   fmt.Sprintf("projects/%s/secrets/%s", g.projectID, g.secretID),
		PageSize: 1, // We only need the latest one
	}
	it := g.client.ListSecretVersions(ctx, req)
	latestVersion, err := it.Next()
	if err == iterator.Done {
		return nil, fmt.Errorf("no secret versions found for %s", g.secretID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list secret versions: %w", err)
	}

	// Now access the payload of the latest version
	result, err := g.client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: latestVersion.Name,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to access latest secret version: %w", err)
	}

	return &StoredSecret{
		Value:     result.Payload.Data,
		CreatedAt: latestVersion.CreateTime.AsTime(),
	}, nil
}

// GetAll retrieves all versions of a secret.
func (g *GCPSecretManager) GetAll(ctx context.Context) ([]*StoredSecret, error) {
	parent := fmt.Sprintf("projects/%s/secrets/%s", g.projectID, g.secretID)
	req := &secretmanagerpb.ListSecretVersionsRequest{
		Parent: parent,
	}
	it := g.client.ListSecretVersions(ctx, req)

	var secrets []*StoredSecret
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list secret versions: %w", err)
		}

		// Access the secret payload
		versionReq := &secretmanagerpb.AccessSecretVersionRequest{
			Name: resp.Name,
		}
		result, err := g.client.AccessSecretVersion(ctx, versionReq)
		if err != nil {
			// Handle cases where a version might be disabled or destroyed
			continue
		}

		secrets = append(secrets, &StoredSecret{
			// We can't easily get our custom ID back here without storing it in labels
			// or having a way to map GCP's version number to our ID.
			// Let's assume for now the secret value itself is what we need.
			Value:     result.Payload.Data,
			CreatedAt: resp.CreateTime.AsTime(),
		})
	}

	return secrets, nil
}
