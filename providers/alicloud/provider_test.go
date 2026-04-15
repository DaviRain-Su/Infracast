package alicloud

import (
	"context"
	"testing"

	"github.com/DaviRain-Su/infracast/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// TestProvider_ProvisionCache validates cache provisioning
func TestProvider_ProvisionCache(t *testing.T) {
	p := &Provider{region: "cn-hangzhou"}

	spec := providers.CacheSpec{
		Name:     "testcache",
		Engine:   "redis",
		MemoryMB: 256,
	}

	// Should fail because client is nil
	_, err := p.ProvisionCache(nil, spec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

// TestProvider_ProvisionObjectStorage validates object storage provisioning
func TestProvider_ProvisionObjectStorage(t *testing.T) {
	p := &Provider{region: "cn-hangzhou"}

	spec := providers.ObjectStorageSpec{
		Name: "testbucket",
		ACL:  "private",
	}

	// Should fail because client is nil
	_, err := p.ProvisionObjectStorage(nil, spec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
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

func TestProvider_ensureNetwork_Cached(t *testing.T) {
	p := &Provider{
		region: "cn-hangzhou",
		networkCache: &networkState{
			ready:     true,
			vpcID:     "vpc-cached-cn-hangzhou",
			vswitchID: "vsw-cached-cn-hangzhou",
		},
	}
	vpcID1, vswID1, err := p.ensureNetwork(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "vpc-cached-cn-hangzhou", vpcID1)
	assert.Equal(t, "vsw-cached-cn-hangzhou", vswID1)

	vpcID2, vswID2, err := p.ensureNetwork(context.Background())
	require.NoError(t, err)
	assert.Equal(t, vpcID1, vpcID2)
	assert.Equal(t, vswID1, vswID2)
}

func TestProvider_ensureNetwork_MissingVpcClient(t *testing.T) {
	p := &Provider{region: "cn-hangzhou"}

	vpcID, vswID, err := p.ensureNetwork(context.Background())
	require.Error(t, err)
	assert.Equal(t, "VPC client not initialized", err.Error())
	assert.Equal(t, "", vpcID)
	assert.Equal(t, "", vswID)
}

func TestProvider_ensureNetwork_EmptyRegion(t *testing.T) {
	p := &Provider{}

	vpcID, vswID, err := p.ensureNetwork(context.Background())
	require.Error(t, err)
	assert.Equal(t, "", vpcID)
	assert.Equal(t, "", vswID)
}

func TestAlicloudInitRegistersProvider(t *testing.T) {
	p, err := providers.Get("alicloud")
	require.NoError(t, err)
	assert.Equal(t, "alicloud", p.Name())
}
