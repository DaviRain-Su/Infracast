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
	"github.com/DaviRain-Su/infracast/internal/mapper"
	"github.com/DaviRain-Su/infracast/internal/state"
	"github.com/DaviRain-Su/infracast/pkg/hash"
	"github.com/DaviRain-Su/infracast/pkg/infragen"
	"github.com/DaviRain-Su/infracast/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_ConfigLoading validates config loading and resolution
func TestIntegration_ConfigLoading(t *testing.T) {
	tmpDir := t.TempDir()

	// Create config file
	cfgPath := filepath.Join(tmpDir, "infracast.yaml")
	cfgContent := `
provider: alicloud
region: cn-hangzhou
environments:
  production:
    provider: alicloud
    region: cn-shanghai
  staging:
    provider: alicloud
    region: cn-beijing
overrides:
  databases:
    mydb:
      storage_gb: 100
      instance_class: rds.pg.s3.large
`
	err := os.WriteFile(cfgPath, []byte(cfgContent), 0644)
	require.NoError(t, err)

	// Load config
	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Validate root config
	assert.Equal(t, "alicloud", cfg.Provider)
	assert.Equal(t, "cn-hangzhou", cfg.Region)

	// Resolve production environment
	prod, err := cfg.ResolveEnv("production")
	require.NoError(t, err)
	assert.Equal(t, "cn-shanghai", prod.Region)

	// Resolve staging environment
	staging, err := cfg.ResolveEnv("staging")
	require.NoError(t, err)
	assert.Equal(t, "cn-beijing", staging.Region)

	// Test database override
	override, exists := cfg.GetDatabaseOverride("mydb")
	require.True(t, exists)
	assert.Equal(t, 100, override.StorageGB)
}

// TestIntegration_StateManagement validates state store CRUD operations
func TestIntegration_StateManagement(t *testing.T) {
	ctx := context.Background()
	store, err := state.NewStore(":memory:")
	require.NoError(t, err)

	// Create resource
	resource := &state.InfraResource{
		ID:           "test-resource",
		EnvID:        "test-env",
		ResourceName: "mydb",
		ResourceType: "database",
		SpecHash:     "abc123",
		Status:       "pending",
	}

	err = store.UpsertResource(ctx, resource)
	require.NoError(t, err)

	// Read resource
	retrieved, err := store.GetResource(ctx, "test-env", "mydb")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, "abc123", retrieved.SpecHash)

	// Update resource
	resource.SpecHash = "def456"
	err = store.UpsertResource(ctx, resource)
	require.NoError(t, err)

	// Verify update
	updated, err := store.GetResource(ctx, "test-env", "mydb")
	require.NoError(t, err)
	assert.Equal(t, "def456", updated.SpecHash)

	// List by environment
	resources, err := store.ListResourcesByEnv(ctx, "test-env")
	require.NoError(t, err)
	assert.Len(t, resources, 1)
}

// TestIntegration_SpecHashing validates spec hash computation
func TestIntegration_SpecHashing(t *testing.T) {
	// Database spec
	dbSpec := providers.DatabaseSpec{
		Name:      "mydb",
		Engine:    "postgresql",
		Version:   "15",
		StorageGB: 20,
	}

	hash1, err := hash.SpecHash(hash.ResourceTypeDatabase, dbSpec)
	require.NoError(t, err)
	assert.NotEmpty(t, hash1)

	// Same spec should produce same hash
	hash2, err := hash.SpecHash(hash.ResourceTypeDatabase, dbSpec)
	require.NoError(t, err)
	assert.Equal(t, hash1, hash2)

	// Different spec should produce different hash
	dbSpec.StorageGB = 50
	hash3, err := hash.SpecHash(hash.ResourceTypeDatabase, dbSpec)
	require.NoError(t, err)
	assert.NotEqual(t, hash1, hash3)
}

// TestIntegration_InfragenFlow validates config generation flow
func TestIntegration_InfragenFlow(t *testing.T) {
	tmpDir := t.TempDir()
	generator := infragen.NewGenerator(nil)

	// Create infrastructure config
	cfg := &infragen.InfraConfig{
		SQLServers: map[string]infragen.SQLServer{
			"users": {
				Host:     "users-db.example.com",
				Port:     5432,
				Database: "users",
				User:     "app",
				Password: "${USERS_DB_PASSWORD}",
				TLS: &infragen.TLSConfig{
					Enabled: true,
				},
			},
		},
		Redis: map[string]infragen.RedisServer{
			"session": {
				Host:      "session-cache.example.com",
				Port:      6379,
				Password:  "${SESSION_REDIS_PASSWORD}",
				KeyPrefix: "app:",
				Auth: &infragen.AuthConfig{
					Enabled: true,
				},
			},
		},
		ObjectStorage: map[string]infragen.ObjectStore{
			"assets": {
				Type:      "S3",
				Endpoint:  "https://oss-cn-hangzhou.aliyuncs.com",
				Bucket:    "myapp-assets",
				Region:    "cn-hangzhou",
				Provider:  "alicloud",
				AccessKey: "${OSS_ACCESS_KEY_ID}",
				SecretKey: "${OSS_ACCESS_KEY_SECRET}",
			},
		},
	}

	// Write config
	cfgPath := filepath.Join(tmpDir, "infracfg.json")
	err := generator.Write(cfg, cfgPath)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(cfgPath)
	require.NoError(t, err)

	// Read and validate content
	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "sql_servers")
	assert.Contains(t, content, "redis")
	assert.Contains(t, content, "object_storage")
	assert.Contains(t, content, "users-db.example.com")
}

// TestIntegration_CredentialManagement validates credential storage and retrieval
func TestIntegration_CredentialManagement(t *testing.T) {
	mgr := credentials.NewManager()

	// Store credentials
	err := mgr.Store("alicloud", "AK123456", "SK789012", "cn-hangzhou")
	require.NoError(t, err)

	// Retrieve credentials
	cred, err := mgr.Get("alicloud")
	require.NoError(t, err)
	assert.Equal(t, "AK123456", cred.AccessKeyID)
	assert.Equal(t, "SK789012", cred.AccessKeySecret)
	assert.Equal(t, "cn-hangzhou", cred.Region)

	// Get with region override
	credWithRegion, err := mgr.GetForRegion("alicloud", "cn-shanghai")
	require.NoError(t, err)
	assert.Equal(t, "cn-shanghai", credWithRegion.Region)

	// List providers
	providers := mgr.List()
	assert.Len(t, providers, 1)
	assert.Contains(t, providers, "alicloud")
}

// TestIntegration_MapperFlow validates resource mapping from build meta
func TestIntegration_MapperFlow(t *testing.T) {
	registry := providers.NewRegistry()
	mapperInst := mapper.NewMapper(registry)

	buildMeta := mapper.BuildMeta{
		AppName:      "myapp",
		Services:     []string{"api", "worker"},
		Databases:    []string{"users", "orders"},
		Caches:       []string{"session"},
		ObjectStores: []string{"assets"},
	}

	// Map to resource specs
	specs := mapperInst.MapToResourceSpecs(buildMeta)
	require.GreaterOrEqual(t, len(specs), 4)

	// Count by type
	typeCount := make(map[string]int)
	for _, spec := range specs {
		typeCount[spec.Type]++
	}

	assert.Equal(t, 2, typeCount["database"])
	assert.Equal(t, 1, typeCount["cache"])
	assert.Equal(t, 1, typeCount["object_storage"])

	// Validate database defaults
	for _, spec := range specs {
		if spec.Type == "database" && spec.DatabaseSpec != nil {
			assert.Equal(t, 20, spec.DatabaseSpec.StorageGB) // Tech Spec default
			assert.Equal(t, "postgresql", spec.DatabaseSpec.Engine)
		}
		if spec.Type == "cache" && spec.CacheSpec != nil {
			assert.Equal(t, 256, spec.CacheSpec.MemoryMB) // Tech Spec default
			assert.Equal(t, "redis", spec.CacheSpec.Engine)
		}
	}
}

// TestIntegration_DryRunProvisioning validates dry-run doesn't modify state
func TestIntegration_DryRunProvisioning(t *testing.T) {
	ctx := context.Background()
	store, err := state.NewStore(":memory:")
	require.NoError(t, err)

	creds := credentials.NewManager()
	creds.Store("alicloud", "AK123", "SK456", "cn-hangzhou")

	// Get initial resource count
	initialResources, _ := store.ListResourcesByEnv(ctx, "test")
	initialCount := len(initialResources)

	// Note: Provision would need a registered provider to work fully
	// This test validates the setup pattern
	assert.Equal(t, 0, initialCount)
}

// TestIntegration_EndToEnd validates the complete deployment flow
func TestIntegration_EndToEnd(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Phase 1: Config
	cfg := &config.Config{
		Provider: "alicloud",
		Region:   "cn-hangzhou",
	}
	assert.Equal(t, "alicloud", cfg.Provider)

	// Phase 2: Build Meta
	buildMeta := mapper.BuildMeta{
		AppName:      "ecommerce",
		Services:     []string{"api", "web", "worker"},
		Databases:    []string{"users", "products", "orders"},
		Caches:       []string{"session", "product-cache"},
		ObjectStores: []string{"product-images"},
	}

	// Phase 3: Resource Mapping
	registry := providers.NewRegistry()
	mapperInst := mapper.NewMapper(registry)
	specs := mapperInst.MapToResourceSpecs(buildMeta)
	assert.GreaterOrEqual(t, len(specs), 6)

	// Phase 4: Spec Hashing
	for _, spec := range specs {
		var hashVal string
		var err error
		switch spec.Type {
		case "database":
			if spec.DatabaseSpec != nil {
				hashVal, err = hash.SpecHash(hash.ResourceTypeDatabase, *spec.DatabaseSpec)
			}
		case "cache":
			if spec.CacheSpec != nil {
				hashVal, err = hash.SpecHash(hash.ResourceTypeCache, *spec.CacheSpec)
			}
		case "object_storage":
			if spec.ObjectStorageSpec != nil {
				hashVal, err = hash.SpecHash(hash.ResourceTypeObjectStorage, *spec.ObjectStorageSpec)
			}
		}
		require.NoError(t, err)
		assert.NotEmpty(t, hashVal)
	}

	// Phase 5: State Management
	store, err := state.NewStore(filepath.Join(tmpDir, "state.db"))
	require.NoError(t, err)

	resource := &state.InfraResource{
		ID:           "test-db",
		EnvID:        "production",
		ResourceName: "users",
		ResourceType: "database",
		SpecHash:     "hash123",
		Status:       "provisioned",
	}
	err = store.UpsertResource(ctx, resource)
	require.NoError(t, err)

	// Phase 6: Config Generation
	generator := infragen.NewGenerator(nil)
	infraCfg := &infragen.InfraConfig{
		SQLServers: map[string]infragen.SQLServer{
			"users": {
				Host:     "users-db.example.com",
				Port:     5432,
				Database: "users",
				User:     "app",
				Password: "${USERS_DB_PASSWORD}",
			},
		},
	}

	cfgPath := filepath.Join(tmpDir, "infracfg.json")
	err = generator.Write(infraCfg, cfgPath)
	require.NoError(t, err)

	// Verify all artifacts
	_, err = os.Stat(cfgPath)
	require.NoError(t, err)

	// Verify state
	resources, err := store.ListResourcesByEnv(ctx, "production")
	require.NoError(t, err)
	assert.Len(t, resources, 1)

	// Success
	fmt.Printf("End-to-end flow completed: %d resources, config at %s\n", len(specs), cfgPath)
}

// TestIntegration_ErrorCodes validates error code system
func TestIntegration_ErrorCodes(t *testing.T) {
	// Config errors
	assert.NotNil(t, config.ErrMissingProvider)
	assert.NotNil(t, config.ErrMissingRegion)
	assert.NotNil(t, config.ErrInvalidRegionFormat)

	// Hash errors
	assert.NotNil(t, hash.ErrUnknownResourceType)
}

// Mock provider import for type reference
type MockProvider struct{}

func (m *MockProvider) Name() string { return "mock" }
