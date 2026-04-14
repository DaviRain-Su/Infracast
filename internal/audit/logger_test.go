package audit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewLogger validates logger creation
func TestNewLogger(t *testing.T) {
	tmpDir := t.TempDir()
	config := Config{
		LogDir:        tmpDir,
		BufferSize:    10,
		FlushInterval: time.Second,
	}

	logger, err := NewLogger(config)
	require.NoError(t, err)
	defer logger.Close()

	assert.NotNil(t, logger)
	assert.Equal(t, 10, logger.bufferSize)
}

// TestLoggerWrite validates writing audit events
func TestLoggerWrite(t *testing.T) {
	tmpDir := t.TempDir()
	config := Config{
		LogDir:        tmpDir,
		BufferSize:    10,
		FlushInterval: time.Second,
	}

	logger, err := NewLogger(config)
	require.NoError(t, err)
	defer logger.Close()

	event := AuditEvent{
		Level:   LevelInfo,
		Action:  ActionDeploy,
		Message: "Deployment started",
		User:    "test-user",
		Env:     "dev",
	}

	err = logger.Write(event)
	require.NoError(t, err)

	// Flush to ensure it's written
	err = logger.Flush()
	require.NoError(t, err)

	// Verify file was created
	logFile := filepath.Join(tmpDir, "audit.log")
	_, err = os.Stat(logFile)
	assert.NoError(t, err)
}

// TestLoggerLog validates the Log method
func TestLoggerLog(t *testing.T) {
	tmpDir := t.TempDir()
	config := Config{
		LogDir:        tmpDir,
		BufferSize:    10,
		FlushInterval: time.Second,
	}

	logger, err := NewLogger(config)
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()
	err = logger.Log(ctx, LevelInfo, ActionInit, "Project initialized",
		WithUser("admin"),
		WithEnv("dev"),
		WithDetail("provider", "alicloud"),
	)
	require.NoError(t, err)

	err = logger.Flush()
	require.NoError(t, err)
}

// TestLoggerLogOperation validates operation logging with duration
func TestLoggerLogOperation(t *testing.T) {
	tmpDir := t.TempDir()
	config := Config{
		LogDir:        tmpDir,
		BufferSize:    10,
		FlushInterval: time.Second,
	}

	logger, err := NewLogger(config)
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()
	duration := 5 * time.Second

	// Success case
	err = logger.LogOperation(ctx, ActionDeploy, duration, nil,
		WithEnv("staging"),
	)
	require.NoError(t, err)

	// Error case
	testErr := assert.AnError
	err = logger.LogOperation(ctx, ActionDeploy, duration, testErr,
		WithEnv("production"),
	)
	require.NoError(t, err)

	err = logger.Flush()
	require.NoError(t, err)
}

// TestLogOptions validates log options
func TestLogOptions(t *testing.T) {
	event := &AuditEvent{}

	// Test WithUser
	WithUser("test-user")(event)
	assert.Equal(t, "test-user", event.User)

	// Test WithResource
	WithResource("sql-server-main")(event)
	assert.Equal(t, "sql-server-main", event.Resource)

	// Test WithEnv
	WithEnv("production")(event)
	assert.Equal(t, "production", event.Env)

	// Test WithDetail
	WithDetail("version", "1.0.0")(event)
	assert.Equal(t, "1.0.0", event.Details["version"])
}

// TestAuditEventFields validates event struct fields
func TestAuditEventFields(t *testing.T) {
	event := AuditEvent{
		ID:        "evt_123",
		Timestamp: time.Now(),
		Level:     LevelWarning,
		Action:    ActionDestroy,
		User:      "admin",
		Resource:  "redis-cache",
		Env:       "dev",
		Message:   "Resource destroyed",
		Details:   map[string]interface{}{"reason": "cleanup"},
		Duration:  2 * time.Second,
	}

	assert.Equal(t, "evt_123", event.ID)
	assert.Equal(t, LevelWarning, event.Level)
	assert.Equal(t, ActionDestroy, event.Action)
	assert.Equal(t, "cleanup", event.Details["reason"])
}

// TestDefaultConfig validates default configuration
func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, ".infra/audit", config.LogDir)
	assert.Equal(t, 100, config.BufferSize)
	assert.Equal(t, 5*time.Second, config.FlushInterval)
	assert.Equal(t, int64(10*1024*1024), config.MaxFileSize)
	assert.Equal(t, 5, config.MaxBackups)
}

// TestAuditLevelConstants validates level constants
func TestAuditLevelConstants(t *testing.T) {
	assert.Equal(t, AuditLevel("INFO"), LevelInfo)
	assert.Equal(t, AuditLevel("WARN"), LevelWarning)
	assert.Equal(t, AuditLevel("ERROR"), LevelError)
}

// TestActionConstants validates action constants
func TestActionConstants(t *testing.T) {
	assert.Equal(t, "init", ActionInit)
	assert.Equal(t, "provision", ActionProvision)
	assert.Equal(t, "deploy", ActionDeploy)
	assert.Equal(t, "destroy", ActionDestroy)
	assert.Equal(t, "rollback", ActionRollback)
}

// TestNewFileWriter validates file writer creation
func TestNewFileWriter(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")

	writer, err := NewFileWriter(filePath, 1024)
	require.NoError(t, err)
	defer writer.Close()

	assert.NotNil(t, writer)
	assert.Equal(t, filePath, writer.filePath)
	assert.Equal(t, int64(1024), writer.maxSize)
}

// TestFileWriterWrite validates writing to file
func TestFileWriterWrite(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "audit.log")

	writer, err := NewFileWriter(filePath, 1024*1024)
	require.NoError(t, err)
	defer writer.Close()

	event := AuditEvent{
		ID:      "evt_test",
		Level:   LevelInfo,
		Action:  ActionInit,
		Message: "Test event",
	}

	err = writer.Write(event)
	require.NoError(t, err)

	// Read and verify
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	var readEvent AuditEvent
	err = json.Unmarshal(data, &readEvent)
	require.NoError(t, err)
	assert.Equal(t, "evt_test", readEvent.ID)
	assert.Equal(t, ActionInit, readEvent.Action)
}

// TestGenerateEventID validates event ID generation
func TestGenerateEventID(t *testing.T) {
	id1 := generateEventID()
	id2 := generateEventID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.Contains(t, id1, "evt_")
}
