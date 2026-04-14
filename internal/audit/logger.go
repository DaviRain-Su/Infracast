package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditLevel represents the severity level of an audit event
type AuditLevel string

const (
	LevelInfo    AuditLevel = "INFO"
	LevelWarning AuditLevel = "WARN"
	LevelError   AuditLevel = "ERROR"
)

// AuditEvent represents a single audit log entry
type AuditEvent struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Level     AuditLevel             `json:"level"`
	Action    string                 `json:"action"`
	User      string                 `json:"user"`
	Resource  string                 `json:"resource,omitempty"`
	Env       string                 `json:"env,omitempty"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Duration  time.Duration          `json:"duration,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// Logger handles audit log operations
type Logger struct {
	mu            sync.RWMutex
	writer        LogWriter
	buffer        []AuditEvent
	bufferSize    int
	flushInterval time.Duration
	stopCh        chan struct{}
}

// LogWriter defines the interface for writing audit logs
type LogWriter interface {
	Write(event AuditEvent) error
	Close() error
}

// FileWriter writes audit logs to a file
type FileWriter struct {
	mu       sync.Mutex
	file     *os.File
	encoder  *json.Encoder
	filePath string
	maxSize  int64 // max file size in bytes
}

// Config contains audit logger configuration
type Config struct {
	LogDir        string
	BufferSize    int
	FlushInterval time.Duration
	MaxFileSize   int64
	MaxBackups    int
}

// DefaultConfig returns default audit configuration
func DefaultConfig() Config {
	return Config{
		LogDir:        ".infra/audit",
		BufferSize:    100,
		FlushInterval: 5 * time.Second,
		MaxFileSize:   10 * 1024 * 1024, // 10MB
		MaxBackups:    5,
	}
}

// NewLogger creates a new audit logger
func NewLogger(config Config) (*Logger, error) {
	// Ensure log directory exists
	if err := os.MkdirAll(config.LogDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create audit log directory: %w", err)
	}

	// Create file writer
	filePath := filepath.Join(config.LogDir, "audit.log")
	writer, err := NewFileWriter(filePath, config.MaxFileSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create file writer: %w", err)
	}

	logger := &Logger{
		writer:        writer,
		buffer:        make([]AuditEvent, 0, config.BufferSize),
		bufferSize:    config.BufferSize,
		flushInterval: config.FlushInterval,
		stopCh:        make(chan struct{}),
	}

	// Start background flush routine
	go logger.flushRoutine()

	return logger, nil
}

// NewFileWriter creates a new file writer
func NewFileWriter(filePath string, maxSize int64) (*FileWriter, error) {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &FileWriter{
		file:     file,
		encoder:  json.NewEncoder(file),
		filePath: filePath,
		maxSize:  maxSize,
	}, nil
}

// Write writes an audit event to the log
func (l *Logger) Write(event AuditEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Generate ID if not set
	if event.ID == "" {
		event.ID = generateEventID()
	}

	// Set timestamp if not set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Add to buffer
	l.buffer = append(l.buffer, event)

	// Flush if buffer is full
	if len(l.buffer) >= l.bufferSize {
		return l.flushUnlocked()
	}

	return nil
}

// Log creates and writes a new audit event
func (l *Logger) Log(ctx context.Context, level AuditLevel, action, message string, opts ...LogOption) error {
	event := AuditEvent{
		Level:     level,
		Action:    action,
		Message:   message,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Apply options
	for _, opt := range opts {
		opt(&event)
	}

	return l.Write(event)
}

// LogOperation logs an operation with timing
func (l *Logger) LogOperation(ctx context.Context, action string, duration time.Duration, err error, opts ...LogOption) error {
	level := LevelInfo
	if err != nil {
		level = LevelError
	}

	event := AuditEvent{
		Level:     level,
		Action:    action,
		Timestamp: time.Now(),
		Duration:  duration,
		Details:   make(map[string]interface{}),
	}

	if err != nil {
		event.Error = err.Error()
	}

	// Apply options
	for _, opt := range opts {
		opt(&event)
	}

	return l.Write(event)
}

// Flush writes all buffered events to the underlying writer
func (l *Logger) Flush() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.flushUnlocked()
}

// flushUnlocked performs the actual flush (must hold lock)
func (l *Logger) flushUnlocked() error {
	if len(l.buffer) == 0 {
		return nil
	}

	// Write all buffered events
	for _, event := range l.buffer {
		if err := l.writer.Write(event); err != nil {
			return fmt.Errorf("failed to write audit event: %w", err)
		}
	}

	// Clear buffer
	l.buffer = l.buffer[:0]

	return nil
}

// flushRoutine runs in background to periodically flush the buffer
func (l *Logger) flushRoutine() {
	ticker := time.NewTicker(l.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = l.Flush()
		case <-l.stopCh:
			_ = l.Flush()
			return
		}
	}
}

// Close closes the logger and flushes any pending events
func (l *Logger) Close() error {
	close(l.stopCh)

	if err := l.Flush(); err != nil {
		return err
	}

	return l.writer.Close()
}

// Write writes a single event to the file
func (fw *FileWriter) Write(event AuditEvent) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	// Check if rotation is needed
	if fw.shouldRotate() {
		if err := fw.rotate(); err != nil {
			return err
		}
	}

	return fw.encoder.Encode(event)
}

// Close closes the file writer
func (fw *FileWriter) Close() error {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	return fw.file.Close()
}

// shouldRotate checks if the log file should be rotated
func (fw *FileWriter) shouldRotate() bool {
	if fw.maxSize <= 0 {
		return false
	}

	info, err := fw.file.Stat()
	if err != nil {
		return false
	}

	return info.Size() >= fw.maxSize
}

// rotate performs log file rotation
func (fw *FileWriter) rotate() error {
	// Close current file
	if err := fw.file.Close(); err != nil {
		return err
	}

	// Rename current file with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s.%s", fw.filePath, timestamp)
	if err := os.Rename(fw.filePath, backupPath); err != nil {
		return err
	}

	// Create new file
	file, err := os.OpenFile(fw.filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	fw.file = file
	fw.encoder = json.NewEncoder(file)

	return nil
}

// LogOption is a functional option for configuring audit events
type LogOption func(*AuditEvent)

// WithUser sets the user for the audit event
func WithUser(user string) LogOption {
	return func(e *AuditEvent) {
		e.User = user
	}
}

// WithResource sets the resource for the audit event
func WithResource(resource string) LogOption {
	return func(e *AuditEvent) {
		e.Resource = resource
	}
}

// WithEnv sets the environment for the audit event
func WithEnv(env string) LogOption {
	return func(e *AuditEvent) {
		e.Env = env
	}
}

// WithDetail adds a detail field to the audit event
func WithDetail(key string, value interface{}) LogOption {
	return func(e *AuditEvent) {
		if e.Details == nil {
			e.Details = make(map[string]interface{})
		}
		e.Details[key] = value
	}
}

// generateEventID generates a unique event ID
func generateEventID() string {
	return fmt.Sprintf("evt_%d", time.Now().UnixNano())
}

// Common action constants
const (
	ActionInit       = "init"
	ActionProvision  = "provision"
	ActionDeploy     = "deploy"
	ActionDestroy    = "destroy"
	ActionRollback   = "rollback"
	ActionEnvCreate  = "env.create"
	ActionEnvDelete  = "env.delete"
	ActionConfigView = "config.view"
	ActionConfigSet  = "config.set"
)
