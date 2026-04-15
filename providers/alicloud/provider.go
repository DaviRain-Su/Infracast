// Package alicloud provides Aliyun Cloud provider adapter
package alicloud

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"sync"
	"time"

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
	if request.Engine == "PostgreSQL" && request.EngineVersion != "" && !strings.Contains(request.EngineVersion, ".") {
		request.EngineVersion = request.EngineVersion + ".0"
	}
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
	request.SecurityIPList = "127.0.0.1"

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

	// Execute creation with instance class fallback
	response, err := p.createDBInstanceWithFallback(request)
	if err != nil {
		return nil, fmt.Errorf("failed to create RDS instance: %w", err)
	}

	// Poll for instance to be ready and get endpoint
	instanceID := response.DBInstanceId
	endpoint, err := p.waitForDBInstanceReady(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for RDS instance: %w", err)
	}

	// Generate and set initial password
	password := generateRandomPassword()
	if err := p.setDBPassword(ctx, instanceID, "root", password); err != nil {
		return nil, fmt.Errorf("failed to set DB password: %w", err)
	}

	return &providers.DatabaseOutput{
		ResourceID: instanceID,
		Endpoint:   endpoint,
		Port:       getPortForEngine(spec.Engine),
		Username:   "root",
		Password:   password,
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

	// Poll for instance to be ready and get endpoint
	instanceID := response.InstanceId
	endpoint, err := p.waitForCacheInstanceReady(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for Redis instance: %w", err)
	}

	// Generate and set initial password
	password := generateRandomPassword()
	if err := p.setCachePassword(ctx, instanceID, password); err != nil {
		return nil, fmt.Errorf("failed to set Redis password: %w", err)
	}

	return &providers.CacheOutput{
		ResourceID: instanceID,
		Endpoint:   endpoint,
		Port:       6379,
		Password:   password,
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

// waitForDBInstanceReady polls until the RDS instance is ready and returns the endpoint
func (p *Provider) waitForDBInstanceReady(ctx context.Context, instanceID string) (string, error) {
	// Poll for up to 10 minutes (RDS creation takes time)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	timeout := time.After(10 * time.Minute)
	
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for RDS instance %s", instanceID)
		case <-ticker.C:
			req := rds.CreateDescribeDBInstanceAttributeRequest()
			req.DBInstanceId = instanceID
			
			resp, err := p.rdsClient.DescribeDBInstanceAttribute(req)
			if err != nil {
				continue // Retry on error
			}
			
			if len(resp.Items.DBInstanceAttribute) == 0 {
				continue
			}
			
			attr := resp.Items.DBInstanceAttribute[0]
			// Check if instance is running
			if attr.DBInstanceStatus == "Running" {
				// Return the internal connection string (intranet endpoint)
				if attr.ConnectionString != "" {
					return attr.ConnectionString, nil
				}
				// Fallback to instance ID as endpoint if connection string not available
				return attr.DBInstanceId + ".mysql.rds.aliyuncs.com", nil
			}
		}
	}
}

// setDBPassword sets the initial password for the database root account
func (p *Provider) setDBPassword(ctx context.Context, instanceID, username, password string) error {
	// For MySQL/PostgreSQL on Aliyun, we need to reset the account password
	// Try to reset the account password (works for existing accounts like 'root')
	req := rds.CreateResetAccountPasswordRequest()
	req.DBInstanceId = instanceID
	req.AccountName = username
	req.AccountPassword = password
	
	_, err := p.rdsClient.ResetAccountPassword(req)
	if err != nil {
		// If reset fails, try creating a new account
		createReq := rds.CreateCreateAccountRequest()
		createReq.DBInstanceId = instanceID
		createReq.AccountName = username
		createReq.AccountPassword = password
		createReq.AccountType = "Super"
		
		_, err = p.rdsClient.CreateAccount(createReq)
		if err != nil {
			return fmt.Errorf("failed to set password: %w", err)
		}
	}
	
	return nil
}

func (p *Provider) createDBInstanceWithFallback(request *rds.CreateDBInstanceRequest) (*rds.CreateDBInstanceResponse, error) {
	for _, class := range candidateDBInstanceClasses(request.Engine, request.DBInstanceClass) {
		request.DBInstanceClass = class
		resp, err := p.rdsClient.CreateDBInstance(request)
		if err == nil {
			return resp, nil
		}
		if !strings.Contains(err.Error(), "InvalidDBInstanceClass.NotFound") {
			return nil, err
		}
	}
	return nil, fmt.Errorf("no available DB instance class found for engine=%s in region=%s", request.Engine, p.region)
}

func candidateDBInstanceClasses(engine, preferred string) []string {
	classes := make([]string, 0, 8)
	seen := map[string]struct{}{}
	add := func(v string) {
		if v == "" {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		classes = append(classes, v)
	}

	add(preferred)
	switch engine {
	case "PostgreSQL":
		add("pg.n2.1c.1m")
		add("pg.n2.1c.2m")
		add("pg.n2.2c.4m")
		add("pg.n2.2c.8m")
		add("rds.pg.s1.small")
	default:
		add("rds.mysql.s1.small")
		add("mysql.n2.small.1")
		add("mysql.n2.medium.1")
	}
	return classes
}

// generateRandomPassword generates a cryptographically secure random password
func generateRandomPassword() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	const length = 16
	
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return string(b)
}

// waitForCacheInstanceReady polls until the Redis instance is ready and returns the endpoint
func (p *Provider) waitForCacheInstanceReady(ctx context.Context, instanceID string) (string, error) {
	// Poll for up to 10 minutes (Redis creation takes time)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	timeout := time.After(10 * time.Minute)
	
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for Redis instance %s", instanceID)
		case <-ticker.C:
			req := r_kvstore.CreateDescribeInstancesRequest()
			req.RegionId = p.region
			req.InstanceIds = instanceID
			
			resp, err := p.kvstoreClient.DescribeInstances(req)
			if err != nil {
				continue // Retry on error
			}
			
			// Check if instance is available
			for _, inst := range resp.Instances.KVStoreInstance {
				if inst.InstanceId == instanceID {
					if inst.InstanceStatus == "Normal" {
						if inst.ConnectionDomain != "" {
							return inst.ConnectionDomain, nil
						}
						return inst.PrivateIp, nil
					}
				}
			}
		}
	}
}

// setCachePassword sets the password for the Redis instance
func (p *Provider) setCachePassword(ctx context.Context, instanceID, password string) error {
	// Try to reset the account password for the default account
	req := r_kvstore.CreateResetAccountPasswordRequest()
	req.InstanceId = instanceID
	req.AccountName = "default" // Default account name for Redis
	req.AccountPassword = password
	
	_, err := p.kvstoreClient.ResetAccountPassword(req)
	if err != nil {
		// If reset fails, the password might be auto-generated or set during creation
		// This is a simplified implementation
		return fmt.Errorf("failed to set Redis password: %w", err)
	}
	
	return nil
}
