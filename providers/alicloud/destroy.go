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

// DestroyEnvironment destroys all resources for an environment
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

	// Phase 2: Delete network resources (wait for compute to release)
	fmt.Printf("[Destroy] Waiting for compute resources to release network interfaces...\n")
	time.Sleep(10 * time.Second)

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

// destroyRDS deletes RDS instances
func (p *Provider) destroyRDS(ctx context.Context, prefix string, dryRun bool, result *DestroyResult) error {
	fmt.Printf("[Destroy] Scanning RDS instances with prefix=%s\n", prefix)

	req := rds.CreateDescribeDBInstancesRequest()
	req.RegionId = p.region
	req.PageSize = requests.NewInteger(100)

	resp, err := p.rdsClient.DescribeDBInstances(req)
	if err != nil {
		return fmt.Errorf("list RDS: %w", err)
	}

	for _, inst := range resp.Items.DBInstance {
		if !strings.Contains(inst.DBInstanceId, prefix) && !strings.Contains(inst.DBInstanceDescription, prefix) {
			continue
		}

		resourceID := fmt.Sprintf("RDS:%s", inst.DBInstanceId)
		fmt.Printf("[Destroy] Found RDS instance: %s (status=%s)\n", inst.DBInstanceId, inst.DBInstanceStatus)

		if dryRun {
			fmt.Printf("[Destroy] [DRY-RUN] Would delete RDS: %s\n", inst.DBInstanceId)
			result.Skipped = append(result.Skipped, resourceID)
			continue
		}

		// Check if already being deleted
		if inst.DBInstanceStatus == "Deleting" {
			fmt.Printf("[Destroy] RDS %s is already being deleted, waiting...\n", inst.DBInstanceId)
			if err := p.waitForRDSDeleted(ctx, inst.DBInstanceId); err != nil {
				fmt.Printf("[Destroy] Failed to wait for RDS %s deletion: %v\n", inst.DBInstanceId, err)
				result.Failed = append(result.Failed, resourceID)
			} else {
				result.Deleted = append(result.Deleted, resourceID)
			}
			continue
		}

		// Delete the instance
		delReq := rds.CreateDeleteDBInstanceRequest()
		delReq.DBInstanceId = inst.DBInstanceId
		delReq.RegionId = p.region

		if _, err := p.rdsClient.DeleteDBInstance(delReq); err != nil {
			// Check if already deleted (idempotent)
			if strings.Contains(err.Error(), "InvalidDBInstanceId.NotFound") {
				fmt.Printf("[Destroy] RDS %s already deleted (idempotent)\n", inst.DBInstanceId)
				result.Deleted = append(result.Deleted, resourceID)
				continue
			}
			fmt.Printf("[Destroy] Failed to delete RDS %s: %v\n", inst.DBInstanceId, err)
			result.Failed = append(result.Failed, resourceID)
			continue
		}

		fmt.Printf("[Destroy] RDS %s delete initiated, waiting...\n", inst.DBInstanceId)
		if err := p.waitForRDSDeleted(ctx, inst.DBInstanceId); err != nil {
			fmt.Printf("[Destroy] Failed to wait for RDS %s deletion: %v\n", inst.DBInstanceId, err)
			result.Failed = append(result.Failed, resourceID)
		} else {
			result.Deleted = append(result.Deleted, resourceID)
		}
	}

	return nil
}

// waitForRDSDeleted polls until RDS instance is deleted
func (p *Provider) waitForRDSDeleted(ctx context.Context, instanceID string) error {
	maxRetries := 60
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
			// Check if instance not found (means deleted)
			if strings.Contains(err.Error(), "InvalidDBInstanceId.NotFound") {
				fmt.Printf("[Destroy] RDS %s confirmed deleted\n", instanceID)
				return nil
			}
			return err
		}

		// Check if any instances returned
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

// destroyRedis deletes Redis instances
func (p *Provider) destroyRedis(ctx context.Context, prefix string, dryRun bool, result *DestroyResult) error {
	fmt.Printf("[Destroy] Scanning Redis instances with prefix=%s\n", prefix)

	req := r_kvstore.CreateDescribeInstancesRequest()
	req.RegionId = p.region
	req.PageSize = requests.NewInteger(50)

	resp, err := p.kvstoreClient.DescribeInstances(req)
	if err != nil {
		return fmt.Errorf("list Redis: %w", err)
	}

	for _, inst := range resp.Instances.KVStoreInstance {
		if !strings.Contains(inst.InstanceId, prefix) && !strings.Contains(inst.InstanceName, prefix) {
			continue
		}

		resourceID := fmt.Sprintf("Redis:%s", inst.InstanceId)
		fmt.Printf("[Destroy] Found Redis instance: %s (status=%s)\n", inst.InstanceId, inst.InstanceStatus)

		if dryRun {
			fmt.Printf("[Destroy] [DRY-RUN] Would delete Redis: %s\n", inst.InstanceId)
			result.Skipped = append(result.Skipped, resourceID)
			continue
		}

		// Delete the instance
		delReq := r_kvstore.CreateDeleteInstanceRequest()
		delReq.InstanceId = inst.InstanceId

		if _, err := p.kvstoreClient.DeleteInstance(delReq); err != nil {
			// Check if already deleted (idempotent)
			if strings.Contains(err.Error(), "InvalidInstanceId.NotFound") {
				fmt.Printf("[Destroy] Redis %s already deleted (idempotent)\n", inst.InstanceId)
				result.Deleted = append(result.Deleted, resourceID)
				continue
			}
			fmt.Printf("[Destroy] Failed to delete Redis %s: %v\n", inst.InstanceId, err)
			result.Failed = append(result.Failed, resourceID)
			continue
		}

		fmt.Printf("[Destroy] Redis %s delete initiated, waiting...\n", inst.InstanceId)
		if err := p.waitForRedisDeleted(ctx, inst.InstanceId); err != nil {
			fmt.Printf("[Destroy] Failed to wait for Redis %s deletion: %v\n", inst.InstanceId, err)
			result.Failed = append(result.Failed, resourceID)
		} else {
			result.Deleted = append(result.Deleted, resourceID)
		}
	}

	return nil
}

// waitForRedisDeleted polls until Redis instance is deleted
func (p *Provider) waitForRedisDeleted(ctx context.Context, instanceID string) error {
	maxRetries := 60
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

		// Check if instance still exists
		found := false
		for _, inst := range resp.Instances.KVStoreInstance {
			if inst.InstanceId == instanceID {
				found = true
				fmt.Printf("[Destroy] Redis %s status: %s (retry %d/%d)\n", instanceID, inst.InstanceStatus, i+1, maxRetries)
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

// destroyOSS deletes OSS buckets
func (p *Provider) destroyOSS(ctx context.Context, prefix string, dryRun bool, result *DestroyResult) error {
	fmt.Printf("[Destroy] Scanning OSS buckets with prefix=%s\n", prefix)

	// List all buckets
	buckets, err := p.ossClient.ListBuckets()
	if err != nil {
		return fmt.Errorf("list OSS buckets: %w", err)
	}

	for _, bucket := range buckets.Buckets {
		if !strings.Contains(bucket.Name, prefix) {
			continue
		}

		resourceID := fmt.Sprintf("OSS:%s", bucket.Name)
		fmt.Printf("[Destroy] Found OSS bucket: %s\n", bucket.Name)

		if dryRun {
			fmt.Printf("[Destroy] [DRY-RUN] Would delete OSS bucket: %s\n", bucket.Name)
			result.Skipped = append(result.Skipped, resourceID)
			continue
		}

		// Get bucket client
		bucketClient, err := p.ossClient.Bucket(bucket.Name)
		if err != nil {
			fmt.Printf("[Destroy] Failed to get bucket client for %s: %v\n", bucket.Name, err)
			result.Failed = append(result.Failed, resourceID)
			continue
		}

		// Delete all objects first
		marker := ""
		for {
			objects, err := bucketClient.ListObjects(oss.Marker(marker))
			if err != nil {
				fmt.Printf("[Destroy] Failed to list objects in bucket %s: %v\n", bucket.Name, err)
				break
			}

			for _, obj := range objects.Objects {
				if err := bucketClient.DeleteObject(obj.Key); err != nil {
					fmt.Printf("[Destroy] Failed to delete object %s: %v\n", obj.Key, err)
				}
			}

			if !objects.IsTruncated {
				break
			}
			marker = objects.NextMarker
		}

		// Delete the bucket
		if err := p.ossClient.DeleteBucket(bucket.Name); err != nil {
			// Check if already deleted (idempotent)
			if strings.Contains(err.Error(), "NoSuchBucket") {
				fmt.Printf("[Destroy] OSS bucket %s already deleted (idempotent)\n", bucket.Name)
				result.Deleted = append(result.Deleted, resourceID)
				continue
			}
			fmt.Printf("[Destroy] Failed to delete OSS bucket %s: %v\n", bucket.Name, err)
			result.Failed = append(result.Failed, resourceID)
			continue
		}

		fmt.Printf("[Destroy] OSS bucket %s deleted\n", bucket.Name)
		result.Deleted = append(result.Deleted, resourceID)
	}

	return nil
}

// destroyVSwitches deletes VSwitchs
func (p *Provider) destroyVSwitches(ctx context.Context, prefix string, dryRun bool, result *DestroyResult) error {
	fmt.Printf("[Destroy] Scanning VSwitches with prefix=%s\n", prefix)

	req := vpc.CreateDescribeVSwitchesRequest()
	req.RegionId = p.region
	req.PageSize = requests.NewInteger(50)

	resp, err := p.vpcClient.DescribeVSwitches(req)
	if err != nil {
		return fmt.Errorf("list VSwitches: %w", err)
	}

	// Track failed VSwitches for retry
	failedVSwitches := []string{}

	for _, vsw := range resp.VSwitches.VSwitch {
		if !strings.Contains(vsw.VSwitchName, prefix) {
			continue
		}

		resourceID := fmt.Sprintf("VSwitch:%s", vsw.VSwitchId)
		fmt.Printf("[Destroy] Found VSwitch: %s (vpc=%s)\n", vsw.VSwitchId, vsw.VpcId)

		if dryRun {
			fmt.Printf("[Destroy] [DRY-RUN] Would delete VSwitch: %s\n", vsw.VSwitchId)
			result.Skipped = append(result.Skipped, resourceID)
			continue
		}

		// Try to delete with retry for dependency violations
		if err := p.deleteVSwitchWithRetry(ctx, vsw.VSwitchId); err != nil {
			fmt.Printf("[Destroy] Failed to delete VSwitch %s: %v\n", vsw.VSwitchId, err)
			failedVSwitches = append(failedVSwitches, vsw.VSwitchId)
			result.Failed = append(result.Failed, resourceID)
		} else {
			fmt.Printf("[Destroy] VSwitch %s deleted\n", vsw.VSwitchId)
			result.Deleted = append(result.Deleted, resourceID)
		}
	}

	// Retry failed VSwitches after waiting
	if len(failedVSwitches) > 0 {
		fmt.Printf("[Destroy] Retrying %d failed VSwitches after 30s...\n", len(failedVSwitches))
		time.Sleep(30 * time.Second)

		for _, vswID := range failedVSwitches {
			resourceID := fmt.Sprintf("VSwitch:%s", vswID)
			if err := p.deleteVSwitchWithRetry(ctx, vswID); err != nil {
				fmt.Printf("[Destroy] Retry failed for VSwitch %s: %v\n", vswID, err)
			} else {
				fmt.Printf("[Destroy] VSwitch %s deleted on retry\n", vswID)
				// Remove from failed list
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
	maxRetries := 3
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
				fmt.Printf("[Destroy] VSwitch %s has dependencies, waiting 10s before retry %d/%d...\n",
					vswID, i+1, maxRetries)
				time.Sleep(10 * time.Second)
				continue
			}
		}

		return err
	}

	return fmt.Errorf("max retries exceeded for VSwitch %s", vswID)
}

// destroyVPCs deletes VPCs
func (p *Provider) destroyVPCs(ctx context.Context, prefix string, dryRun bool, result *DestroyResult) error {
	fmt.Printf("[Destroy] Scanning VPCs with prefix=%s\n", prefix)

	req := vpc.CreateDescribeVpcsRequest()
	req.RegionId = p.region
	req.PageSize = requests.NewInteger(50)

	resp, err := p.vpcClient.DescribeVpcs(req)
	if err != nil {
		return fmt.Errorf("list VPCs: %w", err)
	}

	for _, vpcInfo := range resp.Vpcs.Vpc {
		if !strings.Contains(vpcInfo.VpcName, prefix) {
			continue
		}

		resourceID := fmt.Sprintf("VPC:%s", vpcInfo.VpcId)
		fmt.Printf("[Destroy] Found VPC: %s\n", vpcInfo.VpcId)

		if dryRun {
			fmt.Printf("[Destroy] [DRY-RUN] Would delete VPC: %s\n", vpcInfo.VpcId)
			result.Skipped = append(result.Skipped, resourceID)
			continue
		}

		// Try to delete with retry for dependency violations
		if err := p.deleteVPCWithRetry(ctx, vpcInfo.VpcId); err != nil {
			fmt.Printf("[Destroy] Failed to delete VPC %s: %v\n", vpcInfo.VpcId, err)
			result.Failed = append(result.Failed, resourceID)
		} else {
			fmt.Printf("[Destroy] VPC %s deleted\n", vpcInfo.VpcId)
			result.Deleted = append(result.Deleted, resourceID)
		}
	}

	return nil
}

// deleteVPCWithRetry attempts to delete a VPC with retry logic
func (p *Provider) deleteVPCWithRetry(ctx context.Context, vpcID string) error {
	maxRetries := 5
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
				waitTime := time.Duration(10+i*5) * time.Second
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
