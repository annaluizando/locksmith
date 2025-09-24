// This is a placeholder for the Azure Function implementation.
// The structure of an Azure Function in Go involves a function.json
// and a go file with the function logic.

// Example function.json:
// {
//  "scriptFile": "main.go",
//  "bindings": [
//    {
//     "name": "myTimer",
//     "type": "timerTrigger",
//     "direction": "in",
//      "schedule": "0 */5 * * * *"
//    }
//  ]
// }

package main

import (
	"context"
	"log"
	"os"
	"time"

	secrets "token-toolkit/jwt-rotation"
	"token-toolkit/jwt-rotation/notifiers"
	"token-toolkit/jwt-rotation/storage"
)

// Run is the entry point for the Azure Function.
func Run(ctx context.Context, myTimer interface{}) {
	config := map[string]string{
		"vaultURI":   os.Getenv("VAULT_URI"),
		"secretName": os.Getenv("SECRET_NAME"),
	}

	storageProvider := storage.NewAzureKeyVault()
	if err := storageProvider.Setup(ctx, config); err != nil {
		log.Printf("Error setting up storage: %v", err)
		return
	}

	policy := secrets.RotationPolicy{
		RotationInterval: 0, // Not needed, triggered by schedule
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
		return
	}

	if _, err := secretManager.RotateSecret(); err != nil {
		log.Printf("Failed to rotate secret: %v", err)
		return
	}

	log.Println("Secret rotated successfully!")
}

func main() {
	// The main function is not used by Azure Functions,
	// but it's good practice to have it.
}
