package alicloud

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/DaviRain-Su/infracast/internal/state"
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

func (p *Provider) ensureNetwork(ctx context.Context, envID string) (string, string, error) {
	p.networkMu.Lock()
	defer p.networkMu.Unlock()

	if p.networkCache == nil {
		p.networkCache = &networkState{}
	}

	if p.networkCache.ready {
		return p.networkCache.vpcID, p.networkCache.vswitchID, nil
	}

	// Try to restore from state store if available
	if p.stateStore != nil && envID != "" {
		if vpcID, vswitchID, err := p.restoreNetworkFromState(ctx, envID); err == nil {
			p.networkCache.vpcID = vpcID
			p.networkCache.vswitchID = vswitchID
			p.networkCache.ready = true
			return vpcID, vswitchID, nil
		}
	}

	if p.region == "" {
		return "", "", fmt.Errorf("region is required for network setup")
	}

	if p.vpcClient == nil {
		return "", "", fmt.Errorf("VPC client not initialized")
	}

	vpcID, err := p.ensureVPC(ctx, envID, p.region)
	if err != nil {
		return "", "", err
	}

	vswitchID, err := p.ensureVSwitch(ctx, envID, vpcID)
	if err != nil {
		return "", "", err
	}

	p.networkCache.vpcID = vpcID
	p.networkCache.vswitchID = vswitchID
	p.networkCache.ready = true
	return vpcID, vswitchID, nil
}

// restoreNetworkFromState attempts to restore network state from state store
func (p *Provider) restoreNetworkFromState(ctx context.Context, envID string) (string, string, error) {
	if p.stateStore == nil {
		return "", "", fmt.Errorf("state store not available")
	}

	// Get VPC from state
	vpcResource, err := p.stateStore.GetNetworkResource(ctx, envID, "vpc")
	if err != nil {
		return "", "", err
	}

	// Get VSwitch from state
	vswitchResource, err := p.stateStore.GetNetworkResource(ctx, envID, "vswitch")
	if err != nil {
		return "", "", err
	}

	// Verify VPC still exists and is available
	vpcID := vpcResource.ProviderResourceID
	if err := p.verifyVPCAvailable(vpcID); err != nil {
		return "", "", fmt.Errorf("stored VPC not available: %w", err)
	}

	// Verify VSwitch still exists
	vswitchID := vswitchResource.ProviderResourceID
	if err := p.verifyVSwitchExists(vswitchID, vpcID); err != nil {
		return "", "", fmt.Errorf("stored VSwitch not available: %w", err)
	}

	fmt.Printf("[Network] Restored from state store: VPC=%s VSwitch=%s\n", vpcID, vswitchID)
	return vpcID, vswitchID, nil
}

// verifyVPCAvailable checks if VPC exists and is available
func (p *Provider) verifyVPCAvailable(vpcID string) error {
	req := vpc.CreateDescribeVpcsRequest()
	req.VpcId = vpcID
	req.RegionId = p.region

	resp, err := p.vpcClient.DescribeVpcs(req)
	if err != nil {
		return err
	}
	if len(resp.Vpcs.Vpc) == 0 {
		return fmt.Errorf("VPC not found")
	}
	if resp.Vpcs.Vpc[0].Status != "Available" {
		return fmt.Errorf("VPC not available")
	}
	return nil
}

// verifyVSwitchExists checks if VSwitch exists
func (p *Provider) verifyVSwitchExists(vswitchID, vpcID string) error {
	req := vpc.CreateDescribeVSwitchesRequest()
	req.VSwitchId = vswitchID
	req.VpcId = vpcID
	req.RegionId = p.region

	resp, err := p.vpcClient.DescribeVSwitches(req)
	if err != nil {
		return err
	}
	if len(resp.VSwitches.VSwitch) == 0 {
		return fmt.Errorf("VSwitch not found")
	}
	return nil
}

func (p *Provider) ensureVPC(ctx context.Context, envID, region string) (string, error) {
	describeReq := vpc.CreateDescribeVpcsRequest()
	describeReq.RegionId = region
	describeReq.VpcName = networkVPCName(region)

	describeResp, err := p.vpcClient.DescribeVpcs(describeReq)
	if err == nil && describeResp != nil && describeResp.TotalCount > 0 && len(describeResp.Vpcs.Vpc) > 0 {
		existing := describeResp.Vpcs.Vpc[0]
		if existing.Status != "Available" {
			if err := p.waitForVPCAvailable(existing.VpcId); err != nil {
				return "", fmt.Errorf("failed to wait for existing VPC to become available: %w", err)
			}
		}
		// Save to state store if available
		if envID != "" {
			p.saveVPCState(ctx, envID, existing.VpcId, existing.VpcName)
		}
		return existing.VpcId, nil
	}

	// Fallback: reuse any existing infracast VPC in this region.
	if reusableID, found := p.findReusableVPC(region); found {
		if err := p.waitForVPCAvailable(reusableID); err != nil {
			return "", fmt.Errorf("failed to wait for reusable VPC to become available: %w", err)
		}
		// Save to state store if available
		if envID != "" {
			p.saveVPCState(ctx, envID, reusableID, networkVPCName(region))
		}
		return reusableID, nil
	}

	createReq := vpc.CreateCreateVpcRequest()
	createReq.RegionId = region
	createReq.VpcName = networkVPCName(region)
	createReq.CidrBlock = networkVpcCidrBlock

	resp, err := p.vpcClient.CreateVpc(createReq)
	if err != nil {
		// If VPC quota is exhausted, try reusing an existing VPC again.
		if strings.Contains(err.Error(), "QuotaExceeded.Vpc") {
			if reusableID, found := p.findReusableVPC(region); found {
				if waitErr := p.waitForVPCAvailable(reusableID); waitErr != nil {
					return "", fmt.Errorf("failed to wait for reusable VPC after quota exceeded: %w", waitErr)
				}
				// Save to state store if available
				if envID != "" {
					p.saveVPCState(ctx, envID, reusableID, networkVPCName(region))
				}
				return reusableID, nil
			}
		}
		return "", fmt.Errorf("failed to create default VPC: %w", err)
	}
	if resp == nil || resp.VpcId == "" {
		return "", fmt.Errorf("failed to create default VPC: empty VPC ID in response")
	}
	
	// Wait for VPC to become Available
	if err := p.waitForVPCAvailable(resp.VpcId); err != nil {
		return "", fmt.Errorf("failed to wait for VPC to become available: %w", err)
	}
	
	// Save to state store if available
	if envID != "" {
		p.saveVPCState(ctx, envID, resp.VpcId, networkVPCName(region))
	}
	
	return resp.VpcId, nil
}

// saveVPCState persists VPC information to state store
func (p *Provider) saveVPCState(ctx context.Context, envID, vpcID, vpcName string) {
	if p.stateStore == nil {
		return
	}

	configJSON, _ := json.Marshal(map[string]string{
		"vpc_name": vpcName,
		"cidr":     networkVpcCidrBlock,
	})

	resource := &state.InfraResource{
		EnvID:              envID,
		ResourceName:       vpcName,
		ResourceType:       "vpc",
		ProviderResourceID: vpcID,
		SpecHash:           fmt.Sprintf("vpc-%s", vpcID),
		ConfigJSON:         string(configJSON),
		Status:             "provisioned",
	}

	if err := p.stateStore.UpsertResource(ctx, resource); err != nil {
		fmt.Printf("[Network] Warning: failed to save VPC state: %v\n", err)
	} else {
		fmt.Printf("[Network] Saved VPC state: %s\n", vpcID)
	}
}

func (p *Provider) findReusableVPC(region string) (string, bool) {
	page := 1
	seen := 0
	var anyAvailable string
	for {
		req := vpc.CreateDescribeVpcsRequest()
		req.RegionId = region
		req.PageNumber = requests.NewInteger(page)

		resp, err := p.vpcClient.DescribeVpcs(req)
		if err != nil || resp == nil || len(resp.Vpcs.Vpc) == 0 {
			return "", false
		}

		seen += len(resp.Vpcs.Vpc)
		for _, item := range resp.Vpcs.Vpc {
			if strings.HasPrefix(item.VpcName, networkVPCNamePrefix) {
				return item.VpcId, true
			}
			if anyAvailable == "" && item.Status == "Available" {
				anyAvailable = item.VpcId
			}
		}

		if seen >= resp.TotalCount {
			if anyAvailable != "" {
				return anyAvailable, true
			}
			return "", false
		}
		page++
	}
}

func (p *Provider) ensureVSwitch(ctx context.Context, envID, vpcID string) (string, error) {
	vswName := networkVSwitchName(p.region)
	
	describeReq := vpc.CreateDescribeVSwitchesRequest()
	describeReq.RegionId = p.region
	describeReq.VpcId = vpcID
	describeReq.VSwitchName = vswName

	describeResp, err := p.vpcClient.DescribeVSwitches(describeReq)
	if err == nil && describeResp != nil && describeResp.TotalCount > 0 && len(describeResp.VSwitches.VSwitch) > 0 {
		vswitchID := describeResp.VSwitches.VSwitch[0].VSwitchId
		// Save to state store if available
		if envID != "" {
			p.saveVSwitchState(ctx, envID, vpcID, vswitchID, vswName)
		}
		return vswitchID, nil
	}

	createReq := vpc.CreateCreateVSwitchRequest()
	createReq.RegionId = p.region
	createReq.VpcId = vpcID
	createReq.VSwitchName = vswName
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
				// Save to state store if available
				if envID != "" {
					p.saveVSwitchState(ctx, envID, vpcID, resp.VSwitchId, vswName)
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

// saveVSwitchState persists VSwitch information to state store
func (p *Provider) saveVSwitchState(ctx context.Context, envID, vpcID, vswitchID, vswitchName string) {
	if p.stateStore == nil {
		return
	}

	configJSON, _ := json.Marshal(map[string]string{
		"vswitch_name": vswitchName,
		"vpc_id":       vpcID,
		"cidr":         networkVSwitchCidrBlock,
	})

	resource := &state.InfraResource{
		EnvID:              envID,
		ResourceName:       vswitchName,
		ResourceType:       "vswitch",
		ProviderResourceID: vswitchID,
		SpecHash:           fmt.Sprintf("vswitch-%s", vswitchID),
		ConfigJSON:         string(configJSON),
		Status:             "provisioned",
	}

	if err := p.stateStore.UpsertResource(ctx, resource); err != nil {
		fmt.Printf("[Network] Warning: failed to save VSwitch state: %v\n", err)
	} else {
		fmt.Printf("[Network] Saved VSwitch state: %s\n", vswitchID)
	}
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
