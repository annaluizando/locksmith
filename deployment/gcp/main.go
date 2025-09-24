package gcp

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	secrets "token-toolkit/jwt-rotation"
	"token-toolkit/jwt-rotation/notifiers"
	"token-toolkit/jwt-rotation/storage"
)

// Google Cloud Function that rotates a JWT secret.
func RotateSecret(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	// Configuration will be passed via environment variables in the Cloud Function
	config := map[string]string{
		"projectID": os.Getenv("PROJECT_ID"),
		"secretID":  os.Getenv("SECRET_ID"),
	}

	storageProvider := storage.NewGCPSecretManager()
	if err := storageProvider.Setup(ctx, config); err != nil {
		log.Printf("Error setting up storage: %v", err)
		http.Error(w, "Error setting up storage", http.StatusInternalServerError)
		return
	}

	policy := secrets.RotationPolicy{
		RotationInterval: 0, // Not needed, triggered by scheduler
		GracePeriod:      48 * time.Hour,
	}

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
		http.Error(w, "Failed to create secret manager", http.StatusInternalServerError)
		return
	}

	if _, err := secretManager.RotateSecret(); err != nil {
		log.Printf("Failed to rotate secret: %v", err)
		http.Error(w, "Failed to rotate secret", http.StatusInternalServerError)
		return
	}

	fmt.Fprintln(w, "Secret rotated successfully!")
}
