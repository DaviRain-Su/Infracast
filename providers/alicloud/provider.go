// Package alicloud provides Aliyun Cloud provider adapter
package alicloud

import (
	"context"
	"fmt"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/auth/credentials"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/r-kvstore"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/rds"
	"github.com/DaviRain-Su/infracast/providers"
)

// Provider implements CloudProviderInterface for Aliyun Cloud
type Provider struct {
	region          string
	accessKeyID     string
	accessKeySecret string
	rdsClient       *rds.Client
	kvstoreClient   *r_kvstore.Client
}

// NewProvider creates a new AliCloud provider instance
func NewProvider(region, accessKeyID, accessKeySecret string) (*Provider, error) {
	config := sdk.NewConfig()
	cred := credentials.NewAccessKeyCredential(accessKeyID, accessKeySecret)
	
	rdsClient, err := rds.NewClientWithOptions(region, config, cred)
	if err != nil {
		return nil, fmt.Errorf("failed to create RDS client: %w", err)
	}
	
	kvstoreClient, err := r_kvstore.NewClientWithOptions(region, config, cred)
	if err != nil {
		return nil, fmt.Errorf("failed to create KVStore client: %w", err)
	}
	
	return &Provider{
		region:          region,
		accessKeyID:     accessKeyID,
		accessKeySecret: accessKeySecret,
		rdsClient:       rdsClient,
		kvstoreClient:   kvstoreClient,
	}, nil
}

// Name returns the provider identifier
func (p *Provider) Name() string {
	return "alicloud"
}

// DisplayName returns the human-readable provider name
func (p *Provider) DisplayName() string {
	return "Aliyun Cloud"
}

// Regions returns supported regions
func (p *Provider) Regions() []providers.Region {
	return []providers.Region{
		{ID: "cn-hangzhou", Name: "Hangzhou", DisplayName: "华东1 (杭州)"},
		{ID: "cn-beijing", Name: "Beijing", DisplayName: "华北2 (北京)"},
		{ID: "cn-shanghai", Name: "Shanghai", DisplayName: "华东2 (上海)"},
		{ID: "cn-shenzhen", Name: "Shenzhen", DisplayName: "华南1 (深圳)"},
		{ID: "ap-southeast-1", Name: "Singapore", DisplayName: "新加坡"},
	}
}

// ProvisionDatabase creates an RDS database instance
func (p *Provider) ProvisionDatabase(ctx context.Context, spec providers.DatabaseSpec) (*providers.DatabaseOutput, error) {
	if p.rdsClient == nil {
		return nil, fmt.Errorf("RDS client not initialized")
	}
	
	// Map engine to Aliyun engine
	engine := spec.Engine
	if engine == "postgresql" {
		engine = "PostgreSQL"
	} else if engine == "mysql" {
		engine = "MySQL"
	}
	
	// Create RDS instance request
	request := rds.CreateCreateDBInstanceRequest()
	request.RegionId = p.region
	request.Engine = engine
	request.EngineVersion = spec.Version
	request.DBInstanceClass = spec.InstanceClass
	if request.DBInstanceClass == "" {
		// Default instance class for small workloads
		request.DBInstanceClass = "rds.mysql.s1.small"
	}
	request.DBInstanceStorage = requests.NewInteger(spec.StorageGB)
	if spec.StorageGB == 0 {
		request.DBInstanceStorage = requests.NewInteger(20)
	}
	request.DBInstanceNetType = "Intranet"
	request.PayType = "Postpaid" // Pay-as-you-go
	
	// Set High Availability category
	if spec.HighAvail {
		request.Category = "HighAvailability"
	} else {
		request.Category = "Basic"
	}
	
	// Check for existing instance (idempotency)
	describeReq := rds.CreateDescribeDBInstancesRequest()
	describeReq.RegionId = p.region
	describeReq.DBInstanceId = spec.Name
	if describeResp, err := p.rdsClient.DescribeDBInstances(describeReq); err == nil && len(describeResp.Items.DBInstance) > 0 {
		// Instance exists, return current info
		existing := describeResp.Items.DBInstance[0]
		return &providers.DatabaseOutput{
			ResourceID: existing.DBInstanceId,
			Endpoint:   existing.ConnectionString,
			Port:       getPortForEngine(spec.Engine),
			Username:   "root",
			Password:   "",
		}, nil
	}
	
	// Execute creation
	response, err := p.rdsClient.CreateDBInstance(request)
	if err != nil {
		return nil, fmt.Errorf("failed to create RDS instance: %w", err)
	}
	
	return &providers.DatabaseOutput{
		ResourceID: response.DBInstanceId,
		Endpoint:   "", // TODO: Poll DescribeDBInstanceAttribute to get connection string
		Port:       getPortForEngine(spec.Engine),
		Username:   "root",
		Password:   "", // TODO: Set via separate call or accept from spec
	}, nil
}

// getPortForEngine returns the default port for the database engine
func getPortForEngine(engine string) int {
	switch engine {
	case "postgresql", "PostgreSQL":
		return 5432
	case "mysql", "MySQL":
		return 3306
	default:
		return 3306
	}
}

// ProvisionCache creates a Redis cache instance
func (p *Provider) ProvisionCache(ctx context.Context, spec providers.CacheSpec) (*providers.CacheOutput, error) {
	if p.kvstoreClient == nil {
		return nil, fmt.Errorf("KVStore client not initialized")
	}
	
	// Determine instance class based on memory
	instanceClass := "redis.master.small.default"
	if spec.MemoryMB >= 4096 {
		instanceClass = "redis.master.mid.default"
	}
	
	request := r_kvstore.CreateCreateInstanceRequest()
	request.RegionId = p.region
	request.InstanceClass = instanceClass
	request.InstanceType = "Redis"
	request.EngineVersion = spec.Version
	if request.EngineVersion == "" {
		request.EngineVersion = "5.0"
	}
	
	// Execute creation
	response, err := p.kvstoreClient.CreateInstance(request)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis instance: %w", err)
	}
	
	return &providers.CacheOutput{
		ResourceID: response.InstanceId,
		Endpoint:   "", // Will be populated after instance is ready
		Port:       6379,
		Password:   "", // Should be retrieved after creation
	}, nil
}

// ProvisionObjectStorage creates an OSS bucket (placeholder)
func (p *Provider) ProvisionObjectStorage(ctx context.Context, spec providers.ObjectStorageSpec) (*providers.ObjectStorageOutput, error) {
	return nil, fmt.Errorf("object storage provisioning not yet implemented")
}

// ProvisionCompute creates a compute resource (placeholder)
func (p *Provider) ProvisionCompute(ctx context.Context, spec providers.ComputeSpec) (*providers.ComputeOutput, error) {
	return nil, fmt.Errorf("compute provisioning not yet implemented")
}

// Plan generates a plan for the resources
func (p *Provider) Plan(ctx context.Context, specs []providers.ResourceSpec) (*providers.PlanResult, error) {
	return nil, fmt.Errorf("plan not implemented")
}

// Apply applies the plan
func (p *Provider) Apply(ctx context.Context, plan *providers.PlanResult) (*providers.ApplyResult, error) {
	return nil, fmt.Errorf("apply not implemented")
}

// Destroy destroys a resource
func (p *Provider) Destroy(ctx context.Context, resourceID string) error {
	return fmt.Errorf("destroy not implemented")
}

// OTLPEndpoint returns the OpenTelemetry endpoint
func (p *Provider) OTLPEndpoint() string {
	return fmt.Sprintf("https://tracing-%s.aliyuncs.com", p.region)
}

// DashboardURL returns the cloud console URL for the environment
func (p *Provider) DashboardURL(envID string) string {
	return fmt.Sprintf("https://rds.console.aliyun.com/%s", p.region)
}
