package notify

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNewNotifier validates notifier creation
func TestNewNotifier(t *testing.T) {
	config := &Config{
		FeishuWebhook:   "https://feishu.example.com/webhook",
		DingTalkWebhook: "https://dingtalk.example.com/webhook",
		Enabled:         true,
	}

	notifier := NewNotifier(config)
	assert.NotNil(t, notifier)
	assert.Equal(t, config, notifier.config)
	assert.NotNil(t, notifier.client)
}

// TestNewNotifierWithNilConfig validates creation with nil config
func TestNewNotifierWithNilConfig(t *testing.T) {
	notifier := NewNotifier(nil)
	assert.NotNil(t, notifier)
	assert.NotNil(t, notifier.config)
	assert.False(t, notifier.config.Enabled)
}

// TestDeploymentEventFields validates event struct
func TestDeploymentEventFields(t *testing.T) {
	event := &DeploymentEvent{
		AppName:    "myapp",
		Env:        "staging",
		Commit:     "abc123",
		Status:     "success",
		Duration:   5 * time.Minute,
		Error:      "",
		StartedAt:  time.Now().Add(-5 * time.Minute),
		FinishedAt: time.Now(),
	}

	assert.Equal(t, "myapp", event.AppName)
	assert.Equal(t, "staging", event.Env)
	assert.Equal(t, "abc123", event.Commit)
	assert.Equal(t, "success", event.Status)
	assert.Equal(t, 5*time.Minute, event.Duration)
}

// TestIsEnabled validates enabled check
func TestIsEnabled(t *testing.T) {
	// Enabled with webhooks
	notifier := NewNotifier(&Config{
		FeishuWebhook: "https://feishu.example.com",
		Enabled:       true,
	})
	assert.True(t, notifier.IsEnabled())

	// Disabled
	notifier = NewNotifier(&Config{
		FeishuWebhook: "https://feishu.example.com",
		Enabled:       false,
	})
	assert.False(t, notifier.IsEnabled())

	// Enabled but no webhooks
	notifier = NewNotifier(&Config{
		Enabled: true,
	})
	assert.False(t, notifier.IsEnabled())
}

// TestFormatFeishuMessage validates message formatting
func TestFormatFeishuMessage(t *testing.T) {
	notifier := NewNotifier(&Config{})
	event := &DeploymentEvent{
		AppName:  "myapp",
		Env:      "staging",
		Commit:   "abc123",
		Status:   "success",
		Duration: 5 * time.Minute,
	}

	message := notifier.formatFeishuMessage(event)
	assert.NotNil(t, message)
	assert.Contains(t, string(message), "myapp")
	assert.Contains(t, string(message), "staging")
	assert.Contains(t, string(message), "abc123")
	assert.Contains(t, string(message), "success")
}

// TestFormatFeishuMessageFailedStatus validates failed status formatting
func TestFormatFeishuMessageFailedStatus(t *testing.T) {
	notifier := NewNotifier(&Config{})
	event := &DeploymentEvent{
		AppName:  "myapp",
		Env:      "staging",
		Commit:   "abc123",
		Status:   "failed",
		Duration: 5 * time.Minute,
		Error:    "build failed",
	}

	message := notifier.formatFeishuMessage(event)
	assert.NotNil(t, message)
	assert.Contains(t, string(message), "failed")
	assert.Contains(t, string(message), "build failed")
}

// TestFormatDingTalkMessage validates DingTalk message formatting
func TestFormatDingTalkMessage(t *testing.T) {
	notifier := NewNotifier(&Config{})
	event := &DeploymentEvent{
		AppName:  "myapp",
		Env:      "staging",
		Commit:   "abc123",
		Status:   "success",
		Duration: 5 * time.Minute,
	}

	message := notifier.formatDingTalkMessage(event)
	assert.NotNil(t, message)
	assert.Contains(t, string(message), "myapp")
	assert.Contains(t, string(message), "staging")
	assert.Contains(t, string(message), "abc123")
}

// TestNotifySkippedWhenDisabled validates skip behavior
func TestNotifySkippedWhenDisabled(t *testing.T) {
	notifier := NewNotifier(&Config{
		Enabled: false,
	})

	event := &DeploymentEvent{
		AppName: "myapp",
		Status:  "success",
	}

	// Should return nil without sending
	err := notifier.Notify(nil, event)
	assert.NoError(t, err)
}

// TestNotifySkippedWhenNoWebhooks validates skip with no webhooks
func TestNotifySkippedWhenNoWebhooks(t *testing.T) {
	notifier := NewNotifier(&Config{
		Enabled: true,
	})

	event := &DeploymentEvent{
		AppName: "myapp",
		Status:  "success",
	}

	// Should return nil without sending
	err := notifier.Notify(nil, event)
	assert.NoError(t, err)
}

// TestConfigFields validates config struct
func TestConfigFields(t *testing.T) {
	config := &Config{
		FeishuWebhook:   "https://feishu.example.com/webhook",
		DingTalkWebhook: "https://dingtalk.example.com/webhook",
		Enabled:         true,
	}

	assert.Equal(t, "https://feishu.example.com/webhook", config.FeishuWebhook)
	assert.Equal(t, "https://dingtalk.example.com/webhook", config.DingTalkWebhook)
	assert.True(t, config.Enabled)
}
