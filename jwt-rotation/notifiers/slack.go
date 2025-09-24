package notifiers

import (
	"fmt"
	"os"

	secrets "token-toolkit/jwt-rotation"

	"github.com/slack-go/slack"
)

type SlackNotifier struct {
	client    *slack.Client
	channelID string
}

func NewSlackNotifier() (*SlackNotifier, error) {
	token := os.Getenv("SLACK_BOT_TOKEN")
	channelID := os.Getenv("SLACK_CHANNEL_ID")

	if token == "" || channelID == "" {
		return nil, nil // Not an error, just means Slack is not configured
	}

	client := slack.New(token)
	return &SlackNotifier{
		client:    client,
		channelID: channelID,
	}, nil
}

// sends a notification about a successful secret rotation.
func (s *SlackNotifier) NotifyRotation(secret *secrets.Secret) {
	if s.client == nil {
		return
	}

	attachment := slack.Attachment{
		Pretext: "Secret Rotation Success",
		Color:   "#36a64f", // green
		Title:   "JWT Secret Rotated Successfully",
		Fields: []slack.AttachmentField{
			{
				Title: "New Secret ID",
				Value: fmt.Sprintf("`%s`", secret.ID),
				Short: false,
			},
		},
	}

	_, _, err := s.client.PostMessage(
		s.channelID,
		slack.MsgOptionAttachments(attachment),
		slack.MsgOptionAsUser(true), // Or false depending on how you want the message to appear
	)

	if err != nil {
		fmt.Printf("Error sending Slack notification: %v\n", err)
	}
}

// sends a notification about an error during secret rotation.
func (s *SlackNotifier) NotifyError(err error) {
	if s.client == nil {
		return
	}

	attachment := slack.Attachment{
		Pretext: "Secret Rotation Failure",
		Color:   "#d9534f", // red
		Title:   "Error During Secret Rotation",
		Text:    fmt.Sprintf("```%v```", err),
	}

	_, _, postErr := s.client.PostMessage(
		s.channelID,
		slack.MsgOptionAttachments(attachment),
		slack.MsgOptionAsUser(true),
	)

	if postErr != nil {
		fmt.Printf("Error sending Slack error notification: %v\n", postErr)
	}
}
