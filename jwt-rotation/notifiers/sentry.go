package notifiers

import (
	"fmt"
	"log"
	"os"
	"time"

	secrets "token-toolkit/jwt-rotation"

	"github.com/getsentry/sentry-go"
)

// sends notifications to Sentry.
type SentryNotifier struct {
	client *sentry.Client
}

// creates a new SentryNotifier.
func NewSentryNotifier() (*SentryNotifier, error) {
	sentryDSN := os.Getenv("SENTRY_DSN")
	if sentryDSN == "" {
		return nil, nil // Not an error, just means Sentry is not configured
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:         sentryDSN,
		Environment: "production",
		Release:     "token-toolkit@1.0.0",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize sentry: %w", err)
	}

	return &SentryNotifier{client: sentry.CurrentHub().Client()}, nil
}

// sends a notification about a successful secret rotation.
func (s *SentryNotifier) NotifyRotation(secret *secrets.Secret) {
	if s.client == nil {
		return
	}
	sentry.CaptureMessage(fmt.Sprintf("JWT Secret rotated successfully: %s", secret.ID))
	log.Println("Notification sent to Sentry for successful rotation.")
	sentry.Flush(2 * time.Second)
}

// sends a notification about an error during secret rotation.
func (s *SentryNotifier) NotifyError(err error) {
	if s.client == nil {
		return
	}
	sentry.CaptureException(err)
	log.Printf("Error notification sent to Sentry: %v\n", err)
	sentry.Flush(2 * time.Second)
}
