package alicloud

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/vpc"
)

const (
	networkVPCNamePrefix     = "infracast-vpc"
	networkVSwitchNamePrefix = "infracast-vswitch"
	networkVpcCidrBlock      = "10.0.0.0/16"
	networkVSwitchCidrBlock  = "10.0.0.0/24"
	networkZoneSuffix        = "a"
)

type networkState struct {
	ready     bool
	vpcID     string
	vswitchID string
}

func (p *Provider) ensureNetwork(_ context.Context) (string, string, error) {
	p.networkMu.Lock()
	defer p.networkMu.Unlock()

	if p.networkCache == nil {
		p.networkCache = &networkState{}
	}

	if p.networkCache.ready {
		return p.networkCache.vpcID, p.networkCache.vswitchID, nil
	}

	if p.region == "" {
		return "", "", fmt.Errorf("region is required for network setup")
	}

	if p.vpcClient == nil {
		return "", "", fmt.Errorf("VPC client not initialized")
	}

	vpcID, err := p.ensureVPC(p.region)
	if err != nil {
		return "", "", err
	}

	vswitchID, err := p.ensureVSwitch(vpcID)
	if err != nil {
		return "", "", err
	}

	p.networkCache.vpcID = vpcID
	p.networkCache.vswitchID = vswitchID
	p.networkCache.ready = true
	return vpcID, vswitchID, nil
}

func (p *Provider) ensureVPC(region string) (string, error) {
	describeReq := vpc.CreateDescribeVpcsRequest()
	describeReq.RegionId = region
	describeReq.VpcName = networkVPCName(region)
	describeReq.PageSize = requests.NewInteger(100)

	describeResp, err := p.vpcClient.DescribeVpcs(describeReq)
	if err == nil && describeResp != nil && describeResp.TotalCount > 0 && len(describeResp.Vpcs.Vpc) > 0 {
		existing := describeResp.Vpcs.Vpc[0]
		if existing.Status != "Available" {
			if err := p.waitForVPCAvailable(existing.VpcId); err != nil {
				return "", fmt.Errorf("failed to wait for existing VPC to become available: %w", err)
			}
		}
		return existing.VpcId, nil
	}

	createReq := vpc.CreateCreateVpcRequest()
	createReq.RegionId = region
	createReq.VpcName = networkVPCName(region)
	createReq.CidrBlock = networkVpcCidrBlock

	resp, err := p.vpcClient.CreateVpc(createReq)
	if err != nil {
		return "", fmt.Errorf("failed to create default VPC: %w", err)
	}
	if resp == nil || resp.VpcId == "" {
		return "", fmt.Errorf("failed to create default VPC: empty VPC ID in response")
	}
	
	// Wait for VPC to become Available
	if err := p.waitForVPCAvailable(resp.VpcId); err != nil {
		return "", fmt.Errorf("failed to wait for VPC to become available: %w", err)
	}
	
	return resp.VpcId, nil
}

func (p *Provider) ensureVSwitch(vpcID string) (string, error) {
	describeReq := vpc.CreateDescribeVSwitchesRequest()
	describeReq.RegionId = p.region
	describeReq.VpcId = vpcID
	describeReq.VSwitchName = networkVSwitchName(p.region)
	describeReq.PageSize = requests.NewInteger(100)

	describeResp, err := p.vpcClient.DescribeVSwitches(describeReq)
	if err == nil && describeResp != nil && describeResp.TotalCount > 0 && len(describeResp.VSwitches.VSwitch) > 0 {
		return describeResp.VSwitches.VSwitch[0].VSwitchId, nil
	}

	createReq := vpc.CreateCreateVSwitchRequest()
	createReq.RegionId = p.region
	createReq.VpcId = vpcID
	createReq.VSwitchName = networkVSwitchName(p.region)
	createReq.CidrBlock = networkVSwitchCidrBlock
	createReq.ZoneId = fmt.Sprintf("%s-%s", p.region, networkZoneSuffix)

	// Re-check VPC readiness before creating VSwitch. AliCloud may still reject
	// immediate VSwitch creation with IncorrectVpcStatus due eventual consistency.
	if err := p.waitForVPCAvailable(vpcID); err != nil {
		return "", fmt.Errorf("failed to wait for VPC before creating vSwitch: %w", err)
	}

	zoneCandidates := candidateZoneIDs(p.region)
	var lastErr error
	for _, zoneID := range zoneCandidates {
		createReq.ZoneId = zoneID

		for i := 0; i < 10; i++ {
			resp, err := p.vpcClient.CreateVSwitch(createReq)
			if err == nil {
				if resp == nil || resp.VSwitchId == "" {
					return "", fmt.Errorf("failed to create default vSwitch: empty vSwitch ID in response")
				}
				return resp.VSwitchId, nil
			}

			lastErr = err
			// VPC not yet fully ready: retry same zone.
			if strings.Contains(err.Error(), "IncorrectVpcStatus") {
				time.Sleep(6 * time.Second)
				continue
			}
			// Zone unavailable: switch to next candidate zone.
			if strings.Contains(err.Error(), "ResourceNotAvailable") {
				break
			}
			return "", fmt.Errorf("failed to create default vSwitch: %w", err)
		}
	}

	return "", fmt.Errorf("failed to create default vSwitch after retries: %w", lastErr)
}

func candidateZoneIDs(region string) []string {
	// Try region-a first, then fallback zones.
	suffixes := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	zones := make([]string, 0, len(suffixes))
	seen := make(map[string]struct{}, len(suffixes))
	for _, s := range suffixes {
		z := fmt.Sprintf("%s-%s", region, s)
		if _, ok := seen[z]; ok {
			continue
		}
		seen[z] = struct{}{}
		zones = append(zones, z)
	}
	return zones
}

func networkVPCName(region string) string {
	return fmt.Sprintf("%s-%s", networkVPCNamePrefix, region)
}

func networkVSwitchName(region string) string {
	return fmt.Sprintf("%s-%s", networkVSwitchNamePrefix, region)
}

// waitForVPCAvailable polls until the VPC status becomes "Available"
func (p *Provider) waitForVPCAvailable(vpcID string) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	timeout := time.After(2 * time.Minute)
	
	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for VPC %s to become Available", vpcID)
		case <-ticker.C:
			req := vpc.CreateDescribeVpcsRequest()
			req.VpcId = vpcID
			req.RegionId = p.region
			
			resp, err := p.vpcClient.DescribeVpcs(req)
			if err != nil {
				continue // Retry on error
			}
			
			if len(resp.Vpcs.Vpc) > 0 && resp.Vpcs.Vpc[0].Status == "Available" {
				return nil
			}
		}
	}
}

// TODO: Persist network IDs in state store for cross-process reuse.
