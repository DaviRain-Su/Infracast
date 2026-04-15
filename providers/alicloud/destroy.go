// Package alicloud provides Aliyun Cloud provider adapter
package alicloud

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/r-kvstore"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/rds"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/vpc"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// DestroyOptions contains options for destroy operation
type DestroyOptions struct {
	DryRun bool
	Prefix string
}

// DestroyResult contains the result of destroy operation
type DestroyResult struct {
	Deleted  []string
	Failed   []string
	Skipped  []string
	Duration time.Duration
}

// Destroy destroys a single resource by its provider resource ID
// This implements CloudProviderInterface.Destroy
// Supports both provider types (RDS/Redis/OSS) and state types (database/cache/object_storage)
func (p *Provider) Destroy(ctx context.Context, resourceID string) error {
	// Parse resource type from resourceID (format: "TYPE:ID")
	parts := strings.SplitN(resourceID, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid resource ID format: %s", resourceID)
	}
	resType := parts[0]
	resID := parts[1]

	// Map state types to provider types
	switch resType {
	case "RDS", "database":
		return p.destroySingleRDS(ctx, resID)
	case "Redis", "cache":
		return p.destroySingleRedis(ctx, resID)
	case "OSS", "object_storage":
		return p.destroySingleOSS(ctx, resID)
	case "VSwitch":
		return p.deleteVSwitchWithRetry(ctx, resID)
	case "VPC":
		return p.deleteVPCWithRetry(ctx, resID)
	case "compute":
		// Compute resources may not need cloud cleanup (handled by K8s)
		fmt.Printf("[Destroy] Compute resource %s - no cloud cleanup needed\n", resID)
		return nil
	default:
		return fmt.Errorf("unknown resource type: %s", resType)
	}
}

// DestroyEnvironment destroys all resources for an environment (bulk operation)
func (p *Provider) DestroyEnvironment(ctx context.Context, envID string, opts DestroyOptions) (*DestroyResult, error) {
	start := time.Now()
	result := &DestroyResult{
		Deleted: []string{},
		Failed:  []string{},
		Skipped: []string{},
	}

	prefix := opts.Prefix
	if prefix == "" {
		prefix = fmt.Sprintf("infracast-%s", envID)
	}

	fmt.Printf("[Destroy] Starting destruction for env=%s dryRun=%v\n", envID, opts.DryRun)

	// Phase 1: Delete compute/database resources (highest level)
	if err := p.destroyRDS(ctx, prefix, opts.DryRun, result); err != nil {
		fmt.Printf("[Destroy] RDS cleanup error: %v\n", err)
	}

	if err := p.destroyRedis(ctx, prefix, opts.DryRun, result); err != nil {
		fmt.Printf("[Destroy] Redis cleanup error: %v\n", err)
	}

	if err := p.destroyOSS(ctx, prefix, opts.DryRun, result); err != nil {
		fmt.Printf("[Destroy] OSS cleanup error: %v\n", err)
	}

	// Phase 2: Delete network resources
	if err := p.destroyVSwitches(ctx, prefix, opts.DryRun, result); err != nil {
		fmt.Printf("[Destroy] VSwitch cleanup error: %v\n", err)
	}

	if err := p.destroyVPCs(ctx, prefix, opts.DryRun, result); err != nil {
		fmt.Printf("[Destroy] VPC cleanup error: %v\n", err)
	}

	result.Duration = time.Since(start)
	fmt.Printf("[Destroy] Completed in %v: deleted=%d failed=%d skipped=%d\n",
		result.Duration, len(result.Deleted), len(result.Failed), len(result.Skipped))

	return result, nil
}

// destroySingleRDS destroys a single RDS instance by ID
func (p *Provider) destroySingleRDS(ctx context.Context, instanceID string) error {
	// Check if exists
	req := rds.CreateDescribeDBInstancesRequest()
	req.RegionId = p.region
	req.DBInstanceId = instanceID

	resp, err := p.rdsClient.DescribeDBInstances(req)
	if err != nil {
		if strings.Contains(err.Error(), "InvalidDBInstanceId.NotFound") {
			return nil // Already deleted, idempotent
		}
		return err
	}
	if len(resp.Items.DBInstance) == 0 {
		return nil // Already deleted
	}

	inst := resp.Items.DBInstance[0]
	if inst.DBInstanceStatus == "Deleting" || inst.DBInstanceStatus == "Deleted" {
		return p.waitForRDSDeleted(ctx, instanceID)
	}

	// Delete
	delReq := rds.CreateDeleteDBInstanceRequest()
	delReq.DBInstanceId = instanceID
	delReq.RegionId = p.region

	if _, err := p.rdsClient.DeleteDBInstance(delReq); err != nil {
		if strings.Contains(err.Error(), "InvalidDBInstanceId.NotFound") {
			return nil // Idempotent
		}
		return err
	}

	return p.waitForRDSDeleted(ctx, instanceID)
}

// destroySingleRedis destroys a single Redis instance by ID
func (p *Provider) destroySingleRedis(ctx context.Context, instanceID string) error {
	// Check if exists
	req := r_kvstore.CreateDescribeInstancesRequest()
	req.RegionId = p.region

	resp, err := p.kvstoreClient.DescribeInstances(req)
	if err != nil {
		return err
	}

	found := false
	for _, inst := range resp.Instances.KVStoreInstance {
		if inst.InstanceId == instanceID {
			found = true
			break
		}
	}
	if !found {
		return nil // Already deleted, idempotent
	}

	// Delete
	delReq := r_kvstore.CreateDeleteInstanceRequest()
	delReq.InstanceId = instanceID

	if _, err := p.kvstoreClient.DeleteInstance(delReq); err != nil {
		if strings.Contains(err.Error(), "InvalidInstanceId.NotFound") {
			return nil // Idempotent
		}
		return err
	}

	return p.waitForRedisDeleted(ctx, instanceID)
}

// destroySingleOSS destroys a single OSS bucket by name
func (p *Provider) destroySingleOSS(ctx context.Context, bucketName string) error {
	// Check if exists
	exists, err := p.ossClient.IsBucketExist(bucketName)
	if err != nil {
		return err
	}
	if !exists {
		return nil // Already deleted, idempotent
	}

	// Get bucket client and delete all objects
	bucketClient, err := p.ossClient.Bucket(bucketName)
	if err != nil {
		return err
	}

	// Delete all objects (batch delete for efficiency)
	marker := ""
	for {
		objects, err := bucketClient.ListObjects(oss.Marker(marker), oss.MaxKeys(1000))
		if err != nil {
			return err
		}

		if len(objects.Objects) > 0 {
			// Prepare keys for batch delete
			keys := make([]string, 0, len(objects.Objects))
			for _, obj := range objects.Objects {
				keys = append(keys, obj.Key)
			}
			// Batch delete
			if _, err := bucketClient.DeleteObjects(keys, oss.DeleteObjectsQuiet(true)); err != nil {
				// Fallback to individual delete on batch failure
				for _, obj := range objects.Objects {
					bucketClient.DeleteObject(obj.Key)
				}
			}
		}

		if !objects.IsTruncated {
			break
		}
		marker = objects.NextMarker
	}

	// Delete bucket
	if err := p.ossClient.DeleteBucket(bucketName); err != nil {
		if strings.Contains(err.Error(), "NoSuchBucket") {
			return nil // Idempotent
		}
		return err
	}

	return nil
}

// destroyRDS deletes RDS instances matching prefix (using Description/Name, not ID)
func (p *Provider) destroyRDS(ctx context.Context, prefix string, dryRun bool, result *DestroyResult) error {
	fmt.Printf("[Destroy] Scanning RDS instances with prefix=%s\n", prefix)

	pageNum := 1
	for {
		req := rds.CreateDescribeDBInstancesRequest()
		req.RegionId = p.region
		req.PageSize = requests.NewInteger(100)
		req.PageNumber = requests.NewInteger(pageNum)

		resp, err := p.rdsClient.DescribeDBInstances(req)
		if err != nil {
			return fmt.Errorf("list RDS page %d: %w", pageNum, err)
		}

		for _, inst := range resp.Items.DBInstance {
			// Match by Description (instance name), not ID
			if !strings.HasPrefix(inst.DBInstanceDescription, prefix) {
				continue
			}

			resourceID := fmt.Sprintf("RDS:%s", inst.DBInstanceId)
			fmt.Printf("[Destroy] Found RDS instance: %s (name=%s, status=%s)\n",
				inst.DBInstanceId, inst.DBInstanceDescription, inst.DBInstanceStatus)

			if dryRun {
				fmt.Printf("[Destroy] [DRY-RUN] Would delete RDS: %s\n", inst.DBInstanceId)
				result.Skipped = append(result.Skipped, resourceID)
				continue
			}

			if err := p.destroySingleRDS(ctx, inst.DBInstanceId); err != nil {
				fmt.Printf("[Destroy] Failed to delete RDS %s: %v\n", inst.DBInstanceId, err)
				result.Failed = append(result.Failed, resourceID)
			} else {
				fmt.Printf("[Destroy] RDS %s deleted\n", inst.DBInstanceId)
				result.Deleted = append(result.Deleted, resourceID)
			}
		}

		// Check if more pages
		if len(resp.Items.DBInstance) < 100 {
			break
		}
		pageNum++
	}

	return nil
}

// waitForRDSDeleted polls until RDS instance is deleted
func (p *Provider) waitForRDSDeleted(ctx context.Context, instanceID string) error {
	maxRetries := 30 // 5 minutes total (30 * 10s)
	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
		}

		req := rds.CreateDescribeDBInstancesRequest()
		req.RegionId = p.region
		req.DBInstanceId = instanceID

		resp, err := p.rdsClient.DescribeDBInstances(req)
		if err != nil {
			if strings.Contains(err.Error(), "InvalidDBInstanceId.NotFound") {
				fmt.Printf("[Destroy] RDS %s confirmed deleted\n", instanceID)
				return nil
			}
			return err
		}

		if len(resp.Items.DBInstance) == 0 {
			fmt.Printf("[Destroy] RDS %s no longer exists\n", instanceID)
			return nil
		}

		status := resp.Items.DBInstance[0].DBInstanceStatus
		fmt.Printf("[Destroy] RDS %s status: %s (retry %d/%d)\n", instanceID, status, i+1, maxRetries)

		if status == "Deleted" {
			return nil
		}
	}

	return fmt.Errorf("timeout waiting for RDS %s deletion", instanceID)
}

// destroyRedis deletes Redis instances matching prefix (using Name, not ID)
func (p *Provider) destroyRedis(ctx context.Context, prefix string, dryRun bool, result *DestroyResult) error {
	fmt.Printf("[Destroy] Scanning Redis instances with prefix=%s\n", prefix)

	pageNum := 1
	for {
		req := r_kvstore.CreateDescribeInstancesRequest()
		req.RegionId = p.region
		req.PageSize = requests.NewInteger(50)
		req.PageNumber = requests.NewInteger(pageNum)

		resp, err := p.kvstoreClient.DescribeInstances(req)
		if err != nil {
			return fmt.Errorf("list Redis page %d: %w", pageNum, err)
		}

		for _, inst := range resp.Instances.KVStoreInstance {
			// Match by InstanceName, not InstanceId
			if !strings.HasPrefix(inst.InstanceName, prefix) {
				continue
			}

			resourceID := fmt.Sprintf("Redis:%s", inst.InstanceId)
			fmt.Printf("[Destroy] Found Redis instance: %s (name=%s, status=%s)\n",
				inst.InstanceId, inst.InstanceName, inst.InstanceStatus)

			if dryRun {
				fmt.Printf("[Destroy] [DRY-RUN] Would delete Redis: %s\n", inst.InstanceId)
				result.Skipped = append(result.Skipped, resourceID)
				continue
			}

			if err := p.destroySingleRedis(ctx, inst.InstanceId); err != nil {
				fmt.Printf("[Destroy] Failed to delete Redis %s: %v\n", inst.InstanceId, err)
				result.Failed = append(result.Failed, resourceID)
			} else {
				fmt.Printf("[Destroy] Redis %s deleted\n", inst.InstanceId)
				result.Deleted = append(result.Deleted, resourceID)
			}
		}

		// Check if more pages
		if len(resp.Instances.KVStoreInstance) < 50 {
			break
		}
		pageNum++
	}

	return nil
}

// waitForRedisDeleted polls until Redis instance is deleted
func (p *Provider) waitForRedisDeleted(ctx context.Context, instanceID string) error {
	maxRetries := 30 // 5 minutes
	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
		}

		req := r_kvstore.CreateDescribeInstancesRequest()
		req.RegionId = p.region

		resp, err := p.kvstoreClient.DescribeInstances(req)
		if err != nil {
			return err
		}

		found := false
		for _, inst := range resp.Instances.KVStoreInstance {
			if inst.InstanceId == instanceID {
				found = true
				fmt.Printf("[Destroy] Redis %s status: %s (retry %d/%d)\n",
					instanceID, inst.InstanceStatus, i+1, maxRetries)
				break
			}
		}

		if !found {
			fmt.Printf("[Destroy] Redis %s confirmed deleted\n", instanceID)
			return nil
		}
	}

	return fmt.Errorf("timeout waiting for Redis %s deletion", instanceID)
}

// destroyOSS deletes OSS buckets matching prefix
func (p *Provider) destroyOSS(ctx context.Context, prefix string, dryRun bool, result *DestroyResult) error {
	fmt.Printf("[Destroy] Scanning OSS buckets with prefix=%s\n", prefix)

	marker := ""
	for {
		// List buckets with pagination
		resp, err := p.ossClient.ListBuckets(oss.Marker(marker))
		if err != nil {
			return fmt.Errorf("list OSS buckets: %w", err)
		}

		for _, bucket := range resp.Buckets {
			if !strings.HasPrefix(bucket.Name, prefix) {
				continue
			}

			resourceID := fmt.Sprintf("OSS:%s", bucket.Name)
			fmt.Printf("[Destroy] Found OSS bucket: %s\n", bucket.Name)

			if dryRun {
				fmt.Printf("[Destroy] [DRY-RUN] Would delete OSS bucket: %s\n", bucket.Name)
				result.Skipped = append(result.Skipped, resourceID)
				continue
			}

			if err := p.destroySingleOSS(ctx, bucket.Name); err != nil {
				fmt.Printf("[Destroy] Failed to delete OSS bucket %s: %v\n", bucket.Name, err)
				result.Failed = append(result.Failed, resourceID)
			} else {
				fmt.Printf("[Destroy] OSS bucket %s deleted\n", bucket.Name)
				result.Deleted = append(result.Deleted, resourceID)
			}
		}

		// Check for more buckets
		if !resp.IsTruncated {
			break
		}
		marker = resp.NextMarker
	}

	return nil
}

// destroyVSwitches deletes VSwitchs with pagination and retry
func (p *Provider) destroyVSwitches(ctx context.Context, prefix string, dryRun bool, result *DestroyResult) error {
	fmt.Printf("[Destroy] Scanning VSwitches with prefix=%s\n", prefix)

	pageNum := 1
	failedVSwitches := []string{}

	for {
		req := vpc.CreateDescribeVSwitchesRequest()
		req.RegionId = p.region
		req.PageSize = requests.NewInteger(50)
		req.PageNumber = requests.NewInteger(pageNum)

		resp, err := p.vpcClient.DescribeVSwitches(req)
		if err != nil {
			return fmt.Errorf("list VSwitches page %d: %w", pageNum, err)
		}

		for _, vsw := range resp.VSwitches.VSwitch {
			if !strings.HasPrefix(vsw.VSwitchName, prefix) {
				continue
			}

			resourceID := fmt.Sprintf("VSwitch:%s", vsw.VSwitchId)
			fmt.Printf("[Destroy] Found VSwitch: %s (vpc=%s)\n", vsw.VSwitchId, vsw.VpcId)

			if dryRun {
				fmt.Printf("[Destroy] [DRY-RUN] Would delete VSwitch: %s\n", vsw.VSwitchId)
				result.Skipped = append(result.Skipped, resourceID)
				continue
			}

			if err := p.deleteVSwitchWithRetry(ctx, vsw.VSwitchId); err != nil {
				fmt.Printf("[Destroy] Failed to delete VSwitch %s: %v\n", vsw.VSwitchId, err)
				failedVSwitches = append(failedVSwitches, vsw.VSwitchId)
				result.Failed = append(result.Failed, resourceID)
			} else {
				fmt.Printf("[Destroy] VSwitch %s deleted\n", vsw.VSwitchId)
				result.Deleted = append(result.Deleted, resourceID)
			}
		}

		// Check if more pages
		if len(resp.VSwitches.VSwitch) < 50 {
			break
		}
		pageNum++
	}

	// Retry failed VSwitches
	if len(failedVSwitches) > 0 {
		fmt.Printf("[Destroy] Retrying %d failed VSwitches...\n", len(failedVSwitches))
		for _, vswID := range failedVSwitches {
			resourceID := fmt.Sprintf("VSwitch:%s", vswID)
			if err := p.deleteVSwitchWithRetry(ctx, vswID); err != nil {
				fmt.Printf("[Destroy] Retry failed for VSwitch %s: %v\n", vswID, err)
			} else {
				fmt.Printf("[Destroy] VSwitch %s deleted on retry\n", vswID)
				// Move from failed to deleted
				for i, f := range result.Failed {
					if f == resourceID {
						result.Failed = append(result.Failed[:i], result.Failed[i+1:]...)
						break
					}
				}
				result.Deleted = append(result.Deleted, resourceID)
			}
		}
	}

	return nil
}

// deleteVSwitchWithRetry attempts to delete a VSwitch with retry logic
func (p *Provider) deleteVSwitchWithRetry(ctx context.Context, vswID string) error {
	maxRetries := 6 // Total ~3 minutes (6 * 30s)
	for i := 0; i < maxRetries; i++ {
		delReq := vpc.CreateDeleteVSwitchRequest()
		delReq.VSwitchId = vswID

		_, err := p.vpcClient.DeleteVSwitch(delReq)
		if err == nil {
			return nil
		}

		// Check if already deleted (idempotent)
		if strings.Contains(err.Error(), "InvalidVSwitchId.NotFound") {
			fmt.Printf("[Destroy] VSwitch %s already deleted (idempotent)\n", vswID)
			return nil
		}

		// Check for dependency violation - wait and retry
		if strings.Contains(err.Error(), "DependencyViolation") {
			if i < maxRetries-1 {
				waitTime := 30 * time.Second
				fmt.Printf("[Destroy] VSwitch %s has dependencies, waiting %v before retry %d/%d...\n",
					vswID, waitTime, i+1, maxRetries)
				time.Sleep(waitTime)
				continue
			}
		}

		return err
	}

	return fmt.Errorf("max retries exceeded for VSwitch %s", vswID)
}

// destroyVPCs deletes VPCs with pagination and retry
func (p *Provider) destroyVPCs(ctx context.Context, prefix string, dryRun bool, result *DestroyResult) error {
	fmt.Printf("[Destroy] Scanning VPCs with prefix=%s\n", prefix)

	pageNum := 1
	for {
		req := vpc.CreateDescribeVpcsRequest()
		req.RegionId = p.region
		req.PageSize = requests.NewInteger(50)
		req.PageNumber = requests.NewInteger(pageNum)

		resp, err := p.vpcClient.DescribeVpcs(req)
		if err != nil {
			return fmt.Errorf("list VPCs page %d: %w", pageNum, err)
		}

		for _, vpcInfo := range resp.Vpcs.Vpc {
			if !strings.HasPrefix(vpcInfo.VpcName, prefix) {
				continue
			}

			resourceID := fmt.Sprintf("VPC:%s", vpcInfo.VpcId)
			fmt.Printf("[Destroy] Found VPC: %s\n", vpcInfo.VpcId)

			if dryRun {
				fmt.Printf("[Destroy] [DRY-RUN] Would delete VPC: %s\n", vpcInfo.VpcId)
				result.Skipped = append(result.Skipped, resourceID)
				continue
			}

			if err := p.deleteVPCWithRetry(ctx, vpcInfo.VpcId); err != nil {
				fmt.Printf("[Destroy] Failed to delete VPC %s: %v\n", vpcInfo.VpcId, err)
				result.Failed = append(result.Failed, resourceID)
			} else {
				fmt.Printf("[Destroy] VPC %s deleted\n", vpcInfo.VpcId)
				result.Deleted = append(result.Deleted, resourceID)
			}
		}

		// Check if more pages
		if len(resp.Vpcs.Vpc) < 50 {
			break
		}
		pageNum++
	}

	return nil
}

// deleteVPCWithRetry attempts to delete a VPC with retry logic
func (p *Provider) deleteVPCWithRetry(ctx context.Context, vpcID string) error {
	maxRetries := 6 // Total ~3 minutes
	for i := 0; i < maxRetries; i++ {
		delReq := vpc.CreateDeleteVpcRequest()
		delReq.VpcId = vpcID

		_, err := p.vpcClient.DeleteVpc(delReq)
		if err == nil {
			return nil
		}

		// Check if already deleted (idempotent)
		if strings.Contains(err.Error(), "InvalidVpcId.NotFound") {
			fmt.Printf("[Destroy] VPC %s already deleted (idempotent)\n", vpcID)
			return nil
		}

		// Check for dependency violation - wait and retry
		if strings.Contains(err.Error(), "DependencyViolation") {
			if i < maxRetries-1 {
				waitTime := 30 * time.Second
				fmt.Printf("[Destroy] VPC %s has dependencies, waiting %v before retry %d/%d...\n",
					vpcID, waitTime, i+1, maxRetries)
				time.Sleep(waitTime)
				continue
			}
		}

		return err
	}

	return fmt.Errorf("max retries exceeded for VPC %s", vpcID)
}
