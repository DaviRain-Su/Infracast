// Package main provides a cleanup tool for Infracast test resources
package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/auth/credentials"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/r-kvstore"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/rds"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/vpc"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

func main() {
	// Load credentials from environment
	accessKey := os.Getenv("ALICLOUD_ACCESS_KEY")
	secretKey := os.Getenv("ALICLOUD_SECRET_KEY")
	region := os.Getenv("ALICLOUD_REGION")
	if region == "" {
		region = "cn-hangzhou"
	}

	if accessKey == "" || secretKey == "" {
		log.Fatal("ALICLOUD_ACCESS_KEY and ALICLOUD_SECRET_KEY must be set")
	}

	fmt.Println("Starting cleanup of Infracast test resources...")
	fmt.Printf("Region: %s\n\n", region)

	// Create clients
	config := sdk.NewConfig()
	cred := credentials.NewAccessKeyCredential(accessKey, secretKey)

	rdsClient, err := rds.NewClientWithOptions(region, config, cred)
	if err != nil {
		log.Printf("Warning: failed to create RDS client: %v", err)
	}

	kvstoreClient, err := r_kvstore.NewClientWithOptions(region, config, cred)
	if err != nil {
		log.Printf("Warning: failed to create KVStore client: %v", err)
	}

	vpcClient, err := vpc.NewClientWithOptions(region, config, cred)
	if err != nil {
		log.Printf("Warning: failed to create VPC client: %v", err)
	}

	endpoint := fmt.Sprintf("oss-%s.aliyuncs.com", region)
	ossClient, err := oss.New(endpoint, accessKey, secretKey)
	if err != nil {
		log.Printf("Warning: failed to create OSS client: %v", err)
	}

	// Cleanup in order: OSS -> Redis -> RDS -> VSwitch -> VPC
	cleanupOSS(ossClient)
	cleanupRedis(kvstoreClient, region)
	cleanupRDS(rdsClient, region)
	cleanupNetwork(vpcClient, region)

	fmt.Println("\n✅ Cleanup completed!")
}

func cleanupOSS(client *oss.Client) {
	if client == nil {
		return
	}
	fmt.Println("Cleaning up OSS buckets...")

	marker := ""
	for {
		result, err := client.ListBuckets(oss.Marker(marker))
		if err != nil {
			log.Printf("  Error listing buckets: %v", err)
			return
		}

		for _, bucket := range result.Buckets {
			if strings.Contains(bucket.Name, "infracast") {
				fmt.Printf("  Deleting OSS bucket: %s\n", bucket.Name)
				if err := client.DeleteBucket(bucket.Name); err != nil {
					log.Printf("    Error deleting bucket %s: %v", bucket.Name, err)
				} else {
					fmt.Printf("    ✅ Deleted: %s\n", bucket.Name)
				}
			}
		}

		if !result.IsTruncated {
			break
		}
		marker = result.NextMarker
	}
}

func cleanupRedis(client *r_kvstore.Client, region string) {
	if client == nil {
		return
	}
	fmt.Println("\nCleaning up Redis instances...")

	req := r_kvstore.CreateDescribeInstancesRequest()
	req.RegionId = region

	resp, err := client.DescribeInstances(req)
	if err != nil {
		log.Printf("  Error listing Redis instances: %v", err)
		return
	}

	for _, inst := range resp.Instances.KVStoreInstance {
		if strings.Contains(inst.InstanceName, "infracast") {
			fmt.Printf("  Deleting Redis instance: %s (%s)\n", inst.InstanceName, inst.InstanceId)
			
			deleteReq := r_kvstore.CreateDeleteInstanceRequest()
			deleteReq.InstanceId = inst.InstanceId
			
			if _, err := client.DeleteInstance(deleteReq); err != nil {
				log.Printf("    Error deleting Redis %s: %v", inst.InstanceId, err)
			} else {
				fmt.Printf("    ✅ Deleted: %s\n", inst.InstanceId)
			}
		}
	}
}

func cleanupRDS(client *rds.Client, region string) {
	if client == nil {
		return
	}
	fmt.Println("\nCleaning up RDS instances...")

	req := rds.CreateDescribeDBInstancesRequest()
	req.RegionId = region

	resp, err := client.DescribeDBInstances(req)
	if err != nil {
		log.Printf("  Error listing RDS instances: %v", err)
		return
	}

	for _, inst := range resp.Items.DBInstance {
		// Check if instance name contains infracast or was created by our tests
		if strings.Contains(inst.DBInstanceId, "infracast") || strings.HasPrefix(inst.DBInstanceId, "rm-") {
			fmt.Printf("  Deleting RDS instance: %s\n", inst.DBInstanceId)
			
			deleteReq := rds.CreateDeleteDBInstanceRequest()
			deleteReq.DBInstanceId = inst.DBInstanceId
			
			if _, err := client.DeleteDBInstance(deleteReq); err != nil {
				log.Printf("    Error deleting RDS %s: %v", inst.DBInstanceId, err)
			} else {
				fmt.Printf("    ✅ Deleted: %s\n", inst.DBInstanceId)
			}
		}
	}
}

func cleanupNetwork(client *vpc.Client, region string) {
	if client == nil {
		return
	}
	fmt.Println("\nCleaning up network resources...")

	// First, list and delete VSwitches
	vswReq := vpc.CreateDescribeVSwitchesRequest()
	vswReq.RegionId = region

	vswResp, err := client.DescribeVSwitches(vswReq)
	if err != nil {
		log.Printf("  Error listing VSwitches: %v", err)
	} else {
		for _, vsw := range vswResp.VSwitches.VSwitch {
			if strings.Contains(vsw.VSwitchName, "infracast") {
				fmt.Printf("  Deleting VSwitch: %s (%s)\n", vsw.VSwitchName, vsw.VSwitchId)
				
				deleteReq := vpc.CreateDeleteVSwitchRequest()
				deleteReq.VSwitchId = vsw.VSwitchId
				
				if _, err := client.DeleteVSwitch(deleteReq); err != nil {
					log.Printf("    Error deleting VSwitch %s: %v", vsw.VSwitchId, err)
				} else {
					fmt.Printf("    ✅ Deleted: %s\n", vsw.VSwitchId)
				}
				
				// Small delay to avoid API throttling
				time.Sleep(500 * time.Millisecond)
			}
		}
	}

	// Then, list and delete VPCs
	vpcReq := vpc.CreateDescribeVpcsRequest()
	vpcReq.RegionId = region

	vpcResp, err := client.DescribeVpcs(vpcReq)
	if err != nil {
		log.Printf("  Error listing VPCs: %v", err)
		return
	}

	deletedCount := 0
	for _, v := range vpcResp.Vpcs.Vpc {
		if strings.Contains(v.VpcName, "infracast") {
			// Skip if this is the last VPC and we want to keep one
			if deletedCount == 0 {
				fmt.Printf("  Keeping 1 VPC for reuse: %s (%s)\n", v.VpcName, v.VpcId)
				deletedCount++
				continue
			}
			
			fmt.Printf("  Deleting VPC: %s (%s)\n", v.VpcName, v.VpcId)
			
			deleteReq := vpc.CreateDeleteVpcRequest()
			deleteReq.VpcId = v.VpcId
			
			if _, err := client.DeleteVpc(deleteReq); err != nil {
				log.Printf("    Error deleting VPC %s: %v", v.VpcId, err)
			} else {
				fmt.Printf("    ✅ Deleted: %s\n", v.VpcId)
				deletedCount++
			}
			
			// Small delay to avoid API throttling
			time.Sleep(500 * time.Millisecond)
		}
	}
}
