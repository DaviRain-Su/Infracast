// Package infragen provides infrastructure configuration generation
package infragen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DaviRain-Su/infracast/internal/mapper"
	"github.com/DaviRain-Su/infracast/providers"
)

// Generator generates infrastructure configuration
type Generator struct {
	// Base configuration (optional)
	base *InfraConfig
}

// NewGenerator creates a new infrastructure config generator
func NewGenerator(base *InfraConfig) *Generator {
	return &Generator{base: base}
}

// Generate generates infrastructure configuration from provisioning outputs and build metadata
func (g *Generator) Generate(outputs []providers.ResourceOutput, meta mapper.BuildMeta) (*InfraConfig, error) {
	cfg := &InfraConfig{
		SQLServers:    make(map[string]SQLServer),
		Redis:         make(map[string]RedisServer),
		ObjectStorage: make(map[string]ObjectStore),
	}

	// Merge with base if provided
	if g.base != nil {
		cfg = g.Merge(cfg, g.base)
	}

	// Build lookup map from outputs
	outputMap := make(map[string]providers.ResourceOutput)
	for _, out := range outputs {
		key := fmt.Sprintf("%s:%s", out.Type, out.Name)
		outputMap[key] = out
	}

	// Map databases from metadata
	for _, dbName := range meta.Databases {
		key := fmt.Sprintf("database:%s", dbName)
		if output, exists := outputMap[key]; exists {
			if dbOutput, ok := output.Output.(providers.DatabaseOutput); ok {
				cfg.SQLServers[dbName] = SQLServer{
					Host:     dbOutput.Endpoint,
					Port:     dbOutput.Port,
					Database: dbName,
					User:     dbOutput.Username,
					Password: dbOutput.Password,
					TLS: &TLSConfig{
						Enabled: true,
					},
				}
			}
		}
	}

	// Map caches from metadata
	for _, cacheName := range meta.Caches {
		key := fmt.Sprintf("cache:%s", cacheName)
		if output, exists := outputMap[key]; exists {
			if cacheOutput, ok := output.Output.(providers.CacheOutput); ok {
				cfg.Redis[cacheName] = RedisServer{
					Host:      cacheOutput.Endpoint,
					Port:      cacheOutput.Port,
					Password:  cacheOutput.Password,
					Auth: &AuthConfig{
						Enabled: true,
					},
					TLS: &TLSConfig{
						Enabled: true,
					},
				}
			}
		}
	}

	// Map object storage from metadata
	for _, bucketName := range meta.ObjectStores {
		key := fmt.Sprintf("object_storage:%s", bucketName)
		if output, exists := outputMap[key]; exists {
			if objOutput, ok := output.Output.(providers.ObjectStorageOutput); ok {
				cfg.ObjectStorage[bucketName] = ObjectStore{
					Type:      "S3",
					Endpoint:  objOutput.Endpoint,
					Bucket:    objOutput.BucketName,
					Region:    objOutput.Region,
					Provider:  "alicloud",
					AccessKey: "${OSS_ACCESS_KEY_ID}",
					SecretKey: "${OSS_ACCESS_KEY_SECRET}",
				}
			}
		}
	}

	return cfg, nil
}

// Merge deep-merges two infrastructure configurations
// Values from override take precedence over base
func (g *Generator) Merge(base, override *InfraConfig) *InfraConfig {
	result := &InfraConfig{
		SQLServers:    make(map[string]SQLServer),
		Redis:         make(map[string]RedisServer),
		ObjectStorage: make(map[string]ObjectStore),
	}

	// Copy base values
	for k, v := range base.SQLServers {
		result.SQLServers[k] = v
	}
	for k, v := range base.Redis {
		result.Redis[k] = v
	}
	for k, v := range base.ObjectStorage {
		result.ObjectStorage[k] = v
	}

	// Override with new values
	for k, v := range override.SQLServers {
		result.SQLServers[k] = v
	}
	for k, v := range override.Redis {
		result.Redis[k] = v
	}
	for k, v := range override.ObjectStorage {
		result.ObjectStorage[k] = v
	}

	return result
}

// Write writes the configuration to a file
func (g *Generator) Write(cfg *InfraConfig, path string) error {
	if cfg == nil {
		return ErrInvalidConfig
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("%w: failed to create directory: %v", ErrWriteFailed, err)
		}
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("%w: failed to marshal config: %v", ErrWriteFailed, err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("%w: failed to write file: %v", ErrWriteFailed, err)
	}

	return nil
}

// ToJSON returns the configuration as formatted JSON
func (cfg *InfraConfig) ToJSON() ([]byte, error) {
	if cfg == nil {
		return nil, ErrInvalidConfig
	}
	return json.MarshalIndent(cfg, "", "  ")
}
