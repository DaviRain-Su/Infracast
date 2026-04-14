package alicloud

import (
	"testing"

	"github.com/DaviRain-Su/infracast/providers"
	"github.com/stretchr/testify/assert"
)

// TestNewProvider validates provider creation
func TestNewProvider(t *testing.T) {
	// Skip if no real credentials
	t.Skip("Skipping provider test - requires real AliCloud credentials")
	
	provider, err := NewProvider("cn-hangzhou", "test-ak", "test-sk")
	assert.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "alicloud", provider.Name())
	assert.Equal(t, "cn-hangzhou", provider.region)
}

// TestProvider_Name validates provider name
func TestProvider_Name(t *testing.T) {
	p := &Provider{region: "cn-hangzhou"}
	assert.Equal(t, "alicloud", p.Name())
}

// TestProvider_DisplayName validates display name
func TestProvider_DisplayName(t *testing.T) {
	p := &Provider{}
	assert.Equal(t, "Aliyun Cloud", p.DisplayName())
}

// TestProvider_Regions validates supported regions
func TestProvider_Regions(t *testing.T) {
	p := &Provider{}
	regions := p.Regions()
	assert.NotEmpty(t, regions)
	
	// Check for key regions
	regionMap := make(map[string]bool)
	for _, r := range regions {
		regionMap[r.ID] = true
	}
	
	assert.True(t, regionMap["cn-hangzhou"], "should support cn-hangzhou")
	assert.True(t, regionMap["cn-beijing"], "should support cn-beijing")
	assert.True(t, regionMap["ap-southeast-1"], "should support ap-southeast-1")
}

// TestProvider_ProvisionDatabase_NotImplemented validates the method exists
func TestProvider_ProvisionDatabase_NotImplemented(t *testing.T) {
	p := &Provider{region: "cn-hangzhou"}
	
	spec := providers.DatabaseSpec{
		Name:      "testdb",
		Engine:    "mysql",
		Version:   "8.0",
		StorageGB: 20,
	}
	
	// Should fail because client is nil (no real credentials)
	_, err := p.ProvisionDatabase(nil, spec)
	assert.Error(t, err)
}

// TestProvider_ProvisionCache_NotImplemented validates placeholder
func TestProvider_ProvisionCache_NotImplemented(t *testing.T) {
	p := &Provider{}
	
	spec := providers.CacheSpec{
		Name:     "testcache",
		Engine:   "redis",
		MemoryMB: 256,
	}
	
	_, err := p.ProvisionCache(nil, spec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not yet implemented")
}

// TestProvider_ProvisionObjectStorage_NotImplemented validates placeholder
func TestProvider_ProvisionObjectStorage_NotImplemented(t *testing.T) {
	p := &Provider{}
	
	spec := providers.ObjectStorageSpec{
		Name: "testbucket",
		ACL:  "private",
	}
	
	_, err := p.ProvisionObjectStorage(nil, spec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not yet implemented")
}

// TestProvider_ProvisionCompute_NotImplemented validates placeholder
func TestProvider_ProvisionCompute_NotImplemented(t *testing.T) {
	p := &Provider{}
	
	spec := providers.ComputeSpec{
		ServiceName: "testsvc",
		Replicas:    1,
		CPU:         "500m",
		Memory:      "256Mi",
	}
	
	_, err := p.ProvisionCompute(nil, spec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not yet implemented")
}

// TestProvider_OTLPEndpoint validates endpoint generation
func TestProvider_OTLPEndpoint(t *testing.T) {
	p := &Provider{region: "cn-hangzhou"}
	endpoint := p.OTLPEndpoint()
	assert.Contains(t, endpoint, "cn-hangzhou")
	assert.Contains(t, endpoint, "aliyun")
}

// TestProvider_DashboardURL validates dashboard URL
func TestProvider_DashboardURL(t *testing.T) {
	p := &Provider{region: "cn-hangzhou"}
	url := p.DashboardURL("test-env")
	assert.Contains(t, url, "rds.console.aliyun")
	assert.Contains(t, url, "cn-hangzhou")
}
