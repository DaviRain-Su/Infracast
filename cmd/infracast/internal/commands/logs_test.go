package commands

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNewLogsCommand validates logs command structure and flags
func TestNewLogsCommand(t *testing.T) {
	cmd := newLogsCommand()
	assert.NotNil(t, cmd)
	assert.Equal(t, "logs", cmd.Name())
	assert.Equal(t, "View audit logs and deployment history", cmd.Short)

	// Verify all flags exist with correct defaults
	tests := []struct {
		name     string
		defValue string
		short    string
	}{
		{"env", "", "e"},
		{"action", "", "a"},
		{"level", "", "l"},
		{"limit", "20", ""},
		{"since", "", ""},
		{"trace", "", ""},
		{"format", "table", "f"},
		{"output", "short", "o"},
	}

	for _, tt := range tests {
		flag := cmd.Flags().Lookup(tt.name)
		assert.NotNil(t, flag, "flag --%s should exist", tt.name)
		assert.Equal(t, tt.defValue, flag.DefValue, "flag --%s default", tt.name)
		if tt.short != "" {
			assert.Equal(t, tt.short, flag.Shorthand, "flag --%s shorthand", tt.name)
		}
	}
}

// TestLogsCommandRegistered verifies logs is registered in root command
func TestLogsCommandRegistered(t *testing.T) {
	root := NewRootCommand("test", "abc123", "2026-01-01")
	commands := root.Commands()
	found := false
	for _, cmd := range commands {
		if cmd.Name() == "logs" {
			found = true
			break
		}
	}
	assert.True(t, found, "logs command should be registered in root")
}

// TestStatusCommandHasNoOutputFlag regression: status --output doesn't exist
// Sprint W3-3/W4-6 found docs referencing `infracast status --output url`
// which caused confusion. This test ensures status never gains --output
// without intentional design.
func TestStatusCommandHasNoOutputFlag(t *testing.T) {
	cmd := newStatusCommand()
	flag := cmd.Flags().Lookup("output")
	assert.Nil(t, flag, "status command must not have --output flag (known limitation)")
}

// TestParseDuration validates duration parsing including day format
func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"1h", 1 * time.Hour, false},
		{"24h", 24 * time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"10m", 10 * time.Minute, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			d, err := parseDuration(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, d)
			}
		})
	}
}

// TestTruncateString validates string truncation with ellipsis
func TestTruncateString(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"exactly ten", 11, "exactly ten"},
		{"this is a long message that should be truncated", 20, "this is a long me..."},
		{"abc", 3, "abc"},
		{"abcd", 3, "..."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := truncateString(tt.input, tt.maxLen)
			assert.Equal(t, tt.want, got)
			assert.LessOrEqual(t, len(got), tt.maxLen)
		})
	}
}

// TestFormatStatus validates status color-coding logic
func TestFormatStatus(t *testing.T) {
	// formatStatus should return "-" for empty, pass through unknown values
	assert.Equal(t, "-", formatStatus(""))
	// Non-empty statuses should return non-empty strings
	assert.NotEmpty(t, formatStatus("ok"))
	assert.NotEmpty(t, formatStatus("fail"))
	assert.NotEmpty(t, formatStatus("skip"))
	assert.Equal(t, "unknown", formatStatus("unknown"))
}

// TestLogsOptionsDefaults validates LogsOptions zero values are sensible
func TestLogsOptionsDefaults(t *testing.T) {
	opts := LogsOptions{}
	assert.Equal(t, "", opts.Env)
	assert.Equal(t, "", opts.Action)
	assert.Equal(t, "", opts.Level)
	assert.Equal(t, 0, opts.Limit)
	assert.Equal(t, "", opts.Format)
	assert.Equal(t, "", opts.Output)
}
