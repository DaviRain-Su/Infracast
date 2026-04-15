// Package hash provides spec hashing utilities using canonical JSON
package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"

	"github.com/DaviRain-Su/infracast/providers"
)

var (
	// ErrUnknownResourceType is returned when resource type is not supported
	ErrUnknownResourceType = errors.New("unknown resource type")
)

// ResourceType represents the type of resource
type ResourceType string

const (
	ResourceTypeDatabase      ResourceType = "database"
	ResourceTypeCache         ResourceType = "cache"
	ResourceTypeObjectStorage ResourceType = "object_storage"
	ResourceTypeCompute       ResourceType = "compute"
)

// SpecHash generates a canonical SHA-256 hash for any resource spec
// Uses canonical JSON: fields → map[string]interface{} → sort keys → json.Marshal → SHA-256
func SpecHash(resourceType ResourceType, spec interface{}) (string, error) {
	switch resourceType {
	case ResourceTypeDatabase:
		s, ok := spec.(providers.DatabaseSpec)
		if !ok {
			return "", ErrUnknownResourceType
		}
		return specHashDatabase(s), nil

	case ResourceTypeCache:
		s, ok := spec.(providers.CacheSpec)
		if !ok {
			return "", ErrUnknownResourceType
		}
		return specHashCache(s), nil

	case ResourceTypeObjectStorage:
		s, ok := spec.(providers.ObjectStorageSpec)
		if !ok {
			return "", ErrUnknownResourceType
		}
		return specHashObjectStorage(s), nil

	case ResourceTypeCompute:
		s, ok := spec.(providers.ComputeSpec)
		if !ok {
			return "", ErrUnknownResourceType
		}
		return specHashCompute(s), nil

	default:
		return "", ErrUnknownResourceType
	}
}

// specHashDatabase generates canonical hash for database spec
func specHashDatabase(spec providers.DatabaseSpec) string {
	// Build map with canonical field order (alphabetical)
	// Handle *bool for HighAvail - use nil if nil, otherwise dereference
	highAvail := interface{}(nil)
	if spec.HighAvail != nil {
		highAvail = *spec.HighAvail
	}

	m := map[string]interface{}{
		"engine":         spec.Engine,
		"high_avail":     highAvail,
		"instance_class": spec.InstanceClass,
		"storage_gb":     spec.StorageGB,
		"version":        spec.Version,
	}
	return canonicalHash(m)
}

// specHashCache generates canonical hash for cache spec
func specHashCache(spec providers.CacheSpec) string {
	m := map[string]interface{}{
		"engine":          spec.Engine,
		"eviction_policy": spec.EvictionPolicy,
		"memory_mb":       spec.MemoryMB,
		"version":         spec.Version,
	}
	return canonicalHash(m)
}

// specHashObjectStorage generates canonical hash for object storage spec
func specHashObjectStorage(spec providers.ObjectStorageSpec) string {
	m := map[string]interface{}{
		"acl": spec.ACL,
	}

	// Sort CORS rules for determinism
	if len(spec.CORSRules) > 0 {
		rules := make([]map[string]interface{}, len(spec.CORSRules))
		for i, rule := range spec.CORSRules {
			rules[i] = map[string]interface{}{
				"allowed_headers": sortStrings(rule.AllowedHeaders),
				"allowed_methods": sortStrings(rule.AllowedMethods),
				"allowed_origins": sortStrings(rule.AllowedOrigins),
			}
		}
		m["cors_rules"] = rules
	}

	return canonicalHash(m)
}

// specHashCompute generates canonical hash for compute spec
func specHashCompute(spec providers.ComputeSpec) string {
	m := map[string]interface{}{
		"cpu":      spec.CPU,
		"memory":   spec.Memory,
		"port":     spec.Port,
		"replicas": spec.Replicas,
	}

	// Sort env_vars by key for determinism
	if len(spec.EnvVars) > 0 {
		keys := make([]string, 0, len(spec.EnvVars))
		for k := range spec.EnvVars {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		envVars := make([]map[string]string, 0, len(keys))
		for _, k := range keys {
			envVars = append(envVars, map[string]string{
				"key":   k,
				"value": spec.EnvVars[k],
			})
		}
		m["env_vars"] = envVars
	}

	// Sort secret_refs for determinism
	if len(spec.SecretRefs) > 0 {
		m["secret_refs"] = sortStrings(spec.SecretRefs)
	}

	return canonicalHash(m)
}

// canonicalHash produces SHA-256 hash of canonical JSON
func canonicalHash(m map[string]interface{}) string {
	// Marshal to canonical JSON (Go's json.Marshal sorts map keys alphabetically)
	data, err := json.Marshal(m)
	if err != nil {
		// This should never happen with our maps
		return ""
	}

	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// sortStrings returns a sorted copy of strings
func sortStrings(s []string) []string {
	result := make([]string, len(s))
	copy(result, s)
	sort.Strings(result)
	return result
}

// SpecHashResource is a convenience wrapper that extracts type from ResourceSpec
func SpecHashResource(spec providers.ResourceSpec) (string, error) {
	switch spec.Type {
	case "database":
		if spec.DatabaseSpec == nil {
			return "", ErrUnknownResourceType
		}
		return SpecHash(ResourceTypeDatabase, *spec.DatabaseSpec)
	case "cache":
		if spec.CacheSpec == nil {
			return "", ErrUnknownResourceType
		}
		return SpecHash(ResourceTypeCache, *spec.CacheSpec)
	case "object_storage":
		if spec.ObjectStorageSpec == nil {
			return "", ErrUnknownResourceType
		}
		return SpecHash(ResourceTypeObjectStorage, *spec.ObjectStorageSpec)
	case "compute":
		if spec.ComputeSpec == nil {
			return "", ErrUnknownResourceType
		}
		return SpecHash(ResourceTypeCompute, *spec.ComputeSpec)
	default:
		return "", ErrUnknownResourceType
	}
}
