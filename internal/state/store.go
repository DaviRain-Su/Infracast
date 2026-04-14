// Package state provides SQLite-based state management for infrastructure resources
package state

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var (
	// ErrResourceNotFound is returned when a resource is not found
	ErrResourceNotFound = errors.New("resource not found")
	// ErrConcurrentUpdate is returned when optimistic locking fails
	ErrConcurrentUpdate = errors.New("concurrent update detected")
)

// InfraResource represents an infrastructure resource record
type InfraResource struct {
	ID                 string    `json:"id"`
	EnvID              string    `json:"env_id"`
	ResourceName       string    `json:"resource_name"`
	ResourceType       string    `json:"resource_type"`
	ProviderResourceID string    `json:"provider_resource_id,omitempty"`
	SpecHash           string    `json:"spec_hash"`
	StateVersion       int       `json:"state_version"`
	ConfigJSON         string    `json:"config_json"`
	Status             string    `json:"status"`
	ErrorMsg           string    `json:"error_msg,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// Store provides SQLite-based state management
type Store struct {
	db *sql.DB
}

// NewStore creates a new SQLite store
func NewStore(dbPath string) (*Store, error) {
	if dbPath == "" {
		dbPath = ":memory:"
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enabling WAL mode: %w", err)
	}

	store := &Store{db: db}
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("initializing schema: %w", err)
	}

	return store, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// initSchema creates the database schema
func (s *Store) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS infra_resources (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
		env_id TEXT NOT NULL,
		resource_name TEXT NOT NULL,
		resource_type TEXT NOT NULL CHECK(resource_type IN ('database','cache','object_storage','compute')),
		provider_resource_id TEXT,
		spec_hash TEXT NOT NULL,
		state_version INTEGER NOT NULL DEFAULT 1 CHECK(state_version > 0),
		config_json TEXT NOT NULL,
		status TEXT NOT NULL CHECK(status IN ('pending','provisioning','provisioned','updating','failed','destroyed')),
		error_msg TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(env_id, resource_name)
	);

	CREATE INDEX IF NOT EXISTS idx_env_id ON infra_resources(env_id);
	CREATE INDEX IF NOT EXISTS idx_status ON infra_resources(status);
	`

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("creating schema: %w", err)
	}
	return nil
}

// UpsertResource creates or updates a resource with optimistic locking
// If the resource exists and spec_hash is the same, it's a no-op
// If the resource exists and spec_hash is different, state_version is incremented
func (s *Store) UpsertResource(ctx context.Context, resource *InfraResource) error {
	// For new resources (StateVersion == 0), we insert with version 1
	// For existing resources, we use the provided StateVersion for optimistic locking
	insertVersion := resource.StateVersion
	if insertVersion == 0 {
		insertVersion = 1
	}

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO infra_resources (
			env_id, resource_name, resource_type, provider_resource_id,
			spec_hash, state_version, config_json, status, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(env_id, resource_name) DO UPDATE SET
			spec_hash = excluded.spec_hash,
			state_version = CASE 
				WHEN infra_resources.spec_hash = excluded.spec_hash 
				THEN infra_resources.state_version 
				ELSE infra_resources.state_version + 1 
			END,
			config_json = excluded.config_json,
			status = excluded.status,
			provider_resource_id = excluded.provider_resource_id,
			updated_at = excluded.updated_at
		WHERE ? = 0 OR infra_resources.state_version = ?
	`,
		resource.EnvID,
		resource.ResourceName,
		resource.ResourceType,
		resource.ProviderResourceID,
		resource.SpecHash,
		insertVersion,
		resource.ConfigJSON,
		resource.Status,
		time.Now().UTC(),
		resource.StateVersion,
		resource.StateVersion,
	)

	if err != nil {
		return fmt.Errorf("upserting resource: %w", err)
	}

	// For updates, check if any row was actually updated
	if resource.StateVersion > 0 {
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			return ErrConcurrentUpdate
		}
	}

	return nil
}

// GetResource retrieves a resource by environment ID and resource name
// Returns (nil, nil) if resource not found (as per Tech Spec)
func (s *Store) GetResource(ctx context.Context, envID, resourceName string) (*InfraResource, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, env_id, resource_name, resource_type, provider_resource_id,
			spec_hash, state_version, config_json, status, error_msg, created_at, updated_at
		FROM infra_resources
		WHERE env_id = ? AND resource_name = ?
	`, envID, resourceName)

	var r InfraResource
	var createdAt, updatedAt sql.NullTime
	err := row.Scan(
		&r.ID, &r.EnvID, &r.ResourceName, &r.ResourceType, &r.ProviderResourceID,
		&r.SpecHash, &r.StateVersion, &r.ConfigJSON, &r.Status, &r.ErrorMsg,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Not found returns (nil, nil) per Tech Spec
		}
		return nil, fmt.Errorf("scanning resource: %w", err)
	}

	r.CreatedAt = createdAt.Time
	r.UpdatedAt = updatedAt.Time
	return &r, nil
}

// ListResourcesByEnv lists all resources for an environment
func (s *Store) ListResourcesByEnv(ctx context.Context, envID string) ([]*InfraResource, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, env_id, resource_name, resource_type, provider_resource_id,
			spec_hash, state_version, config_json, status, error_msg, created_at, updated_at
		FROM infra_resources
		WHERE env_id = ?
		ORDER BY resource_name
	`, envID)
	if err != nil {
		return nil, fmt.Errorf("querying resources: %w", err)
	}
	defer rows.Close()

	var resources []*InfraResource
	for rows.Next() {
		var r InfraResource
		var createdAt, updatedAt sql.NullTime
		err := rows.Scan(
			&r.ID, &r.EnvID, &r.ResourceName, &r.ResourceType, &r.ProviderResourceID,
			&r.SpecHash, &r.StateVersion, &r.ConfigJSON, &r.Status, &r.ErrorMsg,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}
		r.CreatedAt = createdAt.Time
		r.UpdatedAt = updatedAt.Time
		resources = append(resources, &r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return resources, nil
}

// DeleteResource deletes a resource
// No-op if resource not found (idempotent, per Tech Spec)
func (s *Store) DeleteResource(ctx context.Context, envID, resourceName string) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM infra_resources
		WHERE env_id = ? AND resource_name = ?
	`, envID, resourceName)
	if err != nil {
		return fmt.Errorf("deleting resource: %w", err)
	}
	// No-op if not found (idempotent)
	return nil
}

// ListEnvironments returns all unique environment IDs
func (s *Store) ListEnvironments(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT env_id
		FROM infra_resources
		ORDER BY env_id
	`)
	if err != nil {
		return nil, fmt.Errorf("querying environments: %w", err)
	}
	defer rows.Close()

	var envs []string
	for rows.Next() {
		var env string
		if err := rows.Scan(&env); err != nil {
			return nil, fmt.Errorf("scanning env: %w", err)
		}
		envs = append(envs, env)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return envs, nil
}
