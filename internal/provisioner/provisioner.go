// Package provisioner provides infrastructure provisioning orchestration
package provisioner

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/DaviRain-Su/infracast/internal/credentials"
	"github.com/DaviRain-Su/infracast/internal/mapper"
	"github.com/DaviRain-Su/infracast/internal/state"
	"github.com/DaviRain-Su/infracast/pkg/hash"
	"github.com/DaviRain-Su/infracast/providers"
	alicloudprovider "github.com/DaviRain-Su/infracast/providers/alicloud"
)

// ResourceState represents the state of a resource in the provisioning lifecycle
type ResourceState string

const (
	// ResourceStatePending indicates resource is queued for provisioning
	ResourceStatePending ResourceState = "pending"
	// ResourceStateProvisioning indicates resource is being created/updated
	ResourceStateProvisioning ResourceState = "provisioning"
	// ResourceStateProvisioned indicates resource is ready
	ResourceStateProvisioned ResourceState = "provisioned"
	// ResourceStateUpdating indicates resource is being updated
	ResourceStateUpdating ResourceState = "updating"
	// ResourceStateFailed indicates resource provisioning failed
	ResourceStateFailed ResourceState = "failed"
	// ResourceStateDeleting indicates resource is being deleted (use 'updating' in DB)
	ResourceStateDeleting ResourceState = "updating"
	// ResourceStateDeleted indicates resource has been deleted (destroyed in DB schema)
	ResourceStateDeleted ResourceState = "destroyed"
)

// ProvisionInput represents the input to the Provision operation
type ProvisionInput struct {
	EnvID       string                           `json:"env_id"`
	BuildMeta   mapper.BuildMeta                 `json:"build_meta"`
	Resources   []providers.ResourceSpec         `json:"resources,omitempty"` // Explicit resource specs (optional)
	Provider    providers.CloudProviderInterface `json:"-"`                   // Provider implementation
	DryRun      bool                             `json:"dry_run"`
	Credentials credentials.CredentialConfig     `json:"credentials"`
}

// ProvisionSummary provides aggregated counts of provisioning operations
type ProvisionSummary struct {
	Created int `json:"created"`
	Updated int `json:"updated"`
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
	Total   int `json:"total"`
}

// ProvisionResult represents the result of the Provision operation
type ProvisionResult struct {
	Success   bool             `json:"success"`
	Resources []ResourceResult `json:"resources"`
	Plan      *PlanResult      `json:"plan,omitempty"`
	Errors    []ProvisionError `json:"errors,omitempty"`
	Summary   ProvisionSummary `json:"summary"`
}

// Provisioner orchestrates infrastructure provisioning
type Provisioner struct {
	registry    *providers.Registry
	store       *state.Store
	mapper      *mapper.Mapper
	credentials *credentials.Manager
}

// NewProvisioner creates a new provisioner
// Aligned with Tech Spec: NewProvisioner(store, creds)
func NewProvisioner(store *state.Store, creds *credentials.Manager) *Provisioner {
	// Create registry and mapper internally (P0: alicloud only)
	registry := providers.NewRegistry()
	mapperInst := mapper.NewMapper(registry)

	return &Provisioner{
		registry:    registry,
		store:       store,
		mapper:      mapperInst,
		credentials: creds,
	}
}

// PlanResult represents the result of a planning operation
type PlanResult struct {
	Resources []ResourcePlan `json:"resources"`
}

// ResourcePlan represents a planned operation for a single resource
type ResourcePlan struct {
	Action    string                 `json:"action"` // create, update, delete, noop
	Spec      providers.ResourceSpec `json:"spec"`
	OldHash   string                 `json:"old_hash,omitempty"`
	NewHash   string                 `json:"new_hash,omitempty"`
	Priority  int                    `json:"priority"`
	DependsOn []string               `json:"depends_on,omitempty"`
}

// ApplyResult represents the result of applying a plan
type ApplyResult struct {
	Resources []ResourceResult `json:"resources"`
	Success   bool             `json:"success"`
}

// ResourceResult represents the result of applying a resource plan
type ResourceResult struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Action   string      `json:"action"`
	Success  bool        `json:"success"`
	ErrorMsg string      `json:"error_msg,omitempty"`
	Output   interface{} `json:"output,omitempty"`
}

// Provision is the high-level entry point for provisioning
// Implements Tech Spec §3.4: Provision(ctx, ProvisionInput) (*ProvisionResult, error)
func (p *Provisioner) Provision(ctx context.Context, input ProvisionInput) (*ProvisionResult, error) {
	// Validate credentials first (EPROV001)
	if p.credentials != nil {
		_, err := p.credentials.GetCredentials(input.Credentials)
		if err != nil {
			return nil, &ProvisionError{
				Code:      "EPROV001",
				Message:   "failed to fetch credentials",
				Retryable: false, // B1-R2: credential errors are not retryable
				Cause:     err,
			}
		}
	}

	// Use explicit resources if provided, otherwise map from build meta
	var specs []providers.ResourceSpec
	if len(input.Resources) > 0 {
		specs = input.Resources
	} else {
		specs = p.mapper.MapToResourceSpecs(input.BuildMeta)
	}

	// Register provider if provided
	if input.Provider != nil {
		p.registry.Register(input.Provider)
	}

	// Generate plan
	plan, err := p.Plan(ctx, input.EnvID, specs)
	if err != nil {
		return nil, &ProvisionError{
			Code:      "EPROV005",
			Message:   "failed to generate provisioning plan",
			Retryable: true,
			Cause:     err,
		}
	}

	// Dry run: return plan only
	if input.DryRun {
		return &ProvisionResult{
			Success: true,
			Plan:    plan,
		}, nil
	}

	// Apply plan
	applyResult, err := p.Apply(ctx, input.EnvID, plan)
	if err != nil {
		return nil, &ProvisionError{
			Code:      "EPROV004",
			Message:   "resource provisioning failed",
			Retryable: true,
			Cause:     err,
		}
	}

	// Build result with errors
	result := &ProvisionResult{
		Success:   applyResult.Success,
		Resources: applyResult.Resources,
	}

	// Collect errors from failed resources
	for _, res := range applyResult.Resources {
		if !res.Success {
			result.Errors = append(result.Errors, ProvisionError{
				ResourceName: res.Name,
				Code:         "EPROV004",
				Message:      res.ErrorMsg,
				Retryable:    true,
			})
		}
	}

	// Calculate summary (B1-R1)
	result.Summary = p.calculateSummary(applyResult.Resources)

	return result, nil
}

// Plan generates a provisioning plan based on desired state
func (p *Provisioner) Plan(ctx context.Context, envID string, specs []providers.ResourceSpec) (*PlanResult, error) {
	var plans []ResourcePlan

	for _, spec := range specs {
		plan, err := p.planResource(ctx, envID, spec)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to plan resource %s: %v", ErrInvalidSpec, spec.Type, err)
		}
		plans = append(plans, plan)
	}

	// Sort by priority (lower = earlier)
	sort.Slice(plans, func(i, j int) bool {
		return plans[i].Priority < plans[j].Priority
	})

	return &PlanResult{Resources: plans}, nil
}

// planResource plans a single resource operation
func (p *Provisioner) planResource(ctx context.Context, envID string, spec providers.ResourceSpec) (ResourcePlan, error) {
	resourceName := getResourceName(spec)

	// Compute spec hash - extract concrete type from ResourceSpec
	var specHash string
	var hashErr error
	switch spec.Type {
	case "database":
		if spec.DatabaseSpec != nil {
			specHash, hashErr = hash.SpecHash(hash.ResourceTypeDatabase, *spec.DatabaseSpec)
		}
	case "cache":
		if spec.CacheSpec != nil {
			specHash, hashErr = hash.SpecHash(hash.ResourceTypeCache, *spec.CacheSpec)
		}
	case "object_storage":
		if spec.ObjectStorageSpec != nil {
			specHash, hashErr = hash.SpecHash(hash.ResourceTypeObjectStorage, *spec.ObjectStorageSpec)
		}
	case "compute":
		if spec.ComputeSpec != nil {
			specHash, hashErr = hash.SpecHash(hash.ResourceTypeCompute, *spec.ComputeSpec)
		}
	default:
		hashErr = hash.ErrUnknownResourceType
	}
	if hashErr != nil {
		return ResourcePlan{}, fmt.Errorf("failed to compute spec hash: %w", hashErr)
	}

	// Check existing resource (use envID and resourceName, not full ID)
	existing, err := p.store.GetResource(ctx, envID, resourceName)
	if err != nil {
		return ResourcePlan{}, fmt.Errorf("failed to get existing resource: %w", err)
	}

	plan := ResourcePlan{
		Spec:      spec,
		NewHash:   specHash,
		Priority:  p.calculatePriority(spec.Type),
		DependsOn: p.calculateDependencies(spec),
	}

	if existing == nil {
		// Resource doesn't exist - create
		plan.Action = "create"
	} else if existing.SpecHash != specHash {
		// Resource exists but spec changed - update
		plan.Action = "update"
		plan.OldHash = existing.SpecHash
	} else {
		// Resource exists and spec unchanged - noop
		plan.Action = "noop"
		plan.OldHash = existing.SpecHash
	}

	return plan, nil
}

// Apply applies a provisioning plan
func (p *Provisioner) Apply(ctx context.Context, envID string, plan *PlanResult) (*ApplyResult, error) {
	result := &ApplyResult{
		Resources: make([]ResourceResult, 0, len(plan.Resources)),
		Success:   true,
	}

	// Track completed resources for dependency checking
	completed := make(map[string]bool)

	for _, resourcePlan := range plan.Resources {
		// Skip noop resources
		if resourcePlan.Action == "noop" {
			result.Resources = append(result.Resources, ResourceResult{
				Name:    getResourceName(resourcePlan.Spec),
				Type:    resourcePlan.Spec.Type,
				Action:  "noop",
				Success: true,
			})
			completed[fmt.Sprintf("%s:%s", resourcePlan.Spec.Type, getResourceName(resourcePlan.Spec))] = true
			continue
		}

		// Check dependencies
		if err := p.checkDependencies(resourcePlan, completed); err != nil {
			result.Resources = append(result.Resources, ResourceResult{
				Name:     getResourceName(resourcePlan.Spec),
				Type:     resourcePlan.Spec.Type,
				Action:   resourcePlan.Action,
				Success:  false,
				ErrorMsg: err.Error(),
			})
			result.Success = false
			continue
		}

		// Apply the resource
		resResult := p.applyResource(ctx, envID, resourcePlan)
		result.Resources = append(result.Resources, resResult)

		if !resResult.Success {
			result.Success = false
		} else {
			completed[fmt.Sprintf("%s:%s", resourcePlan.Spec.Type, getResourceName(resourcePlan.Spec))] = true
		}
	}

	return result, nil
}

// applyResource applies a single resource plan
func (p *Provisioner) applyResource(ctx context.Context, envID string, plan ResourcePlan) ResourceResult {
	resourceID := fmt.Sprintf("%s:%s", plan.Spec.Type, getResourceName(plan.Spec))
	resName := getResourceName(plan.Spec)

	result := ResourceResult{
		Name:   resName,
		Type:   plan.Spec.Type,
		Action: plan.Action,
	}

	// Get provider
	provider, err := p.registry.Get("alicloud") // P0: only alicloud
	if err != nil {
		result.Success = false
		result.ErrorMsg = fmt.Sprintf("%s: provider not found: %v", ErrInvalidSpec, err)
		return result
	}

	// Wire SC-4 network reuse context into AliCloud provider.
	// This keeps VPC/VSwitch state persistent across runs.
	if aliProvider, ok := provider.(*alicloudprovider.Provider); ok {
		aliProvider.SetStateStore(p.store)
		aliProvider.SetEnvironment(envID)
	}

	// Create or update resource in state store
	resource := &state.InfraResource{
		ID:           resourceID,
		EnvID:        envID,
		ResourceName: resName,
		ResourceType: plan.Spec.Type,
		SpecHash:     plan.NewHash,
		Status:       string(ResourceStateProvisioning),
	}

	if err := p.store.UpsertResource(ctx, resource); err != nil {
		result.Success = false
		result.ErrorMsg = fmt.Sprintf("%s: failed to update state: %v", ErrConcurrencyConflict, err)
		return result
	}

	// Provision based on resource type
	var output interface{}
	switch plan.Spec.Type {
	case "database":
		if plan.Spec.DatabaseSpec != nil {
			dbOutput, err := provider.ProvisionDatabase(ctx, *plan.Spec.DatabaseSpec)
			if err != nil {
				result.Success = false
				result.ErrorMsg = fmt.Sprintf("%s: %v", ErrSDKRetryable, err)
				p.updateResourceStatus(ctx, resource, ResourceStateFailed, result.ErrorMsg)
				return result
			}
			output = dbOutput
		}
	case "cache":
		if plan.Spec.CacheSpec != nil {
			cacheOutput, err := provider.ProvisionCache(ctx, *plan.Spec.CacheSpec)
			if err != nil {
				result.Success = false
				result.ErrorMsg = fmt.Sprintf("%s: %v", ErrSDKRetryable, err)
				p.updateResourceStatus(ctx, resource, ResourceStateFailed, result.ErrorMsg)
				return result
			}
			output = cacheOutput
		}
	case "object_storage":
		if plan.Spec.ObjectStorageSpec != nil {
			objOutput, err := provider.ProvisionObjectStorage(ctx, *plan.Spec.ObjectStorageSpec)
			if err != nil {
				result.Success = false
				result.ErrorMsg = fmt.Sprintf("%s: %v", ErrSDKRetryable, err)
				p.updateResourceStatus(ctx, resource, ResourceStateFailed, result.ErrorMsg)
				return result
			}
			output = objOutput
		}
	case "compute":
		if plan.Spec.ComputeSpec != nil {
			compOutput, err := provider.ProvisionCompute(ctx, *plan.Spec.ComputeSpec)
			if err != nil {
				result.Success = false
				result.ErrorMsg = fmt.Sprintf("%s: %v", ErrSDKRetryable, err)
				p.updateResourceStatus(ctx, resource, ResourceStateFailed, result.ErrorMsg)
				return result
			}
			output = compOutput
		}
	default:
		result.Success = false
		result.ErrorMsg = fmt.Sprintf("%s: unknown resource type: %s", ErrInvalidResourceType, plan.Spec.Type)
		p.updateResourceStatus(ctx, resource, ResourceStateFailed, result.ErrorMsg)
		return result
	}

	// Update resource status to provisioned, persist endpoint output
	resource.Status = string(ResourceStateProvisioned)
	if output != nil {
		if outputJSON, err := json.Marshal(output); err == nil {
			resource.ConfigJSON = string(outputJSON)
		}
	}
	if err := p.store.UpsertResource(ctx, resource); err != nil {
		result.Success = false
		result.ErrorMsg = fmt.Sprintf("%s: failed to update final state: %v", ErrConcurrencyConflict, err)
		return result
	}

	result.Success = true
	result.Output = output
	return result
}

// updateResourceStatus updates the status of a resource
func (p *Provisioner) updateResourceStatus(ctx context.Context, resource *state.InfraResource, status ResourceState, errMsg string) {
	resource.Status = string(status)
	resource.ErrorMsg = errMsg
	p.store.UpsertResource(ctx, resource)
}

// Destroy destroys all resources in an environment
func (p *Provisioner) Destroy(ctx context.Context, envID string) error {
	// Get all resources for environment
	resources, err := p.store.ListResourcesByEnv(ctx, envID)
	if err != nil {
		return fmt.Errorf("%w: failed to list resources: %v", ErrDestroyFailed, err)
	}

	// Delete in reverse priority order
	sort.Slice(resources, func(i, j int) bool {
		return p.calculatePriority(resources[i].ResourceType) > p.calculatePriority(resources[j].ResourceType)
	})

	for _, resource := range resources {
		// Idempotent: skip already deleted resources
		if resource.Status == string(ResourceStateDeleted) {
			continue
		}

		// Update status to deleting
		resource.Status = string(ResourceStateDeleting)
		if err := p.store.UpsertResource(ctx, resource); err != nil {
			return fmt.Errorf("%w: failed to update state for %s: %v", ErrDestroyFailed, resource.ID, err)
		}

		// Get provider
		provider, err := p.registry.Get("alicloud")
		if err != nil {
			return fmt.Errorf("%w: provider not found: %v", ErrDestroyFailed, err)
		}

		// Destroy resource using type:providerID format
		destroyID := resource.ResourceType + ":" + resource.ProviderResourceID
		if err := provider.Destroy(ctx, destroyID); err != nil {
			resource.Status = string(ResourceStateFailed)
			resource.ErrorMsg = err.Error()
			p.store.UpsertResource(ctx, resource)
			return fmt.Errorf("%w: failed to destroy %s: %v", ErrDestroyFailed, resource.ID, err)
		}

		// Mark as deleted
		resource.Status = string(ResourceStateDeleted)
		if err := p.store.UpsertResource(ctx, resource); err != nil {
			return fmt.Errorf("%w: failed to update deleted state: %v", ErrDestroyFailed, err)
		}
	}

	return nil
}

// checkDependencies checks if all dependencies are completed
func (p *Provisioner) checkDependencies(plan ResourcePlan, completed map[string]bool) error {
	for _, dep := range plan.DependsOn {
		if !completed[dep] {
			return fmt.Errorf("%w: dependency %s not ready", ErrDependencyNotMet, dep)
		}
	}
	return nil
}

// calculatePriority determines provisioning priority (lower = earlier)
func (p *Provisioner) calculatePriority(resourceType string) int {
	switch resourceType {
	case "database":
		return 1
	case "cache":
		return 2
	case "object_storage":
		return 3
	case "compute":
		return 4
	default:
		return 10
	}
}

// calculateDependencies extracts dependencies from resource spec
func (p *Provisioner) calculateDependencies(spec providers.ResourceSpec) []string {
	var deps []string
	// Compute depends on database, cache, and object_storage if present
	if spec.Type == "compute" {
		// These would be populated from the spec in a real implementation
		// For now, return empty - dependencies are explicit in spec
	}
	return deps
}

// calculateSummary computes the summary counts from resource results (B1-R1)
func (p *Provisioner) calculateSummary(resources []ResourceResult) ProvisionSummary {
	summary := ProvisionSummary{
		Total: len(resources),
	}
	for _, res := range resources {
		switch res.Action {
		case "create":
			if res.Success {
				summary.Created++
			} else {
				summary.Failed++
			}
		case "update":
			if res.Success {
				summary.Updated++
			} else {
				summary.Failed++
			}
		case "noop":
			summary.Skipped++
		case "delete":
			if !res.Success {
				summary.Failed++
			}
		}
	}
	return summary
}

// getResourceName extracts the resource name from a spec
func getResourceName(spec providers.ResourceSpec) string {
	switch spec.Type {
	case "database":
		if spec.DatabaseSpec != nil {
			return spec.DatabaseSpec.Name
		}
	case "cache":
		if spec.CacheSpec != nil {
			return spec.CacheSpec.Name
		}
	case "object_storage":
		if spec.ObjectStorageSpec != nil {
			return spec.ObjectStorageSpec.Name
		}
	case "compute":
		if spec.ComputeSpec != nil {
			return spec.ComputeSpec.ServiceName
		}
	}
	return "unknown"
}
