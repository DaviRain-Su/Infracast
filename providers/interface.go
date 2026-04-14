// Package providers defines the cloud provider interface
package providers

import (
	"context"
	"fmt"
)

// CloudProviderInterface defines the interface for cloud provider adapters
type CloudProviderInterface interface {
	// Basic info
	Name() string
	DisplayName() string
	Regions() []Region

	// P0 Resources Only
	ProvisionDatabase(ctx context.Context, spec DatabaseSpec) (*DatabaseOutput, error)
	ProvisionCache(ctx context.Context, spec CacheSpec) (*CacheOutput, error)
	ProvisionObjectStorage(ctx context.Context, spec ObjectStorageSpec) (*ObjectStorageOutput, error)
	ProvisionCompute(ctx context.Context, spec ComputeSpec) (*ComputeOutput, error)

	// Lifecycle
	Plan(ctx context.Context, specs []ResourceSpec) (*PlanResult, error)
	Apply(ctx context.Context, plan *PlanResult) (*ApplyResult, error)
	Destroy(ctx context.Context, envID string) error

	// Observability
	OTLPEndpoint() string
	DashboardURL(envID string) string
}

// Region represents a cloud region
type Region struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

// ResourceSpec is a union type for all resource specifications
type ResourceSpec struct {
	Type            string                `json:"type"`
	DatabaseSpec    *DatabaseSpec         `json:"database_spec,omitempty"`
	CacheSpec       *CacheSpec            `json:"cache_spec,omitempty"`
	ObjectStorageSpec *ObjectStorageSpec  `json:"object_storage_spec,omitempty"`
	ComputeSpec     *ComputeSpec          `json:"compute_spec,omitempty"`
}

// DatabaseSpec represents database resource specification
type DatabaseSpec struct {
	Name          string   `json:"name"`
	Engine        string   `json:"engine"` // mysql, postgresql
	Version       string   `json:"version"`
	InstanceClass string   `json:"instance_class"`
	StorageGB     int      `json:"storage_gb"`
	HighAvail     bool     `json:"high_avail"`
}

// DatabaseOutput represents the created database resource
type DatabaseOutput struct {
	ResourceID string `json:"resource_id"`
	Endpoint   string `json:"endpoint"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"` // Reference to secret
}

// CacheSpec represents cache resource specification
type CacheSpec struct {
	Name           string `json:"name"`
	Engine         string `json:"engine"` // redis
	Version        string `json:"version"`
	MemoryMB       int    `json:"memory_mb"`
	EvictionPolicy string `json:"eviction_policy"`
}

// CacheOutput represents the created cache resource
type CacheOutput struct {
	ResourceID string `json:"resource_id"`
	Endpoint   string `json:"endpoint"`
	Port       int    `json:"port"`
	Password   string `json:"password"` // Reference to secret
}

// ObjectStorageSpec represents object storage resource specification
type ObjectStorageSpec struct {
	Name      string     `json:"name"`
	ACL       string     `json:"acl"` // private, public-read
	CORSRules []CORSRule `json:"cors_rules,omitempty"`
}

// CORSRule represents a CORS rule
type CORSRule struct {
	AllowedOrigins []string `json:"allowed_origins"`
	AllowedMethods []string `json:"allowed_methods"`
	AllowedHeaders []string `json:"allowed_headers"`
}

// ObjectStorageOutput represents the created object storage resource
type ObjectStorageOutput struct {
	ResourceID string `json:"resource_id"`
	BucketName string `json:"bucket_name"`
	Endpoint   string `json:"endpoint"`
	Region     string `json:"region"`
}

// ComputeSpec represents compute resource specification
type ComputeSpec struct {
	ServiceName string            `json:"service_name"`
	Replicas    int               `json:"replicas"`
	CPU         string            `json:"cpu"`
	Memory      string            `json:"memory"`
	Port        int               `json:"port"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
	SecretRefs  []string          `json:"secret_refs,omitempty"`
}

// ComputeOutput represents the created compute resource
type ComputeOutput struct {
	ResourceID    string `json:"resource_id"`
	Namespace     string `json:"namespace"`
	ServiceName   string `json:"service_name"`
	DeploymentName string `json:"deployment_name"`
}

// PlanResult represents the plan for infrastructure changes
type PlanResult struct {
	Resources []ResourcePlan `json:"resources"`
}

// ResourcePlan represents a single resource plan
type ResourcePlan struct {
	Action string       `json:"action"` // create, update, delete, noop
	Spec   ResourceSpec `json:"spec"`
}

// ApplyResult represents the result of applying a plan
type ApplyResult struct {
	Resources []ResourceOutput `json:"resources"`
}

// ResourceOutput represents the output of a resource operation
type ResourceOutput struct {
	Type       string      `json:"type"`
	Name       string      `json:"name"`
	ResourceID string      `json:"resource_id"`
	Output     interface{} `json:"output"`
}

// Registry manages cloud provider adapters
type Registry struct {
	providers map[string]CloudProviderInterface
}

// DefaultRegistry is the process-wide registry used by init-time provider registration.
var DefaultRegistry = NewRegistry()

// Register registers a provider into the default registry.
func Register(provider CloudProviderInterface) error {
	return DefaultRegistry.Register(provider)
}

// Get returns a provider from the default registry.
func Get(name string) (CloudProviderInterface, error) {
	return DefaultRegistry.Get(name)
}

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]CloudProviderInterface),
	}
}

// Register registers a cloud provider adapter
func (r *Registry) Register(provider CloudProviderInterface) error {
	name := provider.Name()
	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider %s already registered", name)
	}
	r.providers[name] = provider
	return nil
}

// Get retrieves a cloud provider adapter by name
func (r *Registry) Get(name string) (CloudProviderInterface, error) {
	provider, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", name)
	}
	return provider, nil
}

// List returns all registered provider names
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}
