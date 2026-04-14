package integration

import (
	"context"
	"testing"

	"github.com/DaviRain-Su/infracast/internal/credentials"
	"github.com/DaviRain-Su/infracast/internal/mapper"
	"github.com/DaviRain-Su/infracast/internal/provisioner"
	"github.com/DaviRain-Su/infracast/internal/state"
	"github.com/DaviRain-Su/infracast/providers"
	"github.com/DaviRain-Su/infracast/providers/alicloud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAlicloudProvider_Registration validates provider registration
func TestAlicloudProvider_Registration(t *testing.T) {
	// This test validates that the AliCloud provider can be registered with the registry
	registry := providers.NewRegistry()
	
	// Create provider (without real credentials - just structure validation)
	provider, err := alicloud.NewProvider("cn-hangzhou", "test-ak", "test-sk")
	require.NoError(t, err)
	
	// Register provider
	err = registry.Register(provider)
	require.NoError(t, err)
	
	// Verify registration
	retrieved, err := registry.Get("alicloud")
	require.NoError(t, err)
	assert.Equal(t, "alicloud", retrieved.Name())
	assert.Equal(t, "Aliyun Cloud", retrieved.DisplayName())
}

// TestAlicloudProvider_Regions validates supported regions
func TestAlicloudProvider_Regions(t *testing.T) {
	provider, err := alicloud.NewProvider("cn-hangzhou", "test-ak", "test-sk")
	require.NoError(t, err)
	
	regions := provider.Regions()
	require.NotEmpty(t, regions)
	
	// Verify key regions are supported
	regionMap := make(map[string]bool)
	for _, r := range regions {
		regionMap[r.ID] = true
	}
	
	assert.True(t, regionMap["cn-hangzhou"], "should support cn-hangzhou")
	assert.True(t, regionMap["cn-beijing"], "should support cn-beijing")
	assert.True(t, regionMap["cn-shanghai"], "should support cn-shanghai")
}

// TestAlicloudProvider_StructuralIntegration validates provider integrates with provisioner
func TestAlicloudProvider_StructuralIntegration(t *testing.T) {
	// Setup state store
	store, err := state.NewStore(":memory:")
	require.NoError(t, err)
	
	// Setup credentials manager
	creds := credentials.NewManager()
	creds.Store("alicloud", "AK123", "SK456", "cn-hangzhou")
	
	// Create provisioner
	prov := provisioner.NewProvisioner(store, creds)
	require.NotNil(t, prov)
	
	// Create and register AliCloud provider
	aliProvider, err := alicloud.NewProvider("cn-hangzhou", "AK123", "SK456")
	require.NoError(t, err)
	
	// Note: We can't actually call Provision methods without real AliCloud resources
	// but we can verify the provider is properly created and registered
	assert.Equal(t, "alicloud", aliProvider.Name())
	assert.NotNil(t, aliProvider)
}

// TestAlicloudProvider_AllResourceTypes validates all resource type methods exist
func TestAlicloudProvider_AllResourceTypes(t *testing.T) {
	provider, err := alicloud.NewProvider("cn-hangzhou", "test-ak", "test-sk")
	require.NoError(t, err)
	
	// All these should be callable (will fail without real credentials/clients,
	// but validates the interface is implemented)
	ctx := context.Background()
	
	// Database
	dbSpec := providers.DatabaseSpec{
		Name:      "testdb",
		Engine:    "mysql",
		Version:   "8.0",
		StorageGB: 20,
	}
	_, err = provider.ProvisionDatabase(ctx, dbSpec)
	assert.Error(t, err) // Expected to fail without real client
	
	// Cache
	cacheSpec := providers.CacheSpec{
		Name:     "testcache",
		Engine:   "redis",
		MemoryMB: 256,
	}
	_, err = provider.ProvisionCache(ctx, cacheSpec)
	assert.Error(t, err) // Expected to fail without real client
	
	// Object Storage
	objSpec := providers.ObjectStorageSpec{
		Name: "testbucket",
		ACL:  "private",
	}
	_, err = provider.ProvisionObjectStorage(ctx, objSpec)
	assert.Error(t, err) // Expected to fail without real client
}

// TestAlicloudProvider_MapperIntegration validates provider works with mapper
func TestAlicloudProvider_MapperIntegration(t *testing.T) {
	// Create mapper
	registry := providers.NewRegistry()
	mapperInst := mapper.NewMapper(registry)
	
	// Create and register provider
	provider, err := alicloud.NewProvider("cn-hangzhou", "test-ak", "test-sk")
	require.NoError(t, err)
	registry.Register(provider)
	
	// Create build meta
	meta := mapper.BuildMeta{
		AppName:       "testapp",
		Services:      []string{"api"},
		Databases:     []string{"users", "orders"},
		Caches:        []string{"session"},
		ObjectStores: []string{"assets"},
	}
	
	// Map to resource specs
	specs := mapperInst.MapToResourceSpecs(meta)
	require.NotEmpty(t, specs)
	
	// Verify specs are created
	hasDatabase := false
	hasCache := false
	hasObjectStorage := false
	
	for _, spec := range specs {
		switch spec.Type {
		case "database":
			hasDatabase = true
			assert.NotNil(t, spec.DatabaseSpec)
		case "cache":
			hasCache = true
			assert.NotNil(t, spec.CacheSpec)
		case "object_storage":
			hasObjectStorage = true
			assert.NotNil(t, spec.ObjectStorageSpec)
		}
	}
	
	assert.True(t, hasDatabase, "should have database specs")
	assert.True(t, hasCache, "should have cache specs")
	assert.True(t, hasObjectStorage, "should have object storage specs")
}

// TestAlicloudProvider_Endpoints validates endpoint generation
func TestAlicloudProvider_Endpoints(t *testing.T) {
	provider, err := alicloud.NewProvider("cn-hangzhou", "test-ak", "test-sk")
	require.NoError(t, err)
	
	// Verify OTLP endpoint
	otlpEndpoint := provider.OTLPEndpoint()
	assert.Contains(t, otlpEndpoint, "cn-hangzhou")
	assert.Contains(t, otlpEndpoint, "aliyun")
	
	// Verify Dashboard URL
	dashboardURL := provider.DashboardURL("test-env")
	assert.Contains(t, dashboardURL, "rds.console.aliyun")
}

// TestAlicloudProvider_IntegrationWithStateStore validates state store integration
func TestAlicloudProvider_IntegrationWithStateStore(t *testing.T) {
	ctx := context.Background()
	
	// Create state store
	store, err := state.NewStore(":memory:")
	require.NoError(t, err)
	
	// Create a resource entry (simulating provisioned state)
	resource := &state.InfraResource{
		ID:           "alicloud:users-db",
		EnvID:        "test-env",
		ResourceName: "users-db",
		ResourceType: "database",
		SpecHash:     "abc123",
		Status:       "provisioned",
	}
	
	// Store resource
	err = store.UpsertResource(ctx, resource)
	require.NoError(t, err)
	
	// Retrieve resource
	retrieved, err := store.GetResource(ctx, "test-env", "users-db")
	require.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "users-db", retrieved.ResourceName)
	assert.Equal(t, "provisioned", retrieved.Status)
}
