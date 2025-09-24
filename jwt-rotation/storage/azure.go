package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/keyvault/azsecrets"
)

// AzureKeyVault implements the SecretStorage interface for Azure Key Vault.
type AzureKeyVault struct {
	client     *azsecrets.Client
	vaultURI   string
	secretName string
}

// NewAzureKeyVault creates a new AzureKeyVault.
func NewAzureKeyVault() *AzureKeyVault {
	return &AzureKeyVault{}
}

// Setup initializes the Azure Key Vault client.
func (a *AzureKeyVault) Setup(ctx context.Context, config map[string]string) error {
	vaultURI, ok := config["vaulturi"]
	if !ok || vaultURI == "" {
		return fmt.Errorf("vaultURI is required for Azure Key Vault")
	}
	a.vaultURI = vaultURI

	secretName, ok := config["secretname"]
	if !ok || secretName == "" {
		return fmt.Errorf("secretName is required for Azure Key Vault")
	}
	a.secretName = secretName

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return fmt.Errorf("failed to obtain a credential: %w", err)
	}

	client, err := azsecrets.NewClient(a.vaultURI, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create a client: %w", err)
	}
	a.client = client
	return nil
}

// Store creates a new version of a secret in Azure Key Vault.
func (a *AzureKeyVault) Store(ctx context.Context, id string, value []byte, createdAt time.Time) error {
	secretValue := string(value)
	params := azsecrets.SetSecretParameters{
		Value: &secretValue,
	}
	_, err := a.client.SetSecret(ctx, a.secretName, params, nil)
	return err
}

// Get is not implemented for Azure, as we mainly care about storing and latest.
func (a *AzureKeyVault) Get(ctx context.Context, id string) (*StoredSecret, error) {
	return a.GetLatest(ctx)
}

// GetLatest retrieves the latest version of a secret from Azure Key Vault.
func (a *AzureKeyVault) GetLatest(ctx context.Context) (*StoredSecret, error) {
	resp, err := a.client.GetSecret(ctx, a.secretName, "", nil)
	if err != nil {
		return nil, err
	}
	return &StoredSecret{
		Value: []byte(*resp.Value),
	}, nil
}

// GetAll is not implemented for Azure.
func (a *AzureKeyVault) GetAll(ctx context.Context) ([]*StoredSecret, error) {
	latest, err := a.GetLatest(ctx)
	if err != nil {
		return nil, err
	}
	return []*StoredSecret{latest}, nil
}
