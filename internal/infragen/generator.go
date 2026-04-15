// Package infragen generates infracfg.json configuration files
package infragen

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/DaviRain-Su/infracast/internal/mapper"
)

// InfraCfg represents the infrastructure configuration structure
type InfraCfg struct {
	Version       string                 `json:"version"`
	AppName       string                 `json:"app_name"`
	Environment   string                 `json:"environment"`
	SQLServers    map[string]SQLServer   `json:"sql_servers,omitempty"`
	Redis         map[string]Redis       `json:"redis,omitempty"`
	ObjectStorage map[string]ObjectStore `json:"object_storage,omitempty"`
}

// SQLServer represents a SQL database server configuration
type SQLServer struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	SSLMode  string `json:"ssl_mode"`
}

// Redis represents a Redis cache configuration
type Redis struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Password string `json:"password,omitempty"`
}

// ObjectStore represents an object storage configuration
type ObjectStore struct {
	Name      string `json:"name"`
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
func (g *Generator) Generate(outputs []ResourceOutput, meta mapper.BuildMeta, env string) *InfraCfg {
	cfg := &InfraCfg{
		Version:     g.version,
		AppName:     meta.AppName,
		Environment: env,
		SQLServers:  make(map[string]SQLServer),
		Redis:       make(map[string]Redis),
		ObjectStorage: make(map[string]ObjectStore),
	}

	// Map resource outputs to configuration
	for _, output := range outputs {
		switch output.Type {
		case "sql_server", "database":
			cfg.SQLServers[output.Name] = g.mapSQLServer(output)
		case "redis", "cache":
			cfg.Redis[output.Name] = g.mapRedis(output)
		case "object_storage", "oss":
			cfg.ObjectStorage[output.Name] = g.mapObjectStore(output)
		}
	}

	return cfg
}

// mapSQLServer maps resource output to SQL server config
func (g *Generator) mapSQLServer(output ResourceOutput) SQLServer {
	return SQLServer{
		Name:     output.Name,
		Host:     output.Output["host"],
		Port:     parseInt(output.Output["port"], 5432),
		Database: output.Output["database"],
		Username: output.Output["username"],
		Password: output.Output["password"],
		SSLMode:  output.Output["ssl_mode"],
	}
}

// mapRedis maps resource output to Redis config
func (g *Generator) mapRedis(output ResourceOutput) Redis {
	return Redis{
		Name:     output.Name,
		Host:     output.Output["host"],
		Port:     parseInt(output.Output["port"], 6379),
		Password: output.Output["password"],
	}
}

// mapObjectStore maps resource output to object store config
func (g *Generator) mapObjectStore(output ResourceOutput) ObjectStore {
	return ObjectStore{
		Name:      output.Name,
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
		result[k] = v
	}
	return result
}

// mergeRedis merges Redis configurations
func (g *Generator) mergeRedis(base, override map[string]Redis) map[string]Redis {
	result := make(map[string]Redis)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		result[k] = v
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
		result[k] = v
	}
	return result
}

// Write writes the configuration to a file
func (g *Generator) Write(cfg *InfraCfg, path string) error {
	if cfg == nil {
		return fmt.Errorf("cannot write nil config")
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
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
