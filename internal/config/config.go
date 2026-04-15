// Package config handles infracast.yaml parsing and validation
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the root configuration
type Config struct {
	Provider     string                 `yaml:"provider"`
	Region       string                 `yaml:"region"`
	Environments map[string]Environment `yaml:"environments,omitempty"`
	Overrides    Overrides              `yaml:"overrides,omitempty"`
}

// Environment represents environment-specific configuration
type Environment struct {
	Provider string `yaml:"provider"`
	Region   string `yaml:"region"`
}

// Overrides represents resource-specific overrides
type Overrides struct {
	Databases     map[string]DatabaseOverride     `yaml:"databases,omitempty"`
	Cache         map[string]CacheOverride        `yaml:"cache,omitempty"`
	ObjectStorage map[string]ObjectStorageOverride `yaml:"object_storage,omitempty"`
	Compute       map[string]ComputeOverride      `yaml:"compute,omitempty"`
}

// DatabaseOverride represents database configuration overrides
type DatabaseOverride struct {
	Engine        string `yaml:"engine,omitempty"`
	Version       string `yaml:"version,omitempty"`
	InstanceClass string `yaml:"instance_class,omitempty"`
	StorageGB     int    `yaml:"storage_gb,omitempty"`
	HighAvail     *bool  `yaml:"high_avail,omitempty"`
}

// ComputeOverride represents compute configuration overrides
type ComputeOverride struct {
	Replicas int    `yaml:"replicas,omitempty"`
	CPU      string `yaml:"cpu,omitempty"`
	Memory   string `yaml:"memory,omitempty"`
}

// CacheOverride represents cache configuration overrides
type CacheOverride struct {
	Engine         string `yaml:"engine,omitempty"`
	Version        string `yaml:"version,omitempty"`
	MemoryMB       int    `yaml:"memory_mb,omitempty"`
	EvictionPolicy string `yaml:"eviction_policy,omitempty"`
}

// ObjectStorageOverride represents object storage configuration overrides
type ObjectStorageOverride struct {
	ACL string `yaml:"acl,omitempty"`
}

// Load reads configuration from file
func Load(path string) (*Config, error) {
	if path == "" {
		path = "infracast.yaml"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return &cfg, nil
}

// ResolvedEnv represents a fully resolved environment configuration
type ResolvedEnv struct {
	Name     string
	Provider string
	Region   string
}

// ResolveEnv resolves an environment configuration with full validation
func (c *Config) ResolveEnv(envName string) (*ResolvedEnv, error) {
	if envName == "" {
		return nil, ErrMissingEnvName
	}

	// Get environment (with fallback to defaults)
	env, err := c.GetEnvironment(envName)
	if err != nil {
		return nil, err
	}

	// Validate resolved values
	if env.Provider == "" {
		return nil, ErrMissingProvider
	}
	if env.Region == "" {
		return nil, ErrMissingRegion
	}

	// Validate region format (e.g., cn-hangzhou, cn-shanghai)
	if !isValidRegionFormat(env.Region) {
		return nil, ErrInvalidRegionFormat
	}

	return &ResolvedEnv{
		Name:     envName,
		Provider: env.Provider,
		Region:   env.Region,
	}, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Provider == "" {
		return ErrMissingProvider
	}

	// Only alicloud is supported in P0
	if c.Provider != "alicloud" {
		return ErrUnsupportedProvider
	}

	if c.Region == "" {
		return ErrMissingRegion
	}

	// Validate region format
	if !isValidRegionFormat(c.Region) {
		return ErrInvalidRegionFormat
	}

	// Validate environment names
	for name := range c.Environments {
		// Check max length first
		if len(name) > 50 {
			return ErrInvalidEnvNameLength
		}
		if !isValidEnvironmentName(name) {
			return ErrInvalidEnvName
		}
	}

	// Validate overrides
	if err := c.validateOverrides(); err != nil {
		return err
	}

	return nil
}

// isValidRegionFormat validates region format (e.g., cn-hangzhou, us-west-1)
func isValidRegionFormat(region string) bool {
	// Simple validation: must contain at least one hyphen and only lowercase letters
	if len(region) < 3 {
		return false
	}
	hasHyphen := false
	for _, c := range region {
		if c == '-' {
			hasHyphen = true
		} else if !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') {
			return false
		}
	}
	return hasHyphen
}

// validateOverrides validates all override values
func (c *Config) validateOverrides() error {
	// Validate database overrides
	for name, override := range c.Overrides.Databases {
		if override.StorageGB > 0 {
			if override.StorageGB < 20 || override.StorageGB > 32768 {
				return fmt.Errorf("%w: database %s storage_gb=%d", ErrInvalidStorageGB, name, override.StorageGB)
			}
		}
		// Validate engine if specified
		if override.Engine != "" && override.Engine != "mysql" && override.Engine != "postgresql" {
			return fmt.Errorf("%w: database %s engine=%s", ErrInvalidEngine, name, override.Engine)
		}
	}

	// Validate cache overrides
	for name, override := range c.Overrides.Cache {
		if override.MemoryMB > 0 {
			if override.MemoryMB < 256 || override.MemoryMB > 65536 {
				return fmt.Errorf("%w: cache %s memory_mb=%d", ErrInvalidMemoryMB, name, override.MemoryMB)
			}
		}
		// Validate engine if specified
		if override.Engine != "" && override.Engine != "redis" {
			return fmt.Errorf("%w: cache %s engine=%s", ErrInvalidCacheEngine, name, override.Engine)
		}
	}

	// Validate compute overrides
	for name, override := range c.Overrides.Compute {
		if override.Replicas > 0 {
			if override.Replicas < 1 || override.Replicas > 100 {
				return fmt.Errorf("%w: compute %s replicas=%d", ErrInvalidReplicas, name, override.Replicas)
			}
		}
		// Validate CPU format (e.g., "1000m", "2")
		if override.CPU != "" && !isValidCPUFormat(override.CPU) {
			return fmt.Errorf("%w: compute %s cpu=%s", ErrInvalidCPUFormat, name, override.CPU)
		}
		// Validate memory format (e.g., "512Mi", "1Gi")
		if override.Memory != "" && !isValidMemoryFormat(override.Memory) {
			return fmt.Errorf("%w: compute %s memory=%s", ErrInvalidMemoryFormat, name, override.Memory)
		}
	}

	return nil
}

// isValidCPUFormat validates CPU format (e.g., "1000m", "2", "500m")
func isValidCPUFormat(cpu string) bool {
	if cpu == "" {
		return false
	}
	// Must end with 'm' or be a plain number
	// Must have at least one digit
	hasDigit := false
	for i, c := range cpu {
		if c == 'm' {
			if i != len(cpu)-1 {
				return false // 'm' must be at the end
			}
		} else if c < '0' || c > '9' {
			return false
		} else {
			hasDigit = true
		}
	}
	// Must have at least one digit
	return hasDigit
}

// isValidMemoryFormat validates memory format (e.g., "512Mi", "1Gi", "256Mi")
func isValidMemoryFormat(memory string) bool {
	if memory == "" {
		return false
	}
	// Must end with 'Mi' or 'Gi'
	if len(memory) < 3 {
		return false
	}
	suffix := memory[len(memory)-2:]
	if suffix != "Mi" && suffix != "Gi" {
		return false
	}
	// Prefix must be numeric
	prefix := memory[:len(memory)-2]
	for _, c := range prefix {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// isValidEnvironmentName checks if the name is valid (lowercase alphanumeric + hyphen)
func isValidEnvironmentName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		// Only lowercase letters, numbers, and hyphens allowed
		if !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') && c != '-' {
			return false
		}
	}
	return true
}

// Save writes configuration to file
func (c *Config) Save(path string) error {
	if path == "" {
		path = "infracast.yaml"
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// GetEnvironment retrieves an environment configuration with fallback to defaults
func (c *Config) GetEnvironment(name string) (Environment, error) {
	if env, exists := c.Environments[name]; exists {
		// Merge with defaults: use environment values, fallback to root config
		if env.Provider == "" {
			env.Provider = c.Provider
		}
		if env.Region == "" {
			env.Region = c.Region
		}
		return env, nil
	}

	// Return default environment based on root config
	return Environment{
		Provider: c.Provider,
		Region:   c.Region,
	}, nil
}

// MergeWithDefaults returns a new Config with all environments merged with root defaults
func (c *Config) MergeWithDefaults() *Config {
	merged := &Config{
		Provider:     c.Provider,
		Region:       c.Region,
		Environments: make(map[string]Environment),
		Overrides:    c.Overrides,
	}

	for name, env := range c.Environments {
		if env.Provider == "" {
			env.Provider = c.Provider
		}
		if env.Region == "" {
			env.Region = c.Region
		}
		merged.Environments[name] = env
	}

	return merged
}

// GetDatabaseOverride retrieves a database override if it exists
func (c *Config) GetDatabaseOverride(name string) (DatabaseOverride, bool) {
	if c.Overrides.Databases == nil {
		return DatabaseOverride{}, false
	}
	override, exists := c.Overrides.Databases[name]
	return override, exists
}

// GetComputeOverride retrieves a compute override if it exists
func (c *Config) GetComputeOverride(name string) (ComputeOverride, bool) {
	if c.Overrides.Compute == nil {
		return ComputeOverride{}, false
	}
	override, exists := c.Overrides.Compute[name]
	return override, exists
}

// GetCacheOverride retrieves a cache override if it exists
func (c *Config) GetCacheOverride(name string) (CacheOverride, bool) {
	if c.Overrides.Cache == nil {
		return CacheOverride{}, false
	}
	override, exists := c.Overrides.Cache[name]
	return override, exists
}

// GetObjectStorageOverride retrieves an object storage override if it exists
func (c *Config) GetObjectStorageOverride(name string) (ObjectStorageOverride, bool) {
	if c.Overrides.ObjectStorage == nil {
		return ObjectStorageOverride{}, false
	}
	override, exists := c.Overrides.ObjectStorage[name]
	return override, exists
}
