package service

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
)

var (
	slackClient    *slack.Client
	isSlackEnabled bool
)

func init() {
	// Load .env for local/dev usage; ignore error in serverless
	_ = godotenv.Load()

	botToken := os.Getenv("SLACK_BOT_TOKEN")
	if botToken == "" {
		log.Printf("SLACK_BOT_TOKEN environment variable is not set. Slack notifications will be disabled.")
		isSlackEnabled = false
		return
	}
	if !strings.HasPrefix(botToken, "xoxb-") {
		log.Printf("SLACK_BOT_TOKEN appears to be invalid (should start with 'xoxb-'). Slack notifications will be disabled.")
		isSlackEnabled = false
		return
	}

	slackClient = slack.New(botToken)
	if err := testSlackAuth(); err != nil {
		log.Printf("Slack authentication failed. Slack notifications will be disabled: %v", err)
		isSlackEnabled = false
		return
	}
	isSlackEnabled = true
	log.Printf("Slack integration initialized successfully")
}

func testSlackAuth() error {
	if slackClient == nil {
		return fmt.Errorf("slack client not initialized")
	}
	_, err := slackClient.AuthTest()
	return err
}

// SendMessage sends a message to a channel.
func SendMessage(channelID, message string) (string, error) {
	if !isSlackEnabled {
		return "", fmt.Errorf("slack is not properly configured")
	}
	if channelID == "" {
		return "", fmt.Errorf("channel ID is required")
	}

	_, timestamp, err := slackClient.PostMessage(
		channelID,
		slack.MsgOptionText(message, false),
	)
	if err != nil {
		if strings.Contains(err.Error(), "invalid_auth") {
			return "", fmt.Errorf("slack authentication failed - please check your bot token and permissions")
		}
		if strings.Contains(err.Error(), "channel_not_found") {
			return "", fmt.Errorf("slack channel not found - please check your channel ID")
		}
		if strings.Contains(err.Error(), "not_in_channel") {
			return "", fmt.Errorf("slack bot is not in the specified channel - please invite the bot to the channel")
		}
		return "", fmt.Errorf("failed to send slack message: %v", err)
	}
	return timestamp, nil
}
