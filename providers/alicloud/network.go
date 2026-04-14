package alicloud

import (
	"context"
	"fmt"

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
		return describeResp.Vpcs.Vpc[0].VpcId, nil
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

	resp, err := p.vpcClient.CreateVSwitch(createReq)
	if err != nil {
		return "", fmt.Errorf("failed to create default vSwitch: %w", err)
	}
	if resp == nil || resp.VSwitchId == "" {
		return "", fmt.Errorf("failed to create default vSwitch: empty vSwitch ID in response")
	}
	return resp.VSwitchId, nil
}

func networkVPCName(region string) string {
	return fmt.Sprintf("%s-%s", networkVPCNamePrefix, region)
}

func networkVSwitchName(region string) string {
	return fmt.Sprintf("%s-%s", networkVSwitchNamePrefix, region)
}

// TODO: Persist network IDs in state store for cross-process reuse.
