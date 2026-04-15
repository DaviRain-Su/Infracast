package state

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	return db
}

// TestAuditStoreWrite validates writing audit events
func TestAuditStoreWrite(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewAuditStore(db)
	err := store.InitAuditTable()
	require.NoError(t, err)

	event := AuditEvent{
		ID:      "evt_test",
		Level:   AuditLevelInfo,
		Action:  AuditActionDeploy,
		Message: "Deployment started",
		User:    "test-user",
		Env:     "dev",
	}

	err = store.Write(context.Background(), event)
	require.NoError(t, err)
}

// TestAuditStoreLog validates the Log method
func TestAuditStoreLog(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewAuditStore(db)
	err := store.InitAuditTable()
	require.NoError(t, err)

	ctx := context.Background()
	err = store.Log(ctx, AuditLevelInfo, AuditActionInit, "Project initialized",
		WithAuditUser("admin"),
		WithAuditEnv("dev"),
		WithAuditDetail("provider", "alicloud"),
	)
	require.NoError(t, err)
}

// TestAuditStoreLogOperation validates operation logging with duration
func TestAuditStoreLogOperation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewAuditStore(db)
	err := store.InitAuditTable()
	require.NoError(t, err)

	ctx := context.Background()
	duration := 5 * time.Second

	// Success case
	err = store.LogOperation(ctx, AuditActionDeploy, duration, nil,
		WithAuditEnv("staging"),
	)
	require.NoError(t, err)

	// Error case
	testErr := assert.AnError
	err = store.LogOperation(ctx, AuditActionDeploy, duration, testErr,
		WithAuditEnv("production"),
	)
	require.NoError(t, err)
}

// TestAuditStoreQuery validates querying audit events
func TestAuditStoreQuery(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewAuditStore(db)
	err := store.InitAuditTable()
	require.NoError(t, err)

	ctx := context.Background()

	// Insert test events
	err = store.Log(ctx, AuditLevelInfo, AuditActionDeploy, "Deploy 1",
		WithAuditEnv("dev"))
	require.NoError(t, err)

	err = store.Log(ctx, AuditLevelInfo, AuditActionDeploy, "Deploy 2",
		WithAuditEnv("staging"))
	require.NoError(t, err)

	err = store.Log(ctx, AuditLevelError, AuditActionProvision, "Provision failed",
		WithAuditEnv("dev"))
	require.NoError(t, err)

	// Query all events
	events, err := store.Query(ctx, QueryOptions{Limit: 10})
	require.NoError(t, err)
	assert.Len(t, events, 3)

	// Query by env
	events, err = store.Query(ctx, QueryOptions{Env: "dev", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, events, 2)

	// Query by action
	events, err = store.Query(ctx, QueryOptions{Action: AuditActionProvision, Limit: 10})
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, AuditLevelError, events[0].Level)
}

// TestAuditLogOptions validates log options
func TestAuditLogOptions(t *testing.T) {
	event := &AuditEvent{}

	// Test WithAuditUser
	WithAuditUser("test-user")(event)
	assert.Equal(t, "test-user", event.User)

	// Test WithAuditResource
	WithAuditResource("sql-server-main")(event)
	assert.Equal(t, "sql-server-main", event.Resource)

	// Test WithAuditEnv
	WithAuditEnv("production")(event)
	assert.Equal(t, "production", event.Env)

	// Test WithAuditDetail
	WithAuditDetail("version", "1.0.0")(event)
	assert.Equal(t, "1.0.0", event.Details["version"])
}

// TestAuditEventFields validates event struct fields
func TestAuditEventFields(t *testing.T) {
	event := AuditEvent{
		ID:        "evt_123",
		Timestamp: time.Now(),
		Level:     AuditLevelWarning,
		Action:    AuditActionDestroy,
		User:      "admin",
		Resource:  "redis-cache",
		Env:       "dev",
		Message:   "Resource destroyed",
		Details:   map[string]interface{}{"reason": "cleanup"},
		Duration:  2 * time.Second,
	}

	assert.Equal(t, "evt_123", event.ID)
	assert.Equal(t, AuditLevelWarning, event.Level)
	assert.Equal(t, AuditActionDestroy, event.Action)
	assert.Equal(t, "cleanup", event.Details["reason"])
}

// TestAuditLevelConstants validates level constants
func TestAuditLevelConstants(t *testing.T) {
	assert.Equal(t, AuditLevel("INFO"), AuditLevelInfo)
	assert.Equal(t, AuditLevel("WARN"), AuditLevelWarning)
	assert.Equal(t, AuditLevel("ERROR"), AuditLevelError)
}

// TestActionConstants validates action constants
func TestAuditActionConstants(t *testing.T) {
	assert.Equal(t, "init", AuditActionInit)
	assert.Equal(t, "provision", AuditActionProvision)
	assert.Equal(t, "deploy", AuditActionDeploy)
	assert.Equal(t, "destroy", AuditActionDestroy)
	assert.Equal(t, "rollback", AuditActionRollback)
}

// TestGenerateAuditID validates event ID generation
func TestGenerateAuditID(t *testing.T) {
	id1 := generateAuditID()
	time.Sleep(1 * time.Millisecond)
	id2 := generateAuditID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.Contains(t, id1, "evt_")
}

// TestInitAuditTable validates table creation
func TestInitAuditTable(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewAuditStore(db)
	err := store.InitAuditTable()
	require.NoError(t, err)

	// Verify table exists by inserting a row
	event := AuditEvent{
		Level:   AuditLevelInfo,
		Action:  "test",
		Message: "Test message",
	}
	err = store.Write(context.Background(), event)
	require.NoError(t, err)
}
