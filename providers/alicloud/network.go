package alicloud

import (
	"context"
	"fmt"
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

	p.networkCache.vpcID = formatVPCID(p.region)
	p.networkCache.vswitchID = formatVSwitchID(p.region)
	p.networkCache.ready = true

	return p.networkCache.vpcID, p.networkCache.vswitchID, nil
}

func formatVPCID(region string) string {
	return "vpc-infracast-" + region
}

func formatVSwitchID(region string) string {
	return "vsw-infracast-" + region
}
