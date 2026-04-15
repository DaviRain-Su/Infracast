package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/auth/credentials"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/r-kvstore"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/rds"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/vpc"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

type cleanupConfig struct {
	Region       string
	Prefix       string
	BucketPrefix string
	KeepVPC      int
	Apply        bool
}

type cleaner struct {
	cfg         cleanupConfig
	rdsClient   *rds.Client
	redisClient *r_kvstore.Client
	vpcClient   *vpc.Client
	ossClient   *oss.Client
}

func main() {
	cfg := parseFlags()

	ak := envAny("ALICLOUD_ACCESS_KEY", "ALICLOUD_ACCESS_KEY_ID")
	sk := envAny("ALICLOUD_SECRET_KEY", "ALICLOUD_ACCESS_KEY_SECRET")
	if ak == "" || sk == "" {
		log.Fatal("missing ALICLOUD_ACCESS_KEY / ALICLOUD_SECRET_KEY")
	}

	c, err := newCleaner(cfg, ak, sk)
	if err != nil {
		log.Fatalf("init cleanup client failed: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("cleanup start region=%s prefix=%s keep-vpc=%d apply=%v", cfg.Region, cfg.Prefix, cfg.KeepVPC, cfg.Apply)
	if !cfg.Apply {
		log.Printf("running in dry-run mode, pass --apply to execute deletion")
	}

	if err := c.run(ctx); err != nil {
		log.Fatalf("cleanup failed: %v", err)
	}
	log.Printf("cleanup finished")
}

func parseFlags() cleanupConfig {
	regionDefault := envAny("ALICLOUD_REGION")
	if regionDefault == "" {
		regionDefault = "cn-hangzhou"
	}

	prefixDefault := "infracast"
	cfg := cleanupConfig{}
	flag.StringVar(&cfg.Region, "region", regionDefault, "aliyun region")
	flag.StringVar(&cfg.Prefix, "prefix", prefixDefault, "resource name prefix")
	flag.StringVar(&cfg.BucketPrefix, "bucket-prefix", prefixDefault, "oss bucket prefix")
	flag.IntVar(&cfg.KeepVPC, "keep-vpc", 1, "number of matching VPCs to keep")
	flag.BoolVar(&cfg.Apply, "apply", false, "execute deletion (default dry-run)")
	flag.Parse()
	return cfg
}

func newCleaner(cfg cleanupConfig, accessKeyID, accessKeySecret string) (*cleaner, error) {
	sdkCfg := sdk.NewConfig().WithTimeout(60 * time.Second).WithScheme("HTTPS")
	cred := credentials.NewAccessKeyCredential(accessKeyID, accessKeySecret)

	rdsClient, err := rds.NewClientWithOptions(cfg.Region, sdkCfg, cred)
	if err != nil {
		return nil, fmt.Errorf("create RDS client: %w", err)
	}
	redisClient, err := r_kvstore.NewClientWithOptions(cfg.Region, sdkCfg, cred)
	if err != nil {
		return nil, fmt.Errorf("create Redis client: %w", err)
	}
	vpcClient, err := vpc.NewClientWithOptions(cfg.Region, sdkCfg, cred)
	if err != nil {
		return nil, fmt.Errorf("create VPC client: %w", err)
	}
	ossClient, err := oss.New(fmt.Sprintf("oss-%s.aliyuncs.com", cfg.Region), accessKeyID, accessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("create OSS client: %w", err)
	}

	return &cleaner{
		cfg:         cfg,
		rdsClient:   rdsClient,
		redisClient: redisClient,
		vpcClient:   vpcClient,
		ossClient:   ossClient,
	}, nil
}

func (c *cleaner) run(ctx context.Context) error {
	infracastVPCs, err := c.listPrefixedVPCs(ctx, c.cfg.Prefix)
	if err != nil {
		return err
	}

	keep, drop := splitVPCs(infracastVPCs, c.cfg.KeepVPC)
	dropVPCIDs := make(map[string]struct{}, len(drop))
	for _, v := range drop {
		dropVPCIDs[v.VpcId] = struct{}{}
	}

	if len(keep) > 0 {
		keepIDs := make([]string, 0, len(keep))
		for _, v := range keep {
			keepIDs = append(keepIDs, v.VpcId)
		}
		log.Printf("keeping VPCs: %s", strings.Join(keepIDs, ", "))
	}

	if err := c.cleanupRDS(ctx, dropVPCIDs); err != nil {
		return err
	}
	if err := c.cleanupRedis(ctx, dropVPCIDs); err != nil {
		return err
	}
	if err := c.cleanupBuckets(ctx); err != nil {
		return err
	}
	if err := c.cleanupVPCs(ctx, drop); err != nil {
		return err
	}

	return nil
}

func (c *cleaner) cleanupRDS(_ context.Context, dropVPCIDs map[string]struct{}) error {
	all, err := c.listAllRDS()
	if err != nil {
		return fmt.Errorf("list RDS: %w", err)
	}

	for _, inst := range all {
		if !shouldDeleteRDS(inst, dropVPCIDs, c.cfg.Prefix) {
			continue
		}

		log.Printf("[RDS] delete id=%s name=%s vpc=%s", inst.DBInstanceId, inst.DBInstanceDescription, inst.VpcId)
		if !c.cfg.Apply {
			continue
		}

		req := rds.CreateDeleteDBInstanceRequest()
		req.DBInstanceId = inst.DBInstanceId
		req.ReleasedKeepPolicy = "None"
		if _, err := c.rdsClient.DeleteDBInstance(req); err != nil {
			log.Printf("[RDS] delete failed id=%s err=%v", inst.DBInstanceId, err)
		}
	}
	return nil
}

func (c *cleaner) cleanupRedis(_ context.Context, dropVPCIDs map[string]struct{}) error {
	all, err := c.listAllRedis()
	if err != nil {
		return fmt.Errorf("list Redis: %w", err)
	}

	for _, inst := range all {
		if !shouldDeleteRedis(inst, dropVPCIDs, c.cfg.Prefix) {
			continue
		}

		log.Printf("[Redis] delete id=%s name=%s vpc=%s", inst.InstanceId, inst.InstanceName, inst.VpcId)
		if !c.cfg.Apply {
			continue
		}

		req := r_kvstore.CreateDeleteInstanceRequest()
		req.InstanceId = inst.InstanceId
		req.ReleaseSubInstance = requests.NewBoolean(true)
		if _, err := c.redisClient.DeleteInstance(req); err != nil {
			log.Printf("[Redis] delete failed id=%s err=%v", inst.InstanceId, err)
		}
	}
	return nil
}

func (c *cleaner) cleanupBuckets(_ context.Context) error {
	resp, err := c.ossClient.ListBuckets()
	if err != nil {
		return fmt.Errorf("list OSS buckets: %w", err)
	}

	for _, b := range resp.Buckets {
		if !hasPrefixInsensitive(b.Name, c.cfg.BucketPrefix) {
			continue
		}

		log.Printf("[OSS] delete bucket=%s", b.Name)
		if !c.cfg.Apply {
			continue
		}

		bucket, err := c.ossClient.Bucket(b.Name)
		if err != nil {
			log.Printf("[OSS] open bucket failed bucket=%s err=%v", b.Name, err)
			continue
		}
		if err := deleteAllObjects(bucket); err != nil {
			log.Printf("[OSS] clear objects failed bucket=%s err=%v", b.Name, err)
			continue
		}
		if err := c.ossClient.DeleteBucket(b.Name); err != nil {
			log.Printf("[OSS] delete bucket failed bucket=%s err=%v", b.Name, err)
		}
	}
	return nil
}

func (c *cleaner) cleanupVPCs(_ context.Context, drop []vpc.Vpc) error {
	for _, item := range drop {
		log.Printf("[VPC] cleanup vpc=%s name=%s", item.VpcId, item.VpcName)
		vsws, err := c.listVSwitchesByVPC(item.VpcId)
		if err != nil {
			log.Printf("[VPC] list vswitches failed vpc=%s err=%v", item.VpcId, err)
			continue
		}

		for _, vsw := range vsws {
			log.Printf("[VSwitch] delete id=%s name=%s vpc=%s", vsw.VSwitchId, vsw.VSwitchName, item.VpcId)
			if !c.cfg.Apply {
				continue
			}

			req := vpc.CreateDeleteVSwitchRequest()
			req.RegionId = c.cfg.Region
			req.VSwitchId = vsw.VSwitchId
			if _, err := c.vpcClient.DeleteVSwitch(req); err != nil {
				log.Printf("[VSwitch] delete failed id=%s err=%v", vsw.VSwitchId, err)
			}
		}

		if !c.cfg.Apply {
			continue
		}

		req := vpc.CreateDeleteVpcRequest()
		req.RegionId = c.cfg.Region
		req.VpcId = item.VpcId
		req.ForceDelete = requests.NewBoolean(true)
		if _, err := c.vpcClient.DeleteVpc(req); err != nil {
			log.Printf("[VPC] delete failed id=%s err=%v", item.VpcId, err)
		}
	}
	return nil
}

func (c *cleaner) listPrefixedVPCs(_ context.Context, prefix string) ([]vpc.Vpc, error) {
	all := make([]vpc.Vpc, 0)
	page := 1

	for {
		req := vpc.CreateDescribeVpcsRequest()
		req.RegionId = c.cfg.Region
		req.PageSize = requests.NewInteger(50)
		req.PageNumber = requests.NewInteger(page)

		resp, err := c.vpcClient.DescribeVpcs(req)
		if err != nil {
			return nil, err
		}
		if resp == nil || len(resp.Vpcs.Vpc) == 0 {
			break
		}

		for _, item := range resp.Vpcs.Vpc {
			if hasPrefixInsensitive(item.VpcName, prefix+"-vpc") || hasPrefixInsensitive(item.VpcName, prefix) {
				all = append(all, item)
			}
		}

		if page*resp.PageSize >= resp.TotalCount || resp.PageSize == 0 {
			break
		}
		page++
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].CreationTime < all[j].CreationTime
	})
	return all, nil
}

func (c *cleaner) listAllRDS() ([]rds.DBInstance, error) {
	all := make([]rds.DBInstance, 0)
	page := 1

	for {
		req := rds.CreateDescribeDBInstancesRequest()
		req.RegionId = c.cfg.Region
		req.PageSize = requests.NewInteger(50)
		req.PageNumber = requests.NewInteger(page)

		resp, err := c.rdsClient.DescribeDBInstances(req)
		if err != nil {
			return nil, err
		}
		if resp == nil || len(resp.Items.DBInstance) == 0 {
			break
		}

		all = append(all, resp.Items.DBInstance...)
		if page*50 >= resp.TotalRecordCount {
			break
		}
		page++
	}

	return all, nil
}

func (c *cleaner) listAllRedis() ([]r_kvstore.KVStoreInstance, error) {
	all := make([]r_kvstore.KVStoreInstance, 0)
	page := 1

	for {
		req := r_kvstore.CreateDescribeInstancesRequest()
		req.RegionId = c.cfg.Region
		req.PageSize = requests.NewInteger(50)
		req.PageNumber = requests.NewInteger(page)

		resp, err := c.redisClient.DescribeInstances(req)
		if err != nil {
			return nil, err
		}
		if resp == nil || len(resp.Instances.KVStoreInstance) == 0 {
			break
		}

		all = append(all, resp.Instances.KVStoreInstance...)
		if page*resp.PageSize >= resp.TotalCount || resp.PageSize == 0 {
			break
		}
		page++
	}

	return all, nil
}

func (c *cleaner) listVSwitchesByVPC(vpcID string) ([]vpc.VSwitch, error) {
	all := make([]vpc.VSwitch, 0)
	page := 1

	for {
		req := vpc.CreateDescribeVSwitchesRequest()
		req.RegionId = c.cfg.Region
		req.VpcId = vpcID
		req.PageSize = requests.NewInteger(50)
		req.PageNumber = requests.NewInteger(page)

		resp, err := c.vpcClient.DescribeVSwitches(req)
		if err != nil {
			return nil, err
		}
		if resp == nil || len(resp.VSwitches.VSwitch) == 0 {
			break
		}

		all = append(all, resp.VSwitches.VSwitch...)
		if page*resp.PageSize >= resp.TotalCount || resp.PageSize == 0 {
			break
		}
		page++
	}

	return all, nil
}

func splitVPCs(vpcs []vpc.Vpc, keepN int) (keep []vpc.Vpc, drop []vpc.Vpc) {
	if keepN < 0 {
		keepN = 0
	}
	if keepN > len(vpcs) {
		keepN = len(vpcs)
	}
	return vpcs[:keepN], vpcs[keepN:]
}

func shouldDeleteRDS(inst rds.DBInstance, dropVPCIDs map[string]struct{}, prefix string) bool {
	if _, ok := dropVPCIDs[inst.VpcId]; ok {
		return true
	}
	return hasPrefixInsensitive(inst.DBInstanceDescription, prefix) ||
		hasPrefixInsensitive(inst.DBInstanceName, prefix)
}

func shouldDeleteRedis(inst r_kvstore.KVStoreInstance, dropVPCIDs map[string]struct{}, prefix string) bool {
	if _, ok := dropVPCIDs[inst.VpcId]; ok {
		return true
	}
	return hasPrefixInsensitive(inst.InstanceName, prefix)
}

func deleteAllObjects(bucket *oss.Bucket) error {
	marker := ""
	for {
		resp, err := bucket.ListObjects(oss.Marker(marker), oss.MaxKeys(1000))
		if err != nil {
			return err
		}

		keys := make([]string, 0, len(resp.Objects))
		for _, obj := range resp.Objects {
			keys = append(keys, obj.Key)
		}
		if len(keys) > 0 {
			if _, err := bucket.DeleteObjects(keys, oss.DeleteObjectsQuiet(true)); err != nil {
				return err
			}
		}

		if !resp.IsTruncated {
			break
		}
		marker = resp.NextMarker
	}
	return nil
}

func envAny(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func hasPrefixInsensitive(s, prefix string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(s)), strings.ToLower(strings.TrimSpace(prefix)))
}
