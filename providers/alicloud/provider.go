// Package alicloud provides Aliyun Cloud provider adapter
package alicloud

import (
	"context"
	"fmt"
	"sync"

	"github.com/DaviRain-Su/infracast/providers"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/auth/credentials"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/r-kvstore"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/rds"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/vpc"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// Provider implements CloudProviderInterface for Aliyun Cloud
type Provider struct {
	region          string
	accessKeyID     string
	accessKeySecret string
	rdsClient       *rds.Client
	kvstoreClient   *r_kvstore.Client
	vpcClient       *vpc.Client
	ossClient       *oss.Client
	networkCache    *networkState
	networkMu       sync.RWMutex
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

	vpcClient, err := vpc.NewClientWithOptions(region, config, cred)
	if err != nil {
		return nil, fmt.Errorf("failed to create VPC client: %w", err)
	}

	// OSS client uses different endpoint format
	endpoint := fmt.Sprintf("oss-%s.aliyuncs.com", region)
	ossClient, err := oss.New(endpoint, accessKeyID, accessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("failed to create OSS client: %w", err)
	}

	return &Provider{
		region:          region,
		accessKeyID:     accessKeyID,
		accessKeySecret: accessKeySecret,
		rdsClient:       rdsClient,
		kvstoreClient:   kvstoreClient,
		vpcClient:       vpcClient,
		ossClient:       ossClient,
		networkCache:    &networkState{},
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

	// Ensure default VPC/VSwitch are available for network-bound resources.
	vpcID, vswID, err := p.ensureNetwork(ctx)
	if err != nil {
		return nil, err
	}

	// Map engine to Aliyun engine
	engine := spec.Engine
	if engine == "postgresql" {
		engine = "PostgreSQL"
	} else if engine == "mysql" {
		engine = "MySQL"
	}

	request := rds.CreateCreateDBInstanceRequest()
	request.VPCId = vpcID
	request.VSwitchId = vswID
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
	if spec.HighAvail != nil && *spec.HighAvail {
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

	// Ensure default VPC/VSwitch are available for network-bound resources.
	vpcID, vswID, err := p.ensureNetwork(ctx)
	if err != nil {
		return nil, err
	}

	// Check for existing instance (idempotency)
	describeReq := r_kvstore.CreateDescribeInstancesRequest()
	describeReq.RegionId = p.region
	if describeResp, err := p.kvstoreClient.DescribeInstances(describeReq); err == nil {
		for _, inst := range describeResp.Instances.KVStoreInstance {
			if inst.InstanceName == spec.Name {
				// Instance exists, return current info
				return &providers.CacheOutput{
					ResourceID: inst.InstanceId,
					Endpoint:   inst.ConnectionDomain,
					Port:       6379,
					Password:   "",
				}, nil
			}
		}
	}

	// Determine instance class based on memory
	instanceClass := "redis.master.small.default"
	if spec.MemoryMB >= 4096 {
		instanceClass = "redis.master.mid.default"
	}

	request := r_kvstore.CreateCreateInstanceRequest()
	request.VpcId = vpcID
	request.VSwitchId = vswID
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
		Endpoint:   "", // TODO: Poll DescribeInstances to get connection domain
		Port:       6379,
		Password:   "", // TODO: Retrieve after creation
	}, nil
}

// ProvisionObjectStorage creates an OSS bucket
func (p *Provider) ProvisionObjectStorage(ctx context.Context, spec providers.ObjectStorageSpec) (*providers.ObjectStorageOutput, error) {
	if p.ossClient == nil {
		return nil, fmt.Errorf("OSS client not initialized")
	}

	if _, _, err := p.ensureNetwork(ctx); err != nil {
		return nil, err
	}

	// Check if bucket exists (idempotency)
	exists, err := p.ossClient.IsBucketExist(spec.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if exists {
		// Bucket exists, return current info
		return &providers.ObjectStorageOutput{
			ResourceID: spec.Name,
			BucketName: spec.Name,
			Endpoint:   fmt.Sprintf("%s.oss-%s.aliyuncs.com", spec.Name, p.region),
			Region:     p.region,
		}, nil
	}

	// Create bucket
	err = p.ossClient.CreateBucket(spec.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create OSS bucket: %w", err)
	}

	return &providers.ObjectStorageOutput{
		ResourceID: spec.Name,
		BucketName: spec.Name,
		Endpoint:   fmt.Sprintf("%s.oss-%s.aliyuncs.com", spec.Name, p.region),
		Region:     p.region,
	}, nil
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
