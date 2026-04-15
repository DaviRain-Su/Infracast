// Package integration provides end-to-end integration tests
package integration

import (
	"context"
	"fmt"
	"sync"

	"github.com/DaviRain-Su/infracast/providers"
)

// MockCloudProvider implements a full mock cloud provider for integration testing
type MockCloudProvider struct {
	name        string
	shouldError bool
	errorType   string // "retryable" or "non-retryable"

	// Track provisioned resources
	mu        sync.RWMutex
	databases map[string]*providers.DatabaseOutput
	caches    map[string]*providers.CacheOutput
	storage   map[string]*providers.ObjectStorageOutput
	compute   map[string]*providers.ComputeOutput
}

// NewMockCloudProvider creates a new mock provider
// Note: name should be "alicloud" for P0 compatibility
func NewMockCloudProvider(name string) *MockCloudProvider {
	if name == "" {
		name = "alicloud"
	}
	return &MockCloudProvider{
		name:      name,
		databases: make(map[string]*providers.DatabaseOutput),
		caches:    make(map[string]*providers.CacheOutput),
		storage:   make(map[string]*providers.ObjectStorageOutput),
		compute:   make(map[string]*providers.ComputeOutput),
	}
}

// SetError configures the mock to return errors
func (m *MockCloudProvider) SetError(shouldError bool, errorType string) {
	m.shouldError = shouldError
	m.errorType = errorType
}

// Name returns provider name
func (m *MockCloudProvider) Name() string { return m.name }

// DisplayName returns display name
func (m *MockCloudProvider) DisplayName() string { return "Mock " + m.name }

// Regions returns available regions
func (m *MockCloudProvider) Regions() []providers.Region {
	return []providers.Region{
		{ID: "cn-hangzhou", Name: "Hangzhou", DisplayName: "China (Hangzhou)"},
		{ID: "cn-shanghai", Name: "Shanghai", DisplayName: "China (Shanghai)"},
	}
}

// ProvisionDatabase creates a mock database
func (m *MockCloudProvider) ProvisionDatabase(ctx context.Context, spec providers.DatabaseSpec) (*providers.DatabaseOutput, error) {
	if m.shouldError && m.errorType == "retryable" {
		return nil, fmt.Errorf("retryable error: connection timeout")
	}
	if m.shouldError && m.errorType == "non-retryable" {
		return nil, fmt.Errorf("non-retryable error: invalid configuration")
	}

	output := &providers.DatabaseOutput{
		ResourceID: fmt.Sprintf("db-%s-%s", m.name, spec.Name),
		Endpoint:   fmt.Sprintf("%s-db.example.com", spec.Name),
		Port:       5432,
		Username:   "app",
		Password:   fmt.Sprintf("${%s_DB_PASSWORD}", spec.Name),
	}

	m.mu.Lock()
	m.databases[spec.Name] = output
	m.mu.Unlock()

	return output, nil
}

// ProvisionCache creates a mock cache
func (m *MockCloudProvider) ProvisionCache(ctx context.Context, spec providers.CacheSpec) (*providers.CacheOutput, error) {
	if m.shouldError && m.errorType == "retryable" {
		return nil, fmt.Errorf("retryable error: connection timeout")
	}
	if m.shouldError && m.errorType == "non-retryable" {
		return nil, fmt.Errorf("non-retryable error: invalid cache configuration")
	}

	output := &providers.CacheOutput{
		ResourceID: fmt.Sprintf("cache-%s-%s", m.name, spec.Name),
		Endpoint:   fmt.Sprintf("%s-cache.example.com", spec.Name),
		Port:       6379,
		Password:   fmt.Sprintf("${%s_CACHE_PASSWORD}", spec.Name),
	}

	m.mu.Lock()
	m.caches[spec.Name] = output
	m.mu.Unlock()

	return output, nil
}

// ProvisionObjectStorage creates a mock object storage
func (m *MockCloudProvider) ProvisionObjectStorage(ctx context.Context, spec providers.ObjectStorageSpec) (*providers.ObjectStorageOutput, error) {
	if m.shouldError && m.errorType == "retryable" {
		return nil, fmt.Errorf("retryable error: connection timeout")
	}

	output := &providers.ObjectStorageOutput{
		ResourceID: fmt.Sprintf("oss-%s-%s", m.name, spec.Name),
		BucketName: spec.Name,
		Endpoint:   "https://oss-cn-hangzhou.aliyuncs.com",
		Region:     "cn-hangzhou",
	}

	m.mu.Lock()
	m.storage[spec.Name] = output
	m.mu.Unlock()

	return output, nil
}

// ProvisionCompute creates a mock compute resource
func (m *MockCloudProvider) ProvisionCompute(ctx context.Context, spec providers.ComputeSpec) (*providers.ComputeOutput, error) {
	if m.shouldError && m.errorType == "retryable" {
		return nil, fmt.Errorf("retryable error: connection timeout")
	}

	output := &providers.ComputeOutput{
		ResourceID:     fmt.Sprintf("k8s-%s-%s", m.name, spec.ServiceName),
		Namespace:      "default",
		ServiceName:    spec.ServiceName,
		DeploymentName: spec.ServiceName,
	}

	m.mu.Lock()
	m.compute[spec.ServiceName] = output
	m.mu.Unlock()

	return output, nil
}

// Plan returns empty plan (mock)
func (m *MockCloudProvider) Plan(ctx context.Context, specs []providers.ResourceSpec) (*providers.PlanResult, error) {
	return &providers.PlanResult{}, nil
}

// Apply returns empty result (mock)
func (m *MockCloudProvider) Apply(ctx context.Context, plan *providers.PlanResult) (*providers.ApplyResult, error) {
	return &providers.ApplyResult{}, nil
}

// Destroy simulates destruction
func (m *MockCloudProvider) Destroy(ctx context.Context, envID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.databases = make(map[string]*providers.DatabaseOutput)
	m.caches = make(map[string]*providers.CacheOutput)
	m.storage = make(map[string]*providers.ObjectStorageOutput)
	m.compute = make(map[string]*providers.ComputeOutput)

	return nil
}

// OTLPEndpoint returns empty (mock)
func (m *MockCloudProvider) OTLPEndpoint() string { return "" }

// DashboardURL returns empty (mock)
func (m *MockCloudProvider) DashboardURL(envID string) string { return "" }

// GetDatabase retrieves a provisioned database (for verification)
func (m *MockCloudProvider) GetDatabase(name string) (*providers.DatabaseOutput, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	db, ok := m.databases[name]
	return db, ok
}

// GetCache retrieves a provisioned cache (for verification)
func (m *MockCloudProvider) GetCache(name string) (*providers.CacheOutput, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cache, ok := m.caches[name]
	return cache, ok
}
