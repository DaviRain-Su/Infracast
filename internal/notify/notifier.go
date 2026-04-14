// Package notify provides notification services for deployment events
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Notifier sends deployment notifications
type Notifier struct {
	config *Config
	client *http.Client
}

// Config holds notification configuration
type Config struct {
	FeishuWebhook  string
	DingTalkWebhook string
	Enabled        bool
}

// NewNotifier creates a new notifier
func NewNotifier(config *Config) *Notifier {
	if config == nil {
		config = &Config{}
	}
	
	return &Notifier{
		config: config,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// DeploymentEvent represents a deployment event to notify about
type DeploymentEvent struct {
	AppName    string
	Env        string
	Commit     string
	Status     string // "success", "failed", "rollback"
	Duration   time.Duration
	Error      string
	StartedAt  time.Time
	FinishedAt time.Time
}

// Notify sends notifications to all configured channels
func (n *Notifier) Notify(ctx context.Context, event *DeploymentEvent) error {
	// Skip if notifications are disabled
	if !n.config.Enabled {
		return nil
	}

	// Skip if no webhooks configured
	if n.config.FeishuWebhook == "" && n.config.DingTalkWebhook == "" {
		return nil
	}

	var lastErr error

	// Send to Feishu (non-blocking)
	if n.config.FeishuWebhook != "" {
		if err := n.sendFeishu(ctx, event); err != nil {
			// Log but don't fail - notifications are best-effort
			lastErr = fmt.Errorf("feishu notification failed: %w", err)
		}
	}

	// Send to DingTalk (non-blocking)
	if n.config.DingTalkWebhook != "" {
		if err := n.sendDingTalk(ctx, event); err != nil {
			lastErr = fmt.Errorf("dingtalk notification failed: %w", err)
		}
	}

	// Return last error if any, but don't block deployment
	return lastErr
}

// sendFeishu sends notification to Feishu webhook
func (n *Notifier) sendFeishu(ctx context.Context, event *DeploymentEvent) error {
	message := n.formatFeishuMessage(event)
	
	req, err := http.NewRequestWithContext(ctx, "POST", n.config.FeishuWebhook, bytes.NewBuffer(message))
	if err != nil {
		return err
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("feishu webhook returned %d", resp.StatusCode)
	}
	
	return nil
}

// sendDingTalk sends notification to DingTalk webhook
func (n *Notifier) sendDingTalk(ctx context.Context, event *DeploymentEvent) error {
	message := n.formatDingTalkMessage(event)
	
	req, err := http.NewRequestWithContext(ctx, "POST", n.config.DingTalkWebhook, bytes.NewBuffer(message))
	if err != nil {
		return err
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dingtalk webhook returned %d", resp.StatusCode)
	}
	
	return nil
}

// formatFeishuMessage formats message for Feishu
func (n *Notifier) formatFeishuMessage(event *DeploymentEvent) []byte {
	statusEmoji := "✅"
	if event.Status == "failed" {
		statusEmoji = "❌"
	} else if event.Status == "rollback" {
		statusEmoji = "🔄"
	}

	content := fmt.Sprintf("%s **Deployment %s**\n\n"+
		"**App:** %s\n"+
		"**Env:** %s\n"+
		"**Commit:** %s\n"+
		"**Duration:** %v",
		statusEmoji, event.Status,
		event.AppName,
		event.Env,
		event.Commit,
		event.Duration,
	)

	if event.Error != "" {
		content += fmt.Sprintf("\n**Error:** %s", event.Error)
	}

	msg := map[string]interface{}{
		"msg_type": "text",
		"content": map[string]string{
			"text": content,
		},
	}

	data, _ := json.Marshal(msg)
	return data
}

// formatDingTalkMessage formats message for DingTalk
func (n *Notifier) formatDingTalkMessage(event *DeploymentEvent) []byte {
	statusEmoji := "✅"
	if event.Status == "failed" {
		statusEmoji = "❌"
	} else if event.Status == "rollback" {
		statusEmoji = "🔄"
	}

	content := fmt.Sprintf("%s **Deployment %s**\n\n"+
		"**App:** %s\n"+
		"**Env:** %s\n"+
		"**Commit:** %s\n"+
		"**Duration:** %v",
		statusEmoji, event.Status,
		event.AppName,
		event.Env,
		event.Commit,
		event.Duration,
	)

	if event.Error != "" {
		content += fmt.Sprintf("\n**Error:** %s", event.Error)
	}

	msg := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": content,
		},
	}

	data, _ := json.Marshal(msg)
	return data
}

// IsEnabled checks if notifications are enabled
func (n *Notifier) IsEnabled() bool {
	return n.config.Enabled && (n.config.FeishuWebhook != "" || n.config.DingTalkWebhook != "")
}
