package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// AuditLevel represents the severity level of an audit event
type AuditLevel string

const (
	AuditLevelInfo    AuditLevel = "INFO"
	AuditLevelWarning AuditLevel = "WARN"
	AuditLevelError   AuditLevel = "ERROR"
)

// AuditEvent represents a single audit log entry
type AuditEvent struct {
	ID        string                 `json:"id"`
	TraceID   string                 `json:"trace_id,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Level     AuditLevel             `json:"level"`
	Action    string                 `json:"action"`
	Step      string                 `json:"step,omitempty"`
	Status    string                 `json:"status,omitempty"`
	User      string                 `json:"user"`
	Resource  string                 `json:"resource,omitempty"`
	Env       string                 `json:"env,omitempty"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Duration  time.Duration          `json:"duration,omitempty"`
	Error     string                 `json:"error,omitempty"`
	ErrorCode string                 `json:"error_code,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
}

// AuditStore provides audit log operations
type AuditStore struct {
	db *sql.DB
}

// NewAuditStore creates a new audit store
func NewAuditStore(db *sql.DB) *AuditStore {
	return &AuditStore{db: db}
}

// InitAuditTable creates the audit log table if not exists
func (s *AuditStore) InitAuditTable() error {
	createTable := `
	CREATE TABLE IF NOT EXISTS audit_log (
		id TEXT PRIMARY KEY,
		trace_id TEXT,
		timestamp INTEGER NOT NULL,
		level TEXT NOT NULL,
		action TEXT NOT NULL,
		step TEXT,
		status TEXT,
		user TEXT,
		resource TEXT,
		env TEXT,
		message TEXT NOT NULL,
		details TEXT,
		duration_ms INTEGER,
		error TEXT,
		error_code TEXT,
		request_id TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_log(timestamp);
	CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_log(action);
	CREATE INDEX IF NOT EXISTS idx_audit_env ON audit_log(env);
	CREATE INDEX IF NOT EXISTS idx_audit_trace_id ON audit_log(trace_id);
	`
	if _, err := s.db.Exec(createTable); err != nil {
		return err
	}

	// Idempotent migration: add columns that may not exist in older DBs.
	// SQLite ignores ADD COLUMN if it already exists when we catch the error.
	migrations := []string{
		"ALTER TABLE audit_log ADD COLUMN trace_id TEXT",
		"ALTER TABLE audit_log ADD COLUMN step TEXT",
		"ALTER TABLE audit_log ADD COLUMN status TEXT",
		"ALTER TABLE audit_log ADD COLUMN error_code TEXT",
		"ALTER TABLE audit_log ADD COLUMN request_id TEXT",
	}
	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			if !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
				return fmt.Errorf("audit migration failed (%s): %w", m, err)
			}
		}
	}

	// Ensure index exists (idempotent via IF NOT EXISTS)
	_, err := s.db.Exec("CREATE INDEX IF NOT EXISTS idx_audit_trace_id ON audit_log(trace_id)")
	return err
}

// Write writes an audit event to the database
func (s *AuditStore) Write(ctx context.Context, event AuditEvent) error {
	// Generate ID if not set
	if event.ID == "" {
		event.ID = generateAuditID()
	}

	// Set timestamp if not set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	detailsJSON := ""
	if event.Details != nil {
		// JSON marshaling for details
		bytes, err := json.Marshal(event.Details)
		if err == nil {
			detailsJSON = string(bytes)
		}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO audit_log (id, trace_id, timestamp, level, action, step, status, user, resource, env, message, details, duration_ms, error, error_code, request_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		event.ID,
		event.TraceID,
		event.Timestamp.UnixNano()/int64(time.Millisecond),
		string(event.Level),
		event.Action,
		event.Step,
		event.Status,
		event.User,
		event.Resource,
		event.Env,
		event.Message,
		detailsJSON,
		int64(event.Duration.Milliseconds()),
		event.Error,
		event.ErrorCode,
		event.RequestID,
	)

	return err
}

// Log creates and writes a new audit event
func (s *AuditStore) Log(ctx context.Context, level AuditLevel, action, message string, opts ...AuditLogOption) error {
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

	return s.Write(ctx, event)
}

// LogOperation logs an operation with timing
func (s *AuditStore) LogOperation(ctx context.Context, action string, duration time.Duration, err error, opts ...AuditLogOption) error {
	level := AuditLevelInfo
	if err != nil {
		level = AuditLevelError
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

	return s.Write(ctx, event)
}

// QueryOptions contains query filters
type QueryOptions struct {
	Env       string
	Action    string
	Limit     int
	Since     time.Time
	Level     AuditLevel
	TraceID   string
}

// Query retrieves audit events matching the options
func (s *AuditStore) Query(ctx context.Context, opts QueryOptions) ([]AuditEvent, error) {
	if opts.Limit <= 0 {
		opts.Limit = 100
	}

	query := `SELECT id, trace_id, timestamp, level, action, step, status, user, resource, env, message, details, duration_ms, error, error_code, request_id
		FROM audit_log WHERE 1=1`
	args := []interface{}{}

	if opts.Env != "" {
		query += " AND env = ?"
		args = append(args, opts.Env)
	}

	if opts.Action != "" {
		query += " AND action = ?"
		args = append(args, opts.Action)
	}

	if opts.Level != "" {
		query += " AND level = ?"
		args = append(args, string(opts.Level))
	}

	if opts.TraceID != "" {
		query += " AND trace_id = ?"
		args = append(args, opts.TraceID)
	}

	if !opts.Since.IsZero() {
		query += " AND timestamp >= ?"
		args = append(args, opts.Since.UnixNano()/int64(time.Millisecond))
	}

	query += " ORDER BY timestamp DESC LIMIT ?"
	args = append(args, opts.Limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []AuditEvent
	for rows.Next() {
		var event AuditEvent
		var timestamp int64
		var durationMs sql.NullInt64
		var traceID, step, status, details, user, resource, env, errStr, errorCode, requestID sql.NullString

		err := rows.Scan(
			&event.ID,
			&traceID,
			&timestamp,
			&event.Level,
			&event.Action,
			&step,
			&status,
			&user,
			&resource,
			&env,
			&event.Message,
			&details,
			&durationMs,
			&errStr,
			&errorCode,
			&requestID,
		)
		if err != nil {
			continue
		}

		event.Timestamp = time.Unix(0, timestamp*int64(time.Millisecond))
		event.TraceID = traceID.String
		event.Step = step.String
		event.Status = status.String
		event.User = user.String
		event.Resource = resource.String
		event.Env = env.String
		if durationMs.Valid {
			event.Duration = time.Duration(durationMs.Int64) * time.Millisecond
		}
		event.Error = errStr.String
		event.ErrorCode = errorCode.String
		event.RequestID = requestID.String

		events = append(events, event)
	}

	return events, rows.Err()
}

// AuditLogOption is a functional option for configuring audit events
type AuditLogOption func(*AuditEvent)

// WithAuditUser sets the user for the audit event
func WithAuditUser(user string) AuditLogOption {
	return func(e *AuditEvent) {
		e.User = user
	}
}

// WithAuditResource sets the resource for the audit event
func WithAuditResource(resource string) AuditLogOption {
	return func(e *AuditEvent) {
		e.Resource = resource
	}
}

// WithAuditEnv sets the environment for the audit event
func WithAuditEnv(env string) AuditLogOption {
	return func(e *AuditEvent) {
		e.Env = env
	}
}

// WithAuditDetail adds a detail field to the audit event
func WithAuditDetail(key string, value interface{}) AuditLogOption {
	return func(e *AuditEvent) {
		if e.Details == nil {
			e.Details = make(map[string]interface{})
		}
		e.Details[key] = value
	}
}

// WithAuditTraceID sets the trace ID for correlating related events
func WithAuditTraceID(traceID string) AuditLogOption {
	return func(e *AuditEvent) {
		e.TraceID = traceID
	}
}

// WithAuditStep sets the step name within a deploy/provision pipeline
func WithAuditStep(step string) AuditLogOption {
	return func(e *AuditEvent) {
		e.Step = step
	}
}

// WithAuditStatus sets the outcome status (e.g. "ok", "fail", "skip")
func WithAuditStatus(status string) AuditLogOption {
	return func(e *AuditEvent) {
		e.Status = status
	}
}

// WithAuditErrorCode sets a structured error code (e.g. "EDEPLOY001")
func WithAuditErrorCode(code string) AuditLogOption {
	return func(e *AuditEvent) {
		e.ErrorCode = code
	}
}

// WithAuditRequestID sets the provider request ID for tracing cloud API calls
func WithAuditRequestID(requestID string) AuditLogOption {
	return func(e *AuditEvent) {
		e.RequestID = requestID
	}
}

// generateAuditID generates a unique event ID
func generateAuditID() string {
	return fmt.Sprintf("evt_%d", time.Now().UnixNano())
}

// GenerateTraceID generates a unique trace ID for correlating a pipeline run
func GenerateTraceID() string {
	return fmt.Sprintf("trc_%d", time.Now().UnixNano())
}

// Common action constants
const (
	AuditActionInit       = "init"
	AuditActionProvision  = "provision"
	AuditActionDeploy     = "deploy"
	AuditActionDestroy    = "destroy"
	AuditActionRollback   = "rollback"
	AuditActionEnvCreate  = "env.create"
	AuditActionEnvDelete  = "env.delete"
	AuditActionConfigView = "config.view"
	AuditActionConfigSet  = "config.set"
)
