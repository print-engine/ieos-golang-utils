package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/print-engine/ieos-golang-utils/logger"
)

// PubSubMessage is the payload of a Pub/Sub event.
// Data is a base64-decoded JSON blob when delivered by Log Router.
// See: https://cloud.google.com/logging/docs/export/pubsub#data_format
type PubSubMessage struct {
	Data       []byte            `json:"data"`
	Attributes map[string]string `json:"attributes"`
}

var (
	appLogger      *logger.CloudLogger
	loggerInitOnce sync.Once
)

// getLogger returns a cached Cloud Logger instance.
func getLogger(ctx context.Context) *logger.CloudLogger {
	loggerInitOnce.Do(func() {
		lg, err := logger.New(ctx,
			logger.WithInvoker("ieos-slack-logs"),
			logger.WithLogName("ieos-slack-logs"),
			logger.WithCommonLabels(map[string]string{"service": "ieos-slack-logs"}),
		)
		if err != nil {
			// Fallback to stdout-only logger so callers can still log
			lg, _ = logger.New(ctx,
				logger.WithStdoutOnly(),
				logger.WithInvoker("ieos-slack-logs"),
				logger.WithLogName("ieos-slack-logs"),
				logger.WithCommonLabels(map[string]string{"service": "ieos-slack-logs"}),
			)
		}
		appLogger = lg
	})
	return appLogger
}

// HandleLogAlert is a Cloud Function / Functions Framework handler for Pub/Sub.
// Exported for deployment. It parses the LogEntry JSON and sends a Slack message.
func HandleLogAlert(ctx context.Context, m PubSubMessage) error {
	reqLog := getLogger(ctx).ForRequest(ctx, nil)

	if len(m.Data) == 0 {
		reqLog.Warning("empty pubsub data")
		return fmt.Errorf("empty pubsub data")
	}

	var payload map[string]any
	if err := json.Unmarshal(m.Data, &payload); err != nil {
		reqLog.Error("failed to parse pubsub json", err)
		return err
	}

	severity := getString(payload["severity"]) // "ERROR", "WARNING", etc.
	logName := getString(payload["logName"])    // projects/..../logs/...
	text := getString(payload["textPayload"])   // optional

	// build a concise message
	var b strings.Builder
	if severity == "" {
		severity = "DEFAULT"
	}
	fmt.Fprintf(&b, "[%s] %s", severity, logName)
	if text != "" {
		fmt.Fprintf(&b, "\n%s", text)
	}
	// If jsonPayload exists, include a compact excerpt
	if jp, ok := payload["jsonPayload"]; ok && jp != nil {
		if compact, err := json.Marshal(jp); err == nil {
			fmt.Fprintf(&b, "\njson: %s", compact)
		}
	}

	message := b.String()

	channelID := chooseChannelForSeverity(severity)
	ts, err := SendMessage(channelID, message)
	if err != nil {
		reqLog.Error("slack send failed", err)
		return err
	}

	reqLog.Info("slack message sent", map[string]any{"ts": ts, "channel": channelID})
	return nil
}

func chooseChannelForSeverity(sev string) string {
	sev = strings.ToUpper(sev)
	switch sev {
	case "CRITICAL", "ALERT", "EMERGENCY", "ERROR":
		if v := os.Getenv("SLACK_ERROR_CHANNEL_ID"); v != "" { return v }
	case "WARNING", "NOTICE":
		if v := os.Getenv("SLACK_WARNING_CHANNEL_ID"); v != "" { return v }
	}
	if v := os.Getenv("SLACK_DEFAULT_CHANNEL_ID"); v != "" { return v }
	// last resort: return empty which will surface as validation error in SendMessage
	return ""
}

func getString(v any) string {
	s, _ := v.(string)
	return s
}
