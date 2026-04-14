// Package config handles infracast.yaml parsing and validation
package config

import "errors"

// Error codes for config module (ECFG001-ECFG020)
var (
	// ECFG001-003: Missing required fields
	ErrMissingProvider = errors.New("ECFG001: provider is required")
	ErrMissingRegion   = errors.New("ECFG002: region is required")
	ErrMissingEnvName  = errors.New("ECFG003: environment name is required")

	// ECFG004-006: Invalid values
	ErrUnsupportedProvider = errors.New("ECFG004: unsupported provider")
	ErrInvalidRegionFormat = errors.New("ECFG005: invalid region format")
	ErrInvalidEnvName      = errors.New("ECFG006: invalid environment name")

	// ECFG007-009: Override errors
	ErrInvalidStorageGB   = errors.New("ECFG007: storage_gb must be between 20 and 32768")
	ErrInvalidReplicas    = errors.New("ECFG008: replicas must be between 1 and 100")
	ErrInvalidCPUFormat   = errors.New("ECFG009: invalid CPU format")
	ErrInvalidMemoryFormat = errors.New("ECFG010: invalid memory format")
	ErrInvalidEngine      = errors.New("ECFG011: invalid database engine")
	ErrInvalidVersion     = errors.New("ECFG012: invalid database version")
	ErrInvalidInstanceClass = errors.New("ECFG013: invalid instance class")
	ErrInvalidMemoryMB    = errors.New("ECFG014: memory_mb must be between 256 and 65536")
	ErrInvalidCacheEngine = errors.New("ECFG015: invalid cache engine")
	ErrInvalidCacheVersion = errors.New("ECFG016: invalid cache version")
	ErrInvalidACL         = errors.New("ECFG017: invalid ACL value")
	ErrInvalidEvictionPolicy = errors.New("ECFG018: invalid eviction policy")
	ErrEnvironmentNotFound = errors.New("ECFG019: environment not found")
	ErrConfigLoadFailed    = errors.New("ECFG020: failed to load configuration")
	ErrInvalidEnvNameLength = errors.New("ECFG021: environment name exceeds 50 characters")
)
