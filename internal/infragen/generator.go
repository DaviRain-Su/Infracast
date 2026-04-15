// Package infragen generates infracfg.json configuration files
package infragen

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/DaviRain-Su/infracast/internal/mapper"
)

// Error codes for config generator
const (
	EIGEN001 = "EIGEN001" // Unsupported resource type
	EIGEN002 = "EIGEN002" // Missing required field
	EIGEN003 = "EIGEN003" // Write error
)

// InfraCfg represents the infrastructure configuration structure
type InfraCfg struct {
	Version       string                   `json:"version"`
	AppName       string                   `json:"app_name"`
	Environment   string                   `json:"environment"`
	SQLServers    map[string]SQLServer     `json:"sql_servers,omitempty"`
	Redis         map[string]RedisServer   `json:"redis,omitempty"`
	ObjectStorage map[string]ObjectStore   `json:"object_storage,omitempty"`
}

// SQLServer represents a SQL database server configuration
type SQLServer struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	User     string `json:"user"`
	Password string `json:"password,omitempty"`
	TLS      *TLSConfig `json:"tls,omitempty"`
}

// TLSConfig represents TLS configuration
type TLSConfig struct {
	Enabled bool `json:"enabled"`
}

// RedisServer represents a Redis cache configuration
type RedisServer struct {
	Name      string `json:"name"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Auth      string `json:"auth,omitempty"`
	KeyPrefix string `json:"key_prefix,omitempty"`
}

// ObjectStore represents an object storage configuration
type ObjectStore struct {
	Name      string `json:"name"`
	Provider  string `json:"provider"`
	Region    string `json:"region,omitempty"`
	Endpoint  string `json:"endpoint"`
	Bucket    string `json:"bucket"`
	AccessKey string `json:"access_key,omitempty"`
	SecretKey string `json:"secret_key,omitempty"`
}

// ResourceOutput contains the actual provisioned resource information
type ResourceOutput struct {
	Type   string
	Name   string
	Output map[string]string
}

// Generator creates infrastructure configuration
type Generator struct {
	version string
}

// NewGenerator creates a new configuration generator
func NewGenerator() *Generator {
	return &Generator{
		version: "1.0",
	}
}

// Generate creates an InfraCfg from resource outputs and build metadata
func (g *Generator) Generate(outputs []ResourceOutput, meta mapper.BuildMeta, env string) (*InfraCfg, error) {
	cfg := &InfraCfg{
		Version:     g.version,
		AppName:     meta.AppName,
		Environment: env,
		SQLServers:  make(map[string]SQLServer),
		Redis:       make(map[string]RedisServer),
		ObjectStorage: make(map[string]ObjectStore),
	}

	// Map resource outputs to configuration
	for _, output := range outputs {
		switch output.Type {
		case "sql_server", "database":
			server := g.mapSQLServer(output)
			if err := validateSQLServer(server); err != nil {
				return nil, fmt.Errorf("%s: %s for %s", EIGEN002, err.Error(), output.Name)
			}
			cfg.SQLServers[output.Name] = server
		case "redis", "cache":
			redis := g.mapRedis(output)
			if err := validateRedis(redis); err != nil {
				return nil, fmt.Errorf("%s: %s for %s", EIGEN002, err.Error(), output.Name)
			}
			cfg.Redis[output.Name] = redis
		case "object_storage", "oss":
			store := g.mapObjectStore(output)
			if err := validateObjectStore(store); err != nil {
				return nil, fmt.Errorf("%s: %s for %s", EIGEN002, err.Error(), output.Name)
			}
			cfg.ObjectStorage[output.Name] = store
		default:
			return nil, fmt.Errorf("%s: unsupported resource type %s", EIGEN001, output.Type)
		}
	}

	return cfg, nil
}

// mapSQLServer maps resource output to SQL server config
func (g *Generator) mapSQLServer(output ResourceOutput) SQLServer {
	return SQLServer{
		Name:     output.Name,
		Host:     output.Output["host"],
		Port:     parseInt(output.Output["port"], 5432),
		Database: output.Output["database"],
		User:     output.Output["user"],
		Password: output.Output["password"],
	}
}

// mapRedis maps resource output to Redis config
func (g *Generator) mapRedis(output ResourceOutput) RedisServer {
	return RedisServer{
		Name:      output.Name,
		Host:      output.Output["host"],
		Port:      parseInt(output.Output["port"], 6379),
		Auth:      output.Output["auth"],
		KeyPrefix: output.Output["key_prefix"],
	}
}

// mapObjectStore maps resource output to object store config
func (g *Generator) mapObjectStore(output ResourceOutput) ObjectStore {
	return ObjectStore{
		Name:      output.Name,
		Provider:  output.Output["provider"],
		Region:    output.Output["region"],
		Endpoint:  output.Output["endpoint"],
		Bucket:    output.Output["bucket"],
		AccessKey: output.Output["access_key"],
		SecretKey: output.Output["secret_key"],
	}
}

// Merge deep-merges two configurations, with override taking precedence
func (g *Generator) Merge(base, override *InfraCfg) *InfraCfg {
	if base == nil {
		return override
	}
	if override == nil {
		return base
	}

	result := &InfraCfg{
		Version:     override.Version,
		AppName:     override.AppName,
		Environment: override.Environment,
		SQLServers:  g.mergeSQLServers(base.SQLServers, override.SQLServers),
		Redis:       g.mergeRedis(base.Redis, override.Redis),
		ObjectStorage: g.mergeObjectStorage(base.ObjectStorage, override.ObjectStorage),
	}

	return result
}

// mergeSQLServers merges SQL server configurations
func (g *Generator) mergeSQLServers(base, override map[string]SQLServer) map[string]SQLServer {
	result := make(map[string]SQLServer)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		if existing, ok := result[k]; ok {
			// Field-level merge: override non-zero values
			if v.Host != "" {
				existing.Host = v.Host
			}
			if v.Port != 0 {
				existing.Port = v.Port
			}
			if v.Database != "" {
				existing.Database = v.Database
			}
			if v.User != "" {
				existing.User = v.User
			}
			if v.Password != "" {
				existing.Password = v.Password
			}
			result[k] = existing
		} else {
			result[k] = v
		}
	}
	return result
}

// mergeRedis merges Redis configurations
func (g *Generator) mergeRedis(base, override map[string]RedisServer) map[string]RedisServer {
	result := make(map[string]RedisServer)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		if existing, ok := result[k]; ok {
			// Field-level merge: override non-zero values
			if v.Host != "" {
				existing.Host = v.Host
			}
			if v.Port != 0 {
				existing.Port = v.Port
			}
			if v.Auth != "" {
				existing.Auth = v.Auth
			}
			if v.KeyPrefix != "" {
				existing.KeyPrefix = v.KeyPrefix
			}
			result[k] = existing
		} else {
			result[k] = v
		}
	}
	return result
}

// mergeObjectStorage merges object storage configurations
func (g *Generator) mergeObjectStorage(base, override map[string]ObjectStore) map[string]ObjectStore {
	result := make(map[string]ObjectStore)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		if existing, ok := result[k]; ok {
			// Field-level merge: override non-zero values
			if v.Provider != "" {
				existing.Provider = v.Provider
			}
			if v.Region != "" {
				existing.Region = v.Region
			}
			if v.Endpoint != "" {
				existing.Endpoint = v.Endpoint
			}
			if v.Bucket != "" {
				existing.Bucket = v.Bucket
			}
			if v.AccessKey != "" {
				existing.AccessKey = v.AccessKey
			}
			if v.SecretKey != "" {
				existing.SecretKey = v.SecretKey
			}
			result[k] = existing
		} else {
			result[k] = v
		}
	}
	return result
}

// Write writes the configuration to a file
func (g *Generator) Write(cfg *InfraCfg, path string) error {
	if cfg == nil {
		return fmt.Errorf("%s: cannot write nil config", EIGEN003)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("%s: failed to marshal config: %w", EIGEN003, err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("%s: failed to write config file: %w", EIGEN003, err)
	}

	return nil
}

// ToJSON returns the configuration as JSON bytes
func (g *Generator) ToJSON(cfg *InfraCfg) ([]byte, error) {
	if cfg == nil {
		return nil, fmt.Errorf("cannot marshal nil config")
	}
	return json.MarshalIndent(cfg, "", "  ")
}

// parseInt parses an integer from string with default value
func parseInt(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	var result int
	fmt.Sscanf(s, "%d", &result)
	if result == 0 {
		return defaultVal
	}
	return result
}

// validateSQLServer validates that required fields are present
func validateSQLServer(s SQLServer) error {
	if s.Host == "" {
		return fmt.Errorf("host is required")
	}
	if s.Port == 0 {
		return fmt.Errorf("port is required")
	}
	if s.User == "" {
		return fmt.Errorf("user is required")
	}
	if s.Password == "" {
		return fmt.Errorf("password is required")
	}
	return nil
}

// validateRedis validates that required fields are present
func validateRedis(r RedisServer) error {
	if r.Host == "" {
		return fmt.Errorf("host is required")
	}
	if r.Port == 0 {
		return fmt.Errorf("port is required")
	}
	return nil
}

// validateObjectStore validates that required fields are present
func validateObjectStore(o ObjectStore) error {
	if o.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	if o.Bucket == "" {
		return fmt.Errorf("bucket is required")
	}
	return nil
}
