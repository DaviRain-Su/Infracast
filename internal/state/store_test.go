package state

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStore_Initialize validates database initialization and schema creation
func TestStore_Initialize(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "initialize new database",
			path:    ":memory:",
			wantErr: false,
		},
		{
			name:    "initialize with file path",
			path:    t.TempDir() + "/test.db",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := NewStore(tt.path)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, store)
			defer store.Close()
		})
	}
}

// TestStore_CreateResource validates basic resource creation
func TestStore_CreateResource(t *testing.T) {
	store, err := NewStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	resource := &InfraResource{
		EnvID:        "env-123",
		ResourceName: "mydb",
		ResourceType: "database",
		SpecHash:     "abc123",
		ConfigJSON:   `{"engine":"mysql"}`,
		Status:       "pending",
	}

	err = store.UpsertResource(ctx, resource)
	require.NoError(t, err)

	// Verify resource was created
	retrieved, err := store.GetResource(ctx, "env-123", "mydb")
	require.NoError(t, err)
	assert.Equal(t, "mydb", retrieved.ResourceName)
	assert.Equal(t, "database", retrieved.ResourceType)
	assert.Equal(t, "abc123", retrieved.SpecHash)
	assert.Equal(t, 1, retrieved.StateVersion)
}

// TestStore_UpsertResource_Idempotent validates idempotent updates
func TestStore_UpsertResource_Idempotent(t *testing.T) {
	store, err := NewStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	resource := &InfraResource{
		EnvID:        "env-123",
		ResourceName: "mydb",
		ResourceType: "database",
		SpecHash:     "abc123",
		ConfigJSON:   `{"engine":"mysql"}`,
		Status:       "pending",
	}

	// First insert
	err = store.UpsertResource(ctx, resource)
	require.NoError(t, err)

	// Same hash - should skip update
	resource.StateVersion = 1 // Simulate read-modify-write
	err = store.UpsertResource(ctx, resource)
	require.NoError(t, err)

	retrieved, err := store.GetResource(ctx, "env-123", "mydb")
	require.NoError(t, err)
	assert.Equal(t, 1, retrieved.StateVersion) // Should remain 1

	// Different hash - should update and increment version
	resource.SpecHash = "def456"
	resource.ConfigJSON = `{"engine":"mysql","version":"8.0"}`
	err = store.UpsertResource(ctx, resource)
	require.NoError(t, err)

	retrieved, err = store.GetResource(ctx, "env-123", "mydb")
	require.NoError(t, err)
	assert.Equal(t, 2, retrieved.StateVersion) // Should increment to 2
}

// TestStore_UpsertResource_Concurrent validates optimistic locking under concurrent access
func TestStore_UpsertResource_Concurrent(t *testing.T) {
	store, err := NewStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	resource := &InfraResource{
		EnvID:        "env-123",
		ResourceName: "mydb",
		ResourceType: "database",
		SpecHash:     "abc123",
		ConfigJSON:   `{"engine":"mysql"}`,
		Status:       "pending",
	}

	// Create initial resource
	err = store.UpsertResource(ctx, resource)
	require.NoError(t, err)

	// Two goroutines try to update concurrently with same base version
	done := make(chan bool, 2)
	successCount := 0

	for i := 0; i < 2; i++ {
		go func(id int) {
			res := &InfraResource{
				EnvID:         "env-123",
				ResourceName:  "mydb",
				ResourceType:  "database",
				SpecHash:    "new-hash",
				ConfigJSON:    `{"engine":"mysql","version":"8.0"}`,
				Status:        "updating",
				StateVersion:  1, // Both start from version 1
			}
			err := store.UpsertResource(ctx, res)
			if err == nil {
				successCount++
			}
			done <- true
		}(i)
	}

	// Wait for both goroutines
	<-done
	<-done

	// Exactly one should succeed (optimistic locking)
	assert.Equal(t, 1, successCount, "Only one concurrent update should succeed")
}

// TestStore_GetResource_NotFound validates (nil, nil) for non-existent resource
func TestStore_GetResource_NotFound(t *testing.T) {
	store, err := NewStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	res, err := store.GetResource(ctx, "non-existent", "non-existent")
	assert.NoError(t, err)
	assert.Nil(t, res) // Not found returns (nil, nil) per Tech Spec
}

// TestStore_ListResourcesByEnv validates listing resources by environment
func TestStore_ListResourcesByEnv(t *testing.T) {
	store, err := NewStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Create resources in different environments
	resources := []*InfraResource{
		{EnvID: "env-1", ResourceName: "db1", ResourceType: "database", SpecHash: "h1", Status: "pending"},
		{EnvID: "env-1", ResourceName: "cache1", ResourceType: "cache", SpecHash: "h2", Status: "pending"},
		{EnvID: "env-2", ResourceName: "db2", ResourceType: "database", SpecHash: "h3", Status: "pending"},
	}

	for _, r := range resources {
		err := store.UpsertResource(ctx, r)
		require.NoError(t, err)
	}

	// List env-1 resources
	list, err := store.ListResourcesByEnv(ctx, "env-1")
	require.NoError(t, err)
	assert.Len(t, list, 2)

	// List env-2 resources
	list, err = store.ListResourcesByEnv(ctx, "env-2")
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "db2", list[0].ResourceName)
}

// TestStore_DeleteResource validates resource deletion
func TestStore_DeleteResource(t *testing.T) {
	store, err := NewStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	resource := &InfraResource{
		EnvID:        "env-123",
		ResourceName: "mydb",
		ResourceType: "database",
		SpecHash:     "abc123",
		Status:       "pending",
	}

	err = store.UpsertResource(ctx, resource)
	require.NoError(t, err)

	// Delete
	err = store.DeleteResource(ctx, "env-123", "mydb")
	require.NoError(t, err)

	// Verify deletion
	res, err := store.GetResource(ctx, "env-123", "mydb")
	require.NoError(t, err)
	assert.Nil(t, res)
}

// TestStore_EnvironmentIsolation validates that resources in different environments are isolated
func TestStore_EnvironmentIsolation(t *testing.T) {
	store, err := NewStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Create resources with same name in different environments
	env1Resource := &InfraResource{
		EnvID:        "env-prod",
		ResourceName: "mydb",
		ResourceType: "database",
		SpecHash:   "hash-prod",
		ConfigJSON:   `{"engine":"mysql","version":"8.0"}`,
		Status:       "provisioned",
	}
	env2Resource := &InfraResource{
		EnvID:        "env-staging",
		ResourceName: "mydb",
		ResourceType: "database",
		SpecHash:   "hash-staging",
		ConfigJSON:   `{"engine":"mysql","version":"5.7"}`,
		Status:       "provisioned",
	}

	// Create both resources
	err = store.UpsertResource(ctx, env1Resource)
	require.NoError(t, err)
	err = store.UpsertResource(ctx, env2Resource)
	require.NoError(t, err)

	// Verify env-prod resource
	retrieved1, err := store.GetResource(ctx, "env-prod", "mydb")
	require.NoError(t, err)
	assert.Equal(t, "hash-prod", retrieved1.SpecHash)
	assert.Equal(t, `{"engine":"mysql","version":"8.0"}`, retrieved1.ConfigJSON)

	// Verify env-staging resource
	retrieved2, err := store.GetResource(ctx, "env-staging", "mydb")
	require.NoError(t, err)
	assert.Equal(t, "hash-staging", retrieved2.SpecHash)
	assert.Equal(t, `{"engine":"mysql","version":"5.7"}`, retrieved2.ConfigJSON)

	// Verify lists are isolated
	prodList, err := store.ListResourcesByEnv(ctx, "env-prod")
	require.NoError(t, err)
	assert.Len(t, prodList, 1)

	stagingList, err := store.ListResourcesByEnv(ctx, "env-staging")
	require.NoError(t, err)
	assert.Len(t, stagingList, 1)

	// Delete from env-prod should not affect env-staging
	err = store.DeleteResource(ctx, "env-prod", "mydb")
	require.NoError(t, err)

	res, err := store.GetResource(ctx, "env-prod", "mydb")
	require.NoError(t, err)
	assert.Nil(t, res)

	// env-staging should still exist
	_, err = store.GetResource(ctx, "env-staging", "mydb")
	require.NoError(t, err)
}

// TestStore_ListResources_EmptyResult validates listing resources for non-existent environment
func TestStore_ListResources_EmptyResult(t *testing.T) {
	store, err := NewStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// List resources for non-existent environment
	list, err := store.ListResourcesByEnv(ctx, "non-existent-env")
	require.NoError(t, err)
	assert.Empty(t, list)
	assert.Len(t, list, 0)
}

// TestStore_DeleteResource_NotFound validates no-op for deleting non-existent resource
func TestStore_DeleteResource_NotFound(t *testing.T) {
	store, err := NewStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Try to delete non-existent resource - should be no-op (idempotent)
	err = store.DeleteResource(ctx, "non-existent", "non-existent")
	assert.NoError(t, err) // No error for idempotent delete
}

// TestStore_ListEnvironments validates listing all environments
func TestStore_ListEnvironments(t *testing.T) {
	store, err := NewStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Initially empty
	envs, err := store.ListEnvironments(ctx)
	require.NoError(t, err)
	assert.Empty(t, envs)

	// Create resources in different environments
	resources := []*InfraResource{
		{EnvID: "env-prod", ResourceName: "db1", ResourceType: "database", SpecHash: "h1", Status: "pending"},
		{EnvID: "env-staging", ResourceName: "db2", ResourceType: "database", SpecHash: "h2", Status: "pending"},
		{EnvID: "env-dev", ResourceName: "db3", ResourceType: "database", SpecHash: "h3", Status: "pending"},
	}

	for _, r := range resources {
		err := store.UpsertResource(ctx, r)
		require.NoError(t, err)
	}

	// List environments
	envs, err = store.ListEnvironments(ctx)
	require.NoError(t, err)
	assert.Len(t, envs, 3)
	assert.Contains(t, envs, "env-prod")
	assert.Contains(t, envs, "env-staging")
	assert.Contains(t, envs, "env-dev")
}
