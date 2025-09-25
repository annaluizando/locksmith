package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	"token-toolkit/jwt-rotation/storage"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// AWS Lambda handler for the Slack slash command.
func HandleSlackCommand(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Slack sends the command as a form-urlencoded payload. We need to parse it.
	params, err := url.ParseQuery(req.Body)
	if err != nil {
		return events.APIGatewayProxyResponse{Body: "Error parsing request body", StatusCode: 400}, nil
	}

	command := params.Get("command")
	text := params.Get("text")

	// Ensure the command is what we expect.
	if command != "/locksmith" || text != "status" {
		return events.APIGatewayProxyResponse{
			Body:       "Unsupported command. Please use `/locksmith status`",
			StatusCode: 200,
		}, nil
	}

	// Configure the cloud provider and credentials via environment variables.
	provider := os.Getenv("CLOUD_PROVIDER") // "gcp", "aws", or "azure"
	config := map[string]string{
		// GCP
		"projectID": os.Getenv("GCP_PROJECT_ID"),
		"secretID":  os.Getenv("GCP_SECRET_ID"),
		// AWS
		"region": os.Getenv("AWS_REGION"),
		// Azure
		"vaultURI":   os.Getenv("AZURE_VAULT_URI"),
		"secretName": os.Getenv("AZURE_SECRET_NAME"),
	}

	if provider == "aws" {
		config["secretID"] = os.Getenv("AWS_SECRET_ID")
	}

	var storageProvider storage.SecretStorage
	switch provider {
	case "gcp":
		storageProvider = storage.NewGCPSecretManager()
	case "aws":
		storageProvider = storage.NewAWSSecretsManager()
	case "azure":
		storageProvider = storage.NewAzureKeyVault()
	default:
		return events.APIGatewayProxyResponse{Body: "Error: CLOUD_PROVIDER environment variable is not configured correctly.", StatusCode: 500}, nil
	}

	if err := storageProvider.Setup(ctx, config); err != nil {
		return events.APIGatewayProxyResponse{Body: fmt.Sprintf("Error setting up storage provider: %v", err), StatusCode: 500}, nil
	}

	latestSecret, err := storageProvider.GetLatest(ctx)
	if err != nil {
		return events.APIGatewayProxyResponse{Body: fmt.Sprintf("Error getting latest secret: %v", err), StatusCode: 500}, nil
	}

	responseText := fmt.Sprintf("✅ The last secret rotation was at: *%s*", latestSecret.CreatedAt.Format(time.RFC1123))

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       fmt.Sprintf(`{"response_type": "in_channel", "text": "%s"}`, responseText),
	}, nil
}

func main() {
	lambda.Start(HandleSlackCommand)
}
