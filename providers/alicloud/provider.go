// Package alicloud provides Aliyun Cloud provider adapter
package alicloud

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/DaviRain-Su/infracast/internal/state"
	"github.com/DaviRain-Su/infracast/providers"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/auth/credentials"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/r-kvstore"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/rds"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/vpc"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// Polling timeouts and intervals for resource readiness checks.
const (
	// ProvisionPollTimeout is the maximum time to wait for a resource to become ready.
	ProvisionPollTimeout = 10 * time.Minute
	// ProvisionPollInterval is the interval between readiness checks.
	ProvisionPollInterval = 30 * time.Second
	// VPCPollTimeout is the maximum time to wait for a VPC to become available.
	VPCPollTimeout = 2 * time.Minute
	// VPCPollInterval is the interval between VPC readiness checks.
	VPCPollInterval = 5 * time.Second
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
	stateStore      StateStore // Optional state store for network persistence
	envID           string     // Environment ID for state persistence
}

// StateStore defines the interface for network state persistence
type StateStore interface {
	GetNetworkResource(ctx context.Context, envID string, resourceType string) (*state.InfraResource, error)
	UpsertResource(ctx context.Context, resource *state.InfraResource) error
}

// SetStateStore sets the state store for network persistence
func (p *Provider) SetStateStore(store StateStore) {
	p.stateStore = store
}

// SetEnvironment sets the environment ID for state persistence
func (p *Provider) SetEnvironment(envID string) {
	p.envID = envID
}

// NewProvider creates a new AliCloud provider instance
func NewProvider(region, accessKeyID, accessKeySecret string) (*Provider, error) {
	config := sdk.NewConfig().
		WithTimeout(60 * time.Second).
		WithScheme("HTTPS")
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
	// Uses p.envID for state persistence (set via SetEnvironment)
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
	request.SecurityIPList = p.resolveRdsSecurityIPList(vswID)

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
		username := defaultDBUsername(spec.Engine)
		return &providers.DatabaseOutput{
			ResourceID: existing.DBInstanceId,
			Endpoint:   existing.ConnectionString,
			Port:       getPortForEngine(spec.Engine),
			Username:   username,
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
	password, err := generateRandomPassword()
	if err != nil {
		return nil, fmt.Errorf("EPROV010: failed to generate password: %w", err)
	}
	username, err := p.setDBPassword(ctx, instanceID, defaultDBUsername(spec.Engine), password)
	if err != nil {
		return nil, fmt.Errorf("failed to set DB password: %w", err)
	}

	return &providers.DatabaseOutput{
		ResourceID: instanceID,
		Endpoint:   endpoint,
		Port:       getPortForEngine(spec.Engine),
		Username:   username,
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

func defaultDBUsername(engine string) string {
	switch strings.ToLower(engine) {
	case "postgresql", "postgres":
		return "postgres"
	default:
		return "root"
	}
}

func (p *Provider) resolveRdsSecurityIPList(vswitchID string) string {
	if configured := strings.TrimSpace(os.Getenv("ALICLOUD_RDS_SECURITY_IP_LIST")); configured != "" {
		return configured
	}

	cidr, err := p.vswitchCIDR(vswitchID)
	if err == nil && cidr != "" {
		return cidr
	}

	// Private-network fallback only. Never widen to 0.0.0.0/0 by default.
	return networkVSwitchCidrBlock
}

func (p *Provider) vswitchCIDR(vswitchID string) (string, error) {
	if p.vpcClient == nil {
		return "", fmt.Errorf("VPC client not initialized")
	}

	req := vpc.CreateDescribeVSwitchesRequest()
	req.RegionId = p.region
	req.VSwitchId = vswitchID

	resp, err := p.vpcClient.DescribeVSwitches(req)
	if err != nil {
		return "", err
	}
	if resp == nil || len(resp.VSwitches.VSwitch) == 0 {
		return "", fmt.Errorf("vSwitch %s not found", vswitchID)
	}

	return strings.TrimSpace(resp.VSwitches.VSwitch[0].CidrBlock), nil
}

// ProvisionCache creates a Redis cache instance
func (p *Provider) ProvisionCache(ctx context.Context, spec providers.CacheSpec) (*providers.CacheOutput, error) {
	if p.kvstoreClient == nil {
		return nil, fmt.Errorf("KVStore client not initialized")
	}

	// Ensure default VPC/VSwitch are available for network-bound resources.
	// Uses p.envID for state persistence (set via SetEnvironment)
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

	// Try Redis creation with instance-class fallback + zone fallback.
	var response *r_kvstore.CreateInstanceResponse
	var createErr error
	for _, instanceClass := range candidateRedisInstanceClasses(spec.MemoryMB) {
		response, createErr = p.createRedisWithZoneFallback(vpcID, vswID, instanceClass, spec)
		if createErr == nil {
			break
		}
		if !isRedisCreateRetryable(createErr) {
			return nil, fmt.Errorf("failed to create Redis instance: %w", createErr)
		}
	}
	if createErr != nil {
		return nil, fmt.Errorf("failed to create Redis instance: %w", createErr)
	}

	// Poll for instance to be ready and get endpoint
	instanceID := response.InstanceId
	endpoint, err := p.waitForCacheInstanceReady(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for Redis instance: %w", err)
	}

	// Generate and set initial password
	password, err := generateRandomPassword()
	if err != nil {
		return nil, fmt.Errorf("EPROV010: failed to generate password: %w", err)
	}
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

	// Ensure default VPC/VSwitch are available for network-bound resources.
	// Uses p.envID for state persistence (set via SetEnvironment)
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

// Note: Destroy method is implemented in destroy.go

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
	ticker := time.NewTicker(ProvisionPollInterval)
	defer ticker.Stop()

	timeout := time.After(ProvisionPollTimeout)

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
func (p *Provider) setDBPassword(ctx context.Context, instanceID, username, password string) (string, error) {
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
			// Keyword/reserved account names may be forbidden for certain engines.
			if strings.Contains(err.Error(), "InvalidAccountName.Forbid") {
				fallback := "infracast_admin"
				createReq.AccountName = fallback
				if _, fallbackErr := p.rdsClient.CreateAccount(createReq); fallbackErr == nil {
					return fallback, nil
				} else {
					return "", fmt.Errorf("failed to set password: %w", fallbackErr)
				}
			}
			return "", fmt.Errorf("failed to set password: %w", err)
		}
	}

	return username, nil
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

func candidateRedisInstanceClasses(memoryMB int) []string {
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

	switch {
	case memoryMB >= 16384:
		add("redis.master.large.default")
	case memoryMB >= 4096:
		add("redis.master.mid.default")
	default:
		add("redis.master.small.default")
	}

	// Regional fallbacks
	add("redis.master.micro.default")
	add("redis.master.small.default")
	add("redis.master.mid.default")
	add("redis.master.large.default")
	return classes
}

func isRedisCreateRetryable(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "InvalidvSwitchId") ||
		strings.Contains(s, "ResourceNotAvailable") ||
		strings.Contains(s, "InvalidInstanceClass.NotFound") ||
		strings.Contains(s, "failed to create Redis in any available zone")
}

// generateRandomPassword generates a cryptographically secure random password
func generateRandomPassword() (string, error) {
	const (
		length  = 16
		upper   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		lower   = "abcdefghijklmnopqrstuvwxyz"
		digits  = "0123456789"
		special = "!@#%^*-_+="
		all     = upper + lower + digits + special
	)

	p := make([]byte, length)
	var err error
	if p[0], err = pickRandChar(upper); err != nil {
		return "", err
	}
	if p[1], err = pickRandChar(lower); err != nil {
		return "", err
	}
	if p[2], err = pickRandChar(digits); err != nil {
		return "", err
	}
	if p[3], err = pickRandChar(special); err != nil {
		return "", err
	}
	for i := 4; i < length; i++ {
		if p[i], err = pickRandChar(all); err != nil {
			return "", err
		}
	}
	secureShuffle(p)
	return string(p), nil
}

func pickRandChar(charset string) (byte, error) {
	if charset == "" {
		return 0, fmt.Errorf("EPROV010: empty charset")
	}
	b := make([]byte, 1)
	if _, err := rand.Read(b); err != nil {
		return 0, fmt.Errorf("EPROV010: crypto/rand failed: %w", err)
	}
	return charset[int(b[0])%len(charset)], nil
}

func secureShuffle(data []byte) {
	if len(data) < 2 {
		return
	}
	r := make([]byte, len(data))
	if _, err := rand.Read(r); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	for i := len(data) - 1; i > 0; i-- {
		j := int(r[i]) % (i + 1)
		data[i], data[j] = data[j], data[i]
	}
}

// waitForCacheInstanceReady polls until the Redis instance is ready and returns the endpoint
func (p *Provider) waitForCacheInstanceReady(ctx context.Context, instanceID string) (string, error) {
	ticker := time.NewTicker(ProvisionPollInterval)
	defer ticker.Stop()

	timeout := time.After(ProvisionPollTimeout)

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

// createRedisWithZoneFallback tries to create Redis with the given VSwitch, and falls back
// to creating a new VSwitch in a different zone if the current zone doesn't support Redis
func (p *Provider) createRedisWithZoneFallback(vpcID, vswID, instanceClass string, spec providers.CacheSpec) (*r_kvstore.CreateInstanceResponse, error) {
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

	// Try creating with the provided VSwitch first.
	resp, err := p.kvstoreClient.CreateInstance(request)
	if err == nil {
		return resp, nil
	}

	// Then try creating Redis in a fresh VSwitch across available zones.
	fallbackResp, fallbackErr := p.createRedisWithNewVSwitch(vpcID, instanceClass, spec)
	if fallbackErr == nil {
		return fallbackResp, nil
	}

	// Prefer the original provider error unless it was clearly a zone/vswitch mismatch.
	if strings.Contains(err.Error(), "InvalidvSwitchId") || strings.Contains(err.Error(), "zone not supported") {
		return nil, fallbackErr
	}
	return nil, err
}

// createRedisWithNewVSwitch creates a new VSwitch in a different zone for Redis
func (p *Provider) createRedisWithNewVSwitch(vpcID, instanceClass string, spec providers.CacheSpec) (*r_kvstore.CreateInstanceResponse, error) {
	// Get available zones for Redis
	zoneReq := r_kvstore.CreateDescribeAvailableResourceRequest()
	zoneReq.RegionId = p.region
	zoneReq.InstanceChargeType = "PostPaid"

	zoneResp, err := p.kvstoreClient.DescribeAvailableResource(zoneReq)
	if err != nil {
		return nil, fmt.Errorf("failed to describe available zones for Redis: %w", err)
	}

	lastErr := ""
	// Try each available zone
	for i, zone := range zoneResp.AvailableZones.AvailableZone {
		zoneID := zone.ZoneId

		for _, cidrBlock := range candidateRedisVSwitchCIDRs(i) {
			// Create VSwitch in this zone
			vswReq := vpc.CreateCreateVSwitchRequest()
			vswReq.RegionId = p.region
			vswReq.VpcId = vpcID
			vswReq.ZoneId = zoneID
			vswReq.CidrBlock = cidrBlock
			vswReq.VSwitchName = fmt.Sprintf("infracast-redis-%s-%d", zoneID, time.Now().UnixNano())

			vswResp, err := p.vpcClient.CreateVSwitch(vswReq)
			if err != nil {
				lastErr = fmt.Sprintf("zone=%s cidr=%s create-vswitch=%v", zoneID, cidrBlock, err)
				// try next cidr in this zone
				continue
			}

			// Try creating Redis with this VSwitch
			request := r_kvstore.CreateCreateInstanceRequest()
			request.VpcId = vpcID
			request.VSwitchId = vswResp.VSwitchId
			request.RegionId = p.region
			request.InstanceClass = instanceClass
			request.InstanceType = "Redis"
			request.EngineVersion = spec.Version
			if request.EngineVersion == "" {
				request.EngineVersion = "5.0"
			}

			resp, err := p.kvstoreClient.CreateInstance(request)
			if err == nil {
				return resp, nil
			}
			lastErr = fmt.Sprintf("zone=%s cidr=%s create-redis=%v", zoneID, cidrBlock, err)

			// Clean up the VSwitch we created if Redis creation failed
			deleteReq := vpc.CreateDeleteVSwitchRequest()
			deleteReq.VSwitchId = vswResp.VSwitchId
			_, _ = p.vpcClient.DeleteVSwitch(deleteReq) // Best effort cleanup
		}
	}

	if lastErr != "" {
		return nil, fmt.Errorf("failed to create Redis in any available zone: %s", lastErr)
	}
	return nil, fmt.Errorf("failed to create Redis in any available zone")
}

func candidateRedisVSwitchCIDRs(zoneIdx int) []string {
	base := 10 + (zoneIdx % 20)
	seen := map[string]struct{}{}
	out := make([]string, 0, 12)
	add := func(n int) {
		if n < 1 || n > 250 {
			return
		}
		c := fmt.Sprintf("10.0.%d.0/24", n)
		if _, ok := seen[c]; ok {
			return
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}

	// Try a small set of deterministic + time-salted CIDR candidates.
	add(base)
	add(base + 20)
	add(base + 40)
	add(base + 60)
	add(base + 80)
	add(base + 100)
	add(int(time.Now().UnixNano()%100) + 120)
	add(int(time.Now().UnixNano()%80) + 20)
	return out
}

// setCachePassword sets the password for the Redis instance
func (p *Provider) setCachePassword(ctx context.Context, instanceID, password string) error {
	_ = ctx

	// Try to discover an existing account first.
	accountName := "default"
	describeReq := r_kvstore.CreateDescribeAccountsRequest()
	describeReq.InstanceId = instanceID
	if describeResp, err := p.kvstoreClient.DescribeAccounts(describeReq); err == nil {
		if describeResp != nil && len(describeResp.Accounts.Account) > 0 && describeResp.Accounts.Account[0].AccountName != "" {
			accountName = describeResp.Accounts.Account[0].AccountName
		}
	}

	// Try to reset the account password.
	req := r_kvstore.CreateResetAccountPasswordRequest()
	req.InstanceId = instanceID
	req.AccountName = accountName
	req.AccountPassword = password

	_, err := p.kvstoreClient.ResetAccountPassword(req)
	if err != nil {
		// Some instance types do not expose a resettable account. Fall back to
		// instance-level password update.
		if strings.Contains(err.Error(), "InvalidAccountName.NotFound") {
			modReq := r_kvstore.CreateModifyInstanceAttributeRequest()
			modReq.InstanceId = instanceID
			modReq.NewPassword = password
			if _, modErr := p.kvstoreClient.ModifyInstanceAttribute(modReq); modErr == nil {
				return nil
			} else {
				return fmt.Errorf("failed to set Redis password: %w", modErr)
			}
		}
		return fmt.Errorf("failed to set Redis password: %w", err)
	}

	return nil
}
