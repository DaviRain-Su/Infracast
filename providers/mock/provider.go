// Package mock provides a mock cloud provider for testing
package mock

import (
	"context"
	"fmt"

	"github.com/DaviRain-Su/infracast/providers"
)

// Provider is a mock implementation of CloudProviderInterface
type Provider struct {
	name string
}

// New creates a new mock provider
func New() *Provider {
	return &Provider{name: "mock"}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return p.name
}

// DisplayName returns the display name
func (p *Provider) DisplayName() string {
	return "Mock Provider"
}

// Regions returns available regions
func (p *Provider) Regions() []providers.Region {
	return []providers.Region{
		{ID: "mock-region-1", Name: "mock-region-1", DisplayName: "Mock Region 1"},
	}
}

// ProvisionDatabase creates a mock database
func (p *Provider) ProvisionDatabase(ctx context.Context, spec providers.DatabaseSpec) (*providers.DatabaseOutput, error) {
	return &providers.DatabaseOutput{
		ResourceID: fmt.Sprintf("mock-db-%s", spec.Name),
		Endpoint:   "mock-db.example.com",
		Port:       3306,
		Username:   "mockuser",
		Password:   "mockpassword",
	}, nil
}

// ProvisionCache creates a mock cache
func (p *Provider) ProvisionCache(ctx context.Context, spec providers.CacheSpec) (*providers.CacheOutput, error) {
	return &providers.CacheOutput{
		ResourceID: fmt.Sprintf("mock-cache-%s", spec.Name),
		Endpoint:   "mock-cache.example.com",
		Port:       6379,
		Password:   "mockpassword",
	}, nil
}

// ProvisionObjectStorage creates a mock object storage
func (p *Provider) ProvisionObjectStorage(ctx context.Context, spec providers.ObjectStorageSpec) (*providers.ObjectStorageOutput, error) {
	return &providers.ObjectStorageOutput{
		ResourceID: fmt.Sprintf("mock-oss-%s", spec.Name),
		BucketName: spec.Name,
		Endpoint:   "mock-oss.example.com",
		Region:     "mock-region",
	}, nil
}

// ProvisionCompute creates a mock compute resource
func (p *Provider) ProvisionCompute(ctx context.Context, spec providers.ComputeSpec) (*providers.ComputeOutput, error) {
	return &providers.ComputeOutput{
		ResourceID:     fmt.Sprintf("mock-compute-%s", spec.ServiceName),
		Namespace:      "mock-namespace",
		ServiceName:    spec.ServiceName,
		DeploymentName: spec.ServiceName,
	}, nil
}

// Plan generates a mock plan
func (p *Provider) Plan(ctx context.Context, specs []providers.ResourceSpec) (*providers.PlanResult, error) {
	plans := make([]providers.ResourcePlan, len(specs))
	for i, spec := range specs {
		plans[i] = providers.ResourcePlan{
			Action: "create",
			Spec:   spec,
		}
	}
	return &providers.PlanResult{Resources: plans}, nil
}

// Apply executes a mock plan
func (p *Provider) Apply(ctx context.Context, plan *providers.PlanResult) (*providers.ApplyResult, error) {
	outputs := make([]providers.ResourceOutput, len(plan.Resources))
	for i, res := range plan.Resources {
		outputs[i] = providers.ResourceOutput{
			Type:       res.Spec.Type,
			Name:       "mock-name",
			ResourceID: fmt.Sprintf("mock-id-%d", i),
		}
	}
	return &providers.ApplyResult{Resources: outputs}, nil
}

// Destroy destroys mock resources
func (p *Provider) Destroy(ctx context.Context, envID string) error {
	return nil
}

// OTLPEndpoint returns the mock OTLP endpoint
func (p *Provider) OTLPEndpoint() string {
	return "http://mock-otel:4317"
}

// DashboardURL returns the mock dashboard URL
func (p *Provider) DashboardURL(envID string) string {
	return fmt.Sprintf("http://mock-dashboard/%s", envID)
}
