package main

import (
	"context"
	"log"
	"os"
	"time"

	secrets "token-toolkit/jwt-rotation"
	"token-toolkit/jwt-rotation/notifiers"
	"token-toolkit/jwt-rotation/storage"

	"github.com/aws/aws-lambda-go/lambda"
)

func HandleRequest(ctx context.Context) (string, error) {
	// Configuration will be passed via environment variables in Lambda
	config := map[string]string{
		"secretID": os.Getenv("SECRET_ID"),
		"region":   os.Getenv("REGION"),
	}

	storageProvider := storage.NewAWSSecretsManager()
	if err := storageProvider.Setup(ctx, config); err != nil {
		log.Printf("Error setting up storage: %v", err)
		return "Error", err
	}

	policy := secrets.RotationPolicy{
		RotationInterval: 0, // Not needed for Lambda, it's triggered by schedule
		GracePeriod:      48 * time.Hour,
	}

	// In the Lambda, we'll initialize all available notifiers
	// based on the environment variables provided.
	var notifiersList []secrets.Notifier
	sentryNotifier, err := notifiers.NewSentryNotifier()
	if err != nil {
		log.Printf("Could not create sentry notifier: %v", err)
	}
	if sentryNotifier != nil {
		notifiersList = append(notifiersList, sentryNotifier)
	}

	slackNotifier, err := notifiers.NewSlackNotifier()
	if err != nil {
		log.Printf("Could not create slack notifier: %v", err)
	}
	if slackNotifier != nil {
		notifiersList = append(notifiersList, slackNotifier)
	}

	notifier := notifiers.NewMultiNotifier(notifiersList...)

	secretManager, err := secrets.NewJWTManager(policy, 64, storageProvider, notifier)
	if err != nil {
		log.Printf("Failed to create secret manager: %v", err)
		return "Error", err
	}

	if _, err := secretManager.RotateSecret(); err != nil {
		log.Printf("Failed to rotate secret: %v", err)
		return "Error", err
	}

	return "Secret rotated successfully!", nil
}

func main() {
	lambda.Start(HandleRequest)
}
