// Package mapper provides service mapping from build metadata to resource specs
package mapper

import (
	"fmt"

	"github.com/DaviRain-Su/infracast/internal/config"
	"github.com/DaviRain-Su/infracast/providers"
)

// BuildMeta represents build-time metadata extracted from the application
// This replaces Encore IR dependency for P0
type BuildMeta struct {
	AppName        string   `json:"app_name"`
	Services       []string `json:"services"`
	Databases      []string `json:"databases"`
	Caches         []string `json:"caches"`
	ObjectStores   []string `json:"object_stores"`
	PubSubTopics   []string `json:"pubsub_topics"` // P1 placeholder
	BuildCommit    string   `json:"build_commit"`
	BuildImage     string   `json:"build_image"`
}

// Mapper maps build metadata to resource specifications
type Mapper struct {
	cfg *config.Config
}

// NewMapper creates a new service mapper
func NewMapper(cfg *config.Config) *Mapper {
	return &Mapper{cfg: cfg}
}

// MapToResourceSpecs converts build metadata to resource specs
func (m *Mapper) MapToResourceSpecs(meta BuildMeta) []providers.ResourceSpec {
	var specs []providers.ResourceSpec

	// Map databases
	for _, dbName := range meta.Databases {
		spec := m.mapDatabase(dbName)
		specs = append(specs, spec)
	}

	// Map caches
	for _, cacheName := range meta.Caches {
		spec := m.mapCache(cacheName)
		specs = append(specs, spec)
	}

	// Map object stores
	for _, bucketName := range meta.ObjectStores {
		spec := m.mapObjectStorage(bucketName)
		specs = append(specs, spec)
	}

	return specs
}

// mapDatabase creates a database resource spec with override support
func (m *Mapper) mapDatabase(name string) providers.ResourceSpec {
	spec := providers.DatabaseSpec{
		Name:          name,
		Engine:        "postgresql", // Default
		Version:       "15",         // Default
		InstanceClass: "rds.pg.s1.small",
		StorageGB:     50,
		HighAvail:     false,
	}

	// Apply overrides from config
	if override, exists := m.cfg.GetDatabaseOverride(name); exists {
		if override.InstanceClass != "" {
			spec.InstanceClass = override.InstanceClass
		}
		if override.StorageGB > 0 {
			spec.StorageGB = override.StorageGB
		}
		spec.HighAvail = override.HighAvail
	}

	return providers.ResourceSpec{
		Type:         "database",
		DatabaseSpec: &spec,
	}
}

// mapCache creates a cache resource spec
func (m *Mapper) mapCache(name string) providers.ResourceSpec {
	spec := providers.CacheSpec{
		Name:           name,
		Engine:         "redis",
		Version:        "7",
		MemoryMB:       1024,
		EvictionPolicy: "allkeys-lru",
	}

	return providers.ResourceSpec{
		Type:      "cache",
		CacheSpec: &spec,
	}
}

// mapObjectStorage creates an object storage resource spec
func (m *Mapper) mapObjectStorage(name string) providers.ResourceSpec {
	spec := providers.ObjectStorageSpec{
		Name: name,
		ACL:  "private",
	}

	return providers.ResourceSpec{
		Type:              "object_storage",
		ObjectStorageSpec: &spec,
	}
}

// ValidateBuildMeta validates build metadata
func (m *Mapper) ValidateBuildMeta(meta BuildMeta) error {
	if meta.AppName == "" {
		return fmt.Errorf("EMAP001: app_name is required")
	}
	if len(meta.Services) == 0 {
		return fmt.Errorf("EMAP002: at least one service is required")
	}
	return nil
}

// ExtractFromConfig extracts resource names from config (fallback method)
func (m *Mapper) ExtractFromConfig() BuildMeta {
	return BuildMeta{
		AppName:  m.cfg.Provider,
		Services: []string{"default"},
	}
}

// GetResourceDependencies returns resource dependencies for a service
func (m *Mapper) GetResourceDependencies(serviceName string, meta BuildMeta) ([]string, error) {
	// Simple implementation: return all resources
	var deps []string
	for _, db := range meta.Databases {
		deps = append(deps, fmt.Sprintf("database:%s", db))
	}
	for _, cache := range meta.Caches {
		deps = append(deps, fmt.Sprintf("cache:%s", cache))
	}
	return deps, nil
}
