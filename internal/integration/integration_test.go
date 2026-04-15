// Package integration provides end-to-end integration tests
package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/DaviRain-Su/infracast/internal/config"
	"github.com/DaviRain-Su/infracast/internal/credentials"
	"github.com/DaviRain-Su/infracast/internal/provisioner"
	"github.com/DaviRain-Su/infracast/internal/state"
	"github.com/DaviRain-Su/infracast/pkg/infragen"
	"github.com/DaviRain-Su/infracast/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTest creates provisioner and store for testing
func setupTest(t *testing.T) (*provisioner.Provisioner, *state.Store, context.Context) {
	ctx := context.Background()
	store, err := state.NewStore(":memory:")
	require.NoError(t, err)

	creds := credentials.NewManager()
	creds.Store("alicloud", "AK123", "SK456", "cn-hangzhou")

	prov := provisioner.NewProvisioner(store, creds)
	return prov, store, ctx
}

// TestPipeline_MockProvider_FullCycle validates the complete Map→Provision→Generate→Verify→Idempotent cycle
// Tech Spec §8.1: THE core integration test
func TestPipeline_MockProvider_FullCycle(t *testing.T) {
	prov, store, ctx := setupTest(t)
	tmpDir := t.TempDir()

	// Setup: Create mock provider
	mockProvider := NewMockCloudProvider("alicloud")

	// Phase 1: Map - Define resources explicitly
	resources := []providers.ResourceSpec{
		{
			Type: "database",
			DatabaseSpec: &providers.DatabaseSpec{
				Name:      "users",
				Engine:    "postgresql",
				Version:   "15",
				StorageGB: 20,
			},
		},
		{
			Type: "cache",
			CacheSpec: &providers.CacheSpec{
				Name:           "session",
				Engine:         "redis",
				Version:        "7",
				MemoryMB:       256,
				EvictionPolicy: "allkeys-lru",
			},
		},
		{
			Type: "object_storage",
			ObjectStorageSpec: &providers.ObjectStorageSpec{
				Name: "assets",
				ACL:  "private",
			},
		},
	}

	// Phase 2: Provision - First run (CREATE)
	input := provisioner.ProvisionInput{
		EnvID:     "production",
		Resources: resources,
		Provider:  mockProvider,
		Credentials: credentials.CredentialConfig{
			Provider: "alicloud",
			Region:   "cn-hangzhou",
		},
	}

	result1, err := prov.Provision(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, result1)
	require.True(t, result1.Success, "First provision should succeed")
	require.Len(t, result1.Resources, 3, "Should provision 3 resources")

	// Verify all resources created
	for _, res := range result1.Resources {
		assert.True(t, res.Success)
		assert.Equal(t, "create", res.Action)
	}

	// Verify state persistence
	stateResources, err := store.ListResourcesByEnv(ctx, "production")
	require.NoError(t, err)
	require.Len(t, stateResources, 3, "Should have 3 resources in state")

	// Verify mock provider received the resources
	_, ok := mockProvider.GetDatabase("users")
	assert.True(t, ok, "Database should be provisioned")
	_, ok = mockProvider.GetCache("session")
	assert.True(t, ok, "Cache should be provisioned")

	// Phase 3: Generate - Create infrcfg.json
	generator := infragen.NewGenerator(nil)
	cfg := &infragen.InfraCfg{
		SQLServers: map[string]infragen.SQLServer{
			"users": {
				Host:     "users-db.example.com",
				Port:     5432,
				Database: "users",
				User:     "app",
				Password: "${USERS_DB_PASSWORD}",
				TLS:      &infragen.TLSConfig{Enabled: true},
			},
		},
		Redis: map[string]infragen.RedisServer{
			"session": {
				Host:      "session-cache.example.com",
				Port:      6379,
				Password:  "${SESSION_CACHE_PASSWORD}",
				KeyPrefix: "app:",
				Auth:      &infragen.AuthConfig{Enabled: true},
			},
		},
		ObjectStorage: map[string]infragen.ObjectStore{
			"assets": {
				Type:      "S3",
				Endpoint:  "https://oss-cn-hangzhou.aliyuncs.com",
				Bucket:    "assets",
				Region:    "cn-hangzhou",
				Provider:  "alicloud",
				AccessKey: "${OSS_ACCESS_KEY}",
				SecretKey: "${OSS_SECRET_KEY}",
			},
		},
	}

	cfgPath := filepath.Join(tmpDir, "infracfg.json")
	err = generator.Write(cfg, cfgPath)
	require.NoError(t, err)

	// Verify config file
	_, err = os.Stat(cfgPath)
	require.NoError(t, err)
	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "sql_servers")
	assert.Contains(t, string(data), "users-db.example.com")

	// Phase 4: Second Provision - Idempotency (all SKIP)
	result2, err := prov.Provision(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, result2)
	require.True(t, result2.Success, "Second provision should succeed")
	require.Len(t, result2.Resources, 3)

	// Verify all resources skipped (noop)
	for _, res := range result2.Resources {
		assert.True(t, res.Success)
		assert.Equal(t, "noop", res.Action, "Should skip unchanged resources")
	}

	// Verify state versions unchanged (idempotency)
	stateResources2, err := store.ListResourcesByEnv(ctx, "production")
	require.NoError(t, err)
	for i := range stateResources2 {
		assert.Equal(t, stateResources[i].StateVersion, stateResources2[i].StateVersion,
			"State version should not change for noop")
	}

	fmt.Printf("✓ Full cycle complete: %d resources, idempotency verified\n", len(result1.Resources))
}

// TestPipeline_StateVersionIncrement validates version tracking across changes
// Tech Spec §8.2: Three-scenario test
func TestPipeline_StateVersionIncrement(t *testing.T) {
	prov, store, ctx := setupTest(t)
	mockProvider := NewMockCloudProvider("alicloud")

	// Initial resources
	resources := []providers.ResourceSpec{
		{
			Type: "database",
			DatabaseSpec: &providers.DatabaseSpec{
				Name:      "mydb",
				Engine:    "postgresql",
				Version:   "15",
				StorageGB: 20, // Initial: 20GB
			},
		},
	}

	input := provisioner.ProvisionInput{
		EnvID:     "test-env",
		Resources: resources,
		Provider:  mockProvider,
		Credentials: credentials.CredentialConfig{
			Provider: "alicloud",
			Region:   "cn-hangzhou",
		},
	}

	// Scenario 1: First deploy → all version=1
	result1, err := prov.Provision(ctx, input)
	require.NoError(t, err)
	require.True(t, result1.Success)

	resource, err := store.GetResource(ctx, "test-env", "mydb")
	require.NoError(t, err)
	require.NotNil(t, resource)
	assert.Equal(t, 1, resource.StateVersion, "First provision should create version 1")

	// Scenario 2: Same config → all skip, versions unchanged
	result2, err := prov.Provision(ctx, input)
	require.NoError(t, err)
	require.True(t, result2.Success)

	resource2, err := store.GetResource(ctx, "test-env", "mydb")
	require.NoError(t, err)
	assert.Equal(t, 1, resource2.StateVersion, "Noop should not change version")
	assert.Equal(t, "noop", result2.Resources[0].Action)

	// Scenario 3: Modified config → changed resources version=2
	resources[0].DatabaseSpec.StorageGB = 50 // Change: 20GB → 50GB
	result3, err := prov.Provision(ctx, input)
	require.NoError(t, err)
	require.True(t, result3.Success)

	resource3, err := store.GetResource(ctx, "test-env", "mydb")
	require.NoError(t, err)
	assert.Equal(t, 2, resource3.StateVersion, "Update should increment version to 2")
	assert.Equal(t, "update", result3.Resources[0].Action)
}

// TestPipeline_DryRun validates dry-run mode
func TestPipeline_DryRun(t *testing.T) {
	prov, store, ctx := setupTest(t)
	mockProvider := NewMockCloudProvider("alicloud")

	resources := []providers.ResourceSpec{
		{
			Type: "database",
			DatabaseSpec: &providers.DatabaseSpec{
				Name:      "testdb",
				Engine:    "postgresql",
				StorageGB: 20,
			},
		},
	}

	input := provisioner.ProvisionInput{
		EnvID:     "dryrun-env",
		Resources: resources,
		Provider:  mockProvider,
		DryRun:    true, // Enable dry-run
		Credentials: credentials.CredentialConfig{
			Provider: "alicloud",
			Region:   "cn-hangzhou",
		},
	}

	// Execute dry-run
	result, err := prov.Provision(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Success)

	// Verify plan is returned
	assert.NotNil(t, result.Plan)
	assert.NotEmpty(t, result.Plan.Resources)

	// Verify no actual state changes
	stateResources, err := store.ListResourcesByEnv(ctx, "dryrun-env")
	require.NoError(t, err)
	assert.Empty(t, stateResources, "Dry-run should not modify state")

	// Verify mock provider was NOT called
	_, ok := mockProvider.GetDatabase("testdb")
	assert.False(t, ok, "Dry-run should not provision resources")
}

// TestPipeline_ConfigDriven validates provisioning from config file
func TestPipeline_ConfigDriven(t *testing.T) {
	tmpDir := t.TempDir()
	prov, store, ctx := setupTest(t)
	mockProvider := NewMockCloudProvider("alicloud")

	// Create config file
	cfgPath := filepath.Join(tmpDir, "infracast.yaml")
	cfgContent := `
provider: alicloud
region: cn-hangzhou
environments:
  production:
    provider: alicloud
    region: cn-shanghai
overrides:
  databases:
    mydb:
      storage_gb: 100
`
	err := os.WriteFile(cfgPath, []byte(cfgContent), 0644)
	require.NoError(t, err)

	// Load config
	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)

	// Resolve environment
	resolved, err := cfg.ResolveEnv("production")
	require.NoError(t, err)
	assert.Equal(t, "cn-shanghai", resolved.Region)

	// Apply override
	override, exists := cfg.GetDatabaseOverride("mydb")
	require.True(t, exists)
	assert.Equal(t, 100, override.StorageGB)

	// Provision with resolved config
	resources := []providers.ResourceSpec{
		{
			Type: "database",
			DatabaseSpec: &providers.DatabaseSpec{
				Name:      "mydb",
				Engine:    "postgresql",
				StorageGB: override.StorageGB, // Apply override
			},
		},
	}

	input := provisioner.ProvisionInput{
		EnvID:     "production",
		Resources: resources,
		Provider:  mockProvider,
		Credentials: credentials.CredentialConfig{
			Provider: resolved.Provider,
			Region:   resolved.Region,
		},
	}

	result, err := prov.Provision(ctx, input)
	require.NoError(t, err)
	require.True(t, result.Success)

	// Verify state
	stateResources, err := store.ListResourcesByEnv(ctx, "production")
	require.NoError(t, err)
	assert.Len(t, stateResources, 1)
}

// TestPipeline_PartialFailure validates failure isolation
// Injects failure to verify failed resource doesn't affect others
func TestPipeline_PartialFailure(t *testing.T) {
	prov, store, ctx := setupTest(t)
	mockProvider := NewMockCloudProvider("alicloud")

	// Phase 1: Initial resources - all succeed
	resources := []providers.ResourceSpec{
		{
			Type: "database",
			DatabaseSpec: &providers.DatabaseSpec{
				Name:      "db1",
				Engine:    "postgresql",
				StorageGB: 20,
			},
		},
		{
			Type: "database",
			DatabaseSpec: &providers.DatabaseSpec{
				Name:      "db2",
				Engine:    "postgresql",
				StorageGB: 20,
			},
		},
	}

	input := provisioner.ProvisionInput{
		EnvID:     "partial-test",
		Resources: resources,
		Provider:  mockProvider,
		Credentials: credentials.CredentialConfig{
			Provider: "alicloud",
			Region:   "cn-hangzhou",
		},
	}

	// First provision - all succeed
	result1, err := prov.Provision(ctx, input)
	require.NoError(t, err)
	require.True(t, result1.Success)
	require.Len(t, result1.Resources, 2)

	// Verify all provisioned
	stateResources, err := store.ListResourcesByEnv(ctx, "partial-test")
	require.NoError(t, err)
	require.Len(t, stateResources, 2)
	for _, res := range stateResources {
		assert.Equal(t, "provisioned", res.Status)
	}

	// Phase 2: Add new resource and inject failure
	mockProvider.SetError(true, "non-retryable")
	resourcesWithFailure := []providers.ResourceSpec{
		{
			Type: "database",
			DatabaseSpec: &providers.DatabaseSpec{
				Name:      "db1",
				Engine:    "postgresql",
				StorageGB: 20,
			},
		},
		{
			Type: "database",
			DatabaseSpec: &providers.DatabaseSpec{
				Name:      "db2",
				Engine:    "postgresql",
				StorageGB: 20,
			},
		},
		{
			// This new resource will fail
			Type: "cache",
			CacheSpec: &providers.CacheSpec{
				Name:     "failing-cache",
				Engine:   "redis",
				MemoryMB: 256,
			},
		},
	}

	input2 := provisioner.ProvisionInput{
		EnvID:     "partial-test",
		Resources: resourcesWithFailure,
		Provider:  mockProvider,
		Credentials: credentials.CredentialConfig{
			Provider: "alicloud",
			Region:   "cn-hangzhou",
		},
	}

	// Second provision - new resource fails
	result2, err := prov.Provision(ctx, input2)
	// Should not error at provision level, but result.Success will be false
	require.NoError(t, err)
	require.NotNil(t, result2)
	require.False(t, result2.Success, "Should have partial failure")

	// Verify isolation: original resources remain provisioned, new one failed
	stateResources2, err := store.ListResourcesByEnv(ctx, "partial-test")
	require.NoError(t, err)
	
	for _, res := range stateResources2 {
		switch res.ResourceName {
		case "db1", "db2":
			// Original resources should remain provisioned (isolation)
			assert.Equal(t, "provisioned", res.Status, 
				"Existing resource %s should remain provisioned", res.ResourceName)
		case "failing-cache":
			// New failing resource should be in failed state
			assert.Equal(t, "failed", res.Status,
				"New failing resource should be marked as failed")
		}
	}

	// Reset error for cleanup
	mockProvider.SetError(false, "")
}

// TestPipeline_DestroyAndReprovision validates destroy cycle
func TestPipeline_DestroyAndReprovision(t *testing.T) {
	prov, store, ctx := setupTest(t)
	mockProvider := NewMockCloudProvider("alicloud")

	resources := []providers.ResourceSpec{
		{
			Type: "database",
			DatabaseSpec: &providers.DatabaseSpec{
				Name:      "tempdb",
				Engine:    "postgresql",
				StorageGB: 20,
			},
		},
	}

	input := provisioner.ProvisionInput{
		EnvID:     "temp-env",
		Resources: resources,
		Provider:  mockProvider,
		Credentials: credentials.CredentialConfig{
			Provider: "alicloud",
			Region:   "cn-hangzhou",
		},
	}

	// Initial provision
	result, err := prov.Provision(ctx, input)
	require.NoError(t, err)
	require.True(t, result.Success)

	// Verify exists
	stateResources, _ := store.ListResourcesByEnv(ctx, "temp-env")
	assert.Len(t, stateResources, 1)

	// Destroy
	err = prov.Destroy(ctx, "temp-env")
	require.NoError(t, err)

	// Verify destroyed
	stateResources, _ = store.ListResourcesByEnv(ctx, "temp-env")
	for _, r := range stateResources {
		assert.Equal(t, "destroyed", r.Status)
	}

	// Reprovision should work
	result2, err := prov.Provision(ctx, input)
	require.NoError(t, err)
	require.True(t, result2.Success)
}
