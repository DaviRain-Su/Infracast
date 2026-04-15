// Package mapper provides service mapping from build metadata to resource specs
package mapper

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/DaviRain-Su/infracast/providers"
)

// BuildMeta represents build-time metadata extracted from the application
// This replaces Encore IR dependency for P0
type BuildMeta struct {
	AppName      string   `json:"app_name"`
	Services     []string `json:"services"`
	Databases    []string `json:"databases"`
	Caches       []string `json:"caches"`
	ObjectStores []string `json:"object_stores"`
	PubSubTopics []string `json:"pubsub_topics"` // P1 placeholder
	BuildCommit  string   `json:"build_commit"`
	BuildImage   string   `json:"build_image"`
}

// ResourceDeclaration represents a discovered resource declaration in source code
type ResourceDeclaration struct {
	Type     string // "database", "cache", "object_storage"
	Name     string
	Location string // file path
	Line     int
}

// MappedResource represents a resource with DAG ordering information
type MappedResource struct {
	providers.ResourceSpec
	Priority  int      // Lower = earlier in provisioning order
	DependsOn []string // Resource dependencies (e.g., ["database:users"])
}

// Mapper maps build metadata to resource specifications
type Mapper struct {
	registry *providers.Registry
}

// NewMapper creates a new service mapper
func NewMapper(registry *providers.Registry) *Mapper {
	return &Mapper{registry: registry}
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
		StorageGB:     20,  // Default per Tech Spec v1.1
		HighAvail:     nil, // Default: no HA preference
	}

	// Apply overrides from config
	// TODO: Implement config override support with Registry

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
		MemoryMB:       256, // Default per Tech Spec v1.1
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
	// Use registry info for app name
	providers := m.registry.List()
	appName := "infracast"
	if len(providers) > 0 {
		appName = providers[0]
	}
	return BuildMeta{
		AppName:  appName,
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

// ScanSources scans source code for Encore resource declarations
// Uses regex patterns to find //encore:api, SQLDatabase, etc.
func (m *Mapper) ScanSources(sourceDir string) ([]ResourceDeclaration, error) {
	var declarations []ResourceDeclaration

	// Regex patterns for different resource types
	// Match patterns like: Name: "users" or Name: 'users'
	patterns := map[string]*regexp.Regexp{
		"database":       regexp.MustCompile(`SQLDatabase.*Name\s*:\s*["'](\w+)["']`),
		"cache":          regexp.MustCompile(`Cache.*Name\s*:\s*["'](\w+)["']`),
		"object_storage": regexp.MustCompile(`ObjectStorage.*Name\s*:\s*["'](\w+)["']`),
	}

	// Walk the source directory
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			for resType, pattern := range patterns {
				matches := pattern.FindStringSubmatch(line)
				if len(matches) > 1 {
					declarations = append(declarations, ResourceDeclaration{
						Type:     resType,
						Name:     matches[1],
						Location: path,
						Line:     lineNum,
					})
				}
			}
		}

		return scanner.Err()
	})

	if err != nil {
		return nil, fmt.Errorf("EMAP003: failed to scan sources: %w", err)
	}

	return declarations, nil
}

// MapToMappedResources converts resource specs to MappedResource with DAG info
func (m *Mapper) MapToMappedResources(specs []providers.ResourceSpec) []MappedResource {
	var mapped []MappedResource

	for _, spec := range specs {
		mr := MappedResource{
			ResourceSpec: spec,
			Priority:     m.calculatePriority(spec.Type),
			DependsOn:    m.calculateDependencies(spec),
		}
		mapped = append(mapped, mr)
	}

	return mapped
}

// calculatePriority determines provisioning priority (lower = earlier)
// Order: databases (1) → caches (2) → object_storage (3)
func (m *Mapper) calculatePriority(resourceType string) int {
	switch resourceType {
	case "database":
		return 1
	case "cache":
		return 2
	case "object_storage":
		return 3
	default:
		return 10
	}
}

// calculateDependencies extracts dependencies from resource spec
func (m *Mapper) calculateDependencies(spec providers.ResourceSpec) []string {
	// For now, services depend on databases and caches
	// This can be extended based on actual service code analysis
	return []string{}
}
