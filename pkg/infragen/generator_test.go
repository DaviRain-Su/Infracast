package infragen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/DaviRain-Su/infracast/internal/mapper"
	"github.com/DaviRain-Su/infracast/internal/provisioner"
	"github.com/DaviRain-Su/infracast/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenerator_NewGenerator validates generator creation
func TestGenerator_NewGenerator(t *testing.T) {
	g := NewGenerator(nil)
	assert.NotNil(t, g)
	assert.Nil(t, g.base)

	base := &InfraConfig{}
	g = NewGenerator(base)
	assert.NotNil(t, g)
	assert.NotNil(t, g.base)
}

// TestGenerator_Generate validates config generation (B1-R3)
func TestGenerator_Generate(t *testing.T) {
	g := NewGenerator(nil)
	require.NotNil(t, g)

	meta := mapper.BuildMeta{
		AppName:      "myapp",
		Services:     []string{"api"},
		Databases:    []string{"users"},
		Caches:       []string{"session"},
		ObjectStores: []string{"assets"},
	}

	// Use ResourceResult instead of ResourceOutput (B1-R3)
	results := []provisioner.ResourceResult{
		{
			Name:    "users",
			Type:    "database",
			Action:  "create",
			Success: true,
			Output: &providers.DatabaseOutput{
				ResourceID: "r-123",
				Endpoint:   "pg-xxx.pg.rds.aliyuncs.com",
				Port:       5432,
				Username:   "app",
				Password:   "${USERS_DB_PASSWORD}",
			},
		},
		{
			Name:    "session",
			Type:    "cache",
			Action:  "create",
			Success: true,
			Output: &providers.CacheOutput{
				ResourceID: "r-456",
				Endpoint:   "r-xxx.redis.rds.aliyuncs.com",
				Port:       6379,
				Password:   "${SESSION_REDIS_PASSWORD}",
			},
		},
		{
			Name:    "assets",
			Type:    "object_storage",
			Action:  "create",
			Success: true,
			Output: &providers.ObjectStorageOutput{
				ResourceID: "bucket-789",
				BucketName: "myapp-assets",
				Endpoint:   "https://oss-cn-hangzhou.aliyuncs.com",
				Region:     "cn-hangzhou",
			},
		},
	}

	cfg, err := g.Generate(results, meta)
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify database
	assert.NotNil(t, cfg.SQLServers["users"])
	assert.Equal(t, "pg-xxx.pg.rds.aliyuncs.com", cfg.SQLServers["users"].Host)
	assert.Equal(t, 5432, cfg.SQLServers["users"].Port)
	assert.Equal(t, "${USERS_DB_PASSWORD}", cfg.SQLServers["users"].Password)
	assert.NotNil(t, cfg.SQLServers["users"].TLS)
	assert.True(t, cfg.SQLServers["users"].TLS.Enabled)

	// Verify cache
	assert.NotNil(t, cfg.Redis["session"])
	assert.Equal(t, "r-xxx.redis.rds.aliyuncs.com", cfg.Redis["session"].Host)
	assert.NotNil(t, cfg.Redis["session"].Auth)
	assert.NotNil(t, cfg.Redis["session"].TLS)

	// Verify object storage
	assert.NotNil(t, cfg.ObjectStorage["assets"])
	assert.Equal(t, "myapp-assets", cfg.ObjectStorage["assets"].Bucket)
	assert.Equal(t, "alicloud", cfg.ObjectStorage["assets"].Provider)
	assert.Equal(t, "${OSS_ACCESS_KEY_ID}", cfg.ObjectStorage["assets"].AccessKey)
}

// TestGenerator_Generate_WithBase validates generation with base config (B1-R3)
func TestGenerator_Generate_WithBase(t *testing.T) {
	base := &InfraConfig{
		SQLServers: map[string]SQLServer{
			"legacy": {
				Host:     "legacy-host",
				Port:     3306,
				Database: "legacy",
				User:     "legacy",
			},
		},
	}

	g := NewGenerator(base)

	meta := mapper.BuildMeta{
		AppName:   "myapp",
		Services:  []string{"api"},
		Databases: []string{"users"},
	}

	// Use ResourceResult instead of ResourceOutput (B1-R3)
	results := []provisioner.ResourceResult{
		{
			Name:    "users",
			Type:    "database",
			Action:  "create",
			Success: true,
			Output: &providers.DatabaseOutput{
				ResourceID: "r-123",
				Endpoint:   "pg-xxx.pg.rds.aliyuncs.com",
				Port:       5432,
				Username:   "app",
				Password:   "${PASSWORD}",
			},
		},
	}

	cfg, err := g.Generate(results, meta)
	require.NoError(t, err)

	// Should have both legacy and new database
	assert.NotNil(t, cfg.SQLServers["legacy"])
	assert.NotNil(t, cfg.SQLServers["users"])
}

// TestGenerator_Merge validates deep merge
func TestGenerator_Merge(t *testing.T) {
	g := NewGenerator(nil)

	base := &InfraConfig{
		SQLServers: map[string]SQLServer{
			"db1": {Host: "host1", Port: 3306},
			"db2": {Host: "host2", Port: 3306},
		},
		Redis: map[string]RedisServer{
			"cache1": {Host: "cache-host1", Port: 6379},
		},
	}

	override := &InfraConfig{
		SQLServers: map[string]SQLServer{
			"db2": {Host: "new-host2", Port: 5432}, // Override
			"db3": {Host: "host3", Port: 3306},     // New
		},
		ObjectStorage: map[string]ObjectStore{
			"bucket1": {Bucket: "bucket1"},
		},
	}

	merged := g.Merge(base, override)

	// Should preserve db1 from base
	assert.Equal(t, "host1", merged.SQLServers["db1"].Host)

	// Should use override for db2
	assert.Equal(t, "new-host2", merged.SQLServers["db2"].Host)
	assert.Equal(t, 5432, merged.SQLServers["db2"].Port)

	// Should add db3 from override
	assert.Equal(t, "host3", merged.SQLServers["db3"].Host)

	// Should preserve cache1 from base
	assert.Equal(t, "cache-host1", merged.Redis["cache1"].Host)

	// Should add bucket1 from override
	assert.Equal(t, "bucket1", merged.ObjectStorage["bucket1"].Bucket)
}

// TestGenerator_Merge_Empty validates merge with empty configs
func TestGenerator_Merge_Empty(t *testing.T) {
	g := NewGenerator(nil)

	// Empty base
	base := &InfraConfig{}
	override := &InfraConfig{
		SQLServers: map[string]SQLServer{
			"db1": {Host: "host1"},
		},
	}

	merged := g.Merge(base, override)
	assert.Equal(t, "host1", merged.SQLServers["db1"].Host)

	// Empty override
	base = &InfraConfig{
		SQLServers: map[string]SQLServer{
			"db1": {Host: "host1"},
		},
	}
	override = &InfraConfig{}

	merged = g.Merge(base, override)
	assert.Equal(t, "host1", merged.SQLServers["db1"].Host)
}

// TestGenerator_Merge_NilBase validates merge with nil base
func TestGenerator_Merge_NilBase(t *testing.T) {
	g := NewGenerator(nil)

	var base *InfraConfig
	override := &InfraConfig{
		SQLServers: map[string]SQLServer{
			"db1": {Host: "host1", Port: 3306},
		},
	}

	merged := g.Merge(base, override)
	assert.Equal(t, "host1", merged.SQLServers["db1"].Host)
	assert.Equal(t, 3306, merged.SQLServers["db1"].Port)
}

// TestGenerator_Merge_NilOverride validates merge with nil override
func TestGenerator_Merge_NilOverride(t *testing.T) {
	g := NewGenerator(nil)

	base := &InfraConfig{
		SQLServers: map[string]SQLServer{
			"db1": {Host: "host1", Port: 3306},
		},
	}
	var override *InfraConfig

	merged := g.Merge(base, override)
	assert.Equal(t, "host1", merged.SQLServers["db1"].Host)
	assert.Equal(t, 3306, merged.SQLServers["db1"].Port)
}

// TestGenerator_Write validates file writing
func TestGenerator_Write(t *testing.T) {
	g := NewGenerator(nil)

	cfg := &InfraConfig{
		SQLServers: map[string]SQLServer{
			"mydb": {
				Host:     "localhost",
				Port:     3306,
				Database: "mydb",
				User:     "app",
				Password: "${DB_PASSWORD}",
			},
		},
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "infracfg.json")

	// Write config
	err := g.Write(cfg, path)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(path)
	require.NoError(t, err)

	// Read and verify content
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var decoded InfraConfig
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "localhost", decoded.SQLServers["mydb"].Host)
	assert.Equal(t, 3306, decoded.SQLServers["mydb"].Port)
}

// TestGenerator_Write_CreatesDirectory validates directory creation
func TestGenerator_Write_CreatesDirectory(t *testing.T) {
	g := NewGenerator(nil)

	cfg := &InfraConfig{
		SQLServers: map[string]SQLServer{
			"mydb": {Host: "localhost", Port: 3306},
		},
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subdir", "nested", "infracfg.json")

	err := g.Write(cfg, path)
	require.NoError(t, err)

	// Verify directory and file created
	_, err = os.Stat(path)
	require.NoError(t, err)
}

// TestGenerator_Write_NilConfig validates error on nil config
func TestGenerator_Write_NilConfig(t *testing.T) {
	g := NewGenerator(nil)

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "infracfg.json")

	err := g.Write(nil, path)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidConfig, err)
}

// TestGenerator_Write_InvalidPath validates error on invalid path
func TestGenerator_Write_InvalidPath(t *testing.T) {
	g := NewGenerator(nil)

	cfg := &InfraConfig{
		SQLServers: map[string]SQLServer{
			"mydb": {Host: "localhost", Port: 3306},
		},
	}

	// Use an invalid path (e.g., a file in a non-existent root directory on Unix)
	path := "/nonexistent_root_dir/infracfg.json"

	err := g.Write(cfg, path)
	assert.Error(t, err)
	// Error should wrap ErrWriteFailed
	assert.ErrorIs(t, err, ErrWriteFailed)
}

// TestInfraConfig_ToJSON validates JSON output
func TestInfraConfig_ToJSON(t *testing.T) {
	cfg := &InfraConfig{
		SQLServers: map[string]SQLServer{
			"mydb": {
				Host:     "localhost",
				Port:     3306,
				Database: "mydb",
				User:     "app",
				Password: "${DB_PASSWORD}",
				TLS: &TLSConfig{
					Enabled: true,
				},
			},
		},
	}

	data, err := cfg.ToJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Verify JSON is valid
	var decoded InfraConfig
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "localhost", decoded.SQLServers["mydb"].Host)
}

// TestInfraConfig_ToJSON_Nil validates error on nil config
func TestInfraConfig_ToJSON_Nil(t *testing.T) {
	var cfg *InfraConfig
	_, err := cfg.ToJSON()
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidConfig, err)
}

// TestGenerator_Errors validates error codes (EIGEN001-003)
func TestGenerator_Errors(t *testing.T) {
	// EIGEN001: Invalid config
	err := ErrInvalidConfig
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EIGEN001")

	// EIGEN002: Merge conflict
	err = ErrMergeConflict
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EIGEN002")

	// EIGEN003: Write failed
	err = ErrWriteFailed
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EIGEN003")
}

// TestInfraConfig_StructFields validates all required fields exist
func TestInfraConfig_StructFields(t *testing.T) {
	// SQLServer with all fields
	server := SQLServer{
		Host:     "localhost",
		Port:     3306,
		Database: "mydb",
		User:     "app",
		Password: "${PASSWORD}",
		TLS: &TLSConfig{
			Enabled:            true,
			CAFile:             "/path/to/ca.crt",
			CertFile:           "/path/to/client.crt",
			KeyFile:            "/path/to/client.key",
			InsecureSkipVerify: false,
		},
	}
	assert.Equal(t, "localhost", server.Host)
	assert.NotNil(t, server.TLS)
	assert.True(t, server.TLS.Enabled)

	// RedisServer with all fields
	redis := RedisServer{
		Host:      "redis-host",
		Port:      6379,
		Password:  "${REDIS_PASSWORD}",
		KeyPrefix: "myapp:",
		Database:  0,
		Auth: &AuthConfig{
			Enabled:  true,
			Username: "redis-user",
			Password: "${REDIS_AUTH_PASSWORD}",
		},
		TLS: &TLSConfig{
			Enabled: true,
		},
	}
	assert.Equal(t, "myapp:", redis.KeyPrefix)
	assert.NotNil(t, redis.Auth)
	assert.NotNil(t, redis.TLS)

	// ObjectStore with all fields
	store := ObjectStore{
		Type:      "S3",
		Endpoint:  "https://oss-cn-hangzhou.aliyuncs.com",
		Bucket:    "mybucket",
		Region:    "cn-hangzhou",
		Provider:  "alicloud",
		AccessKey: "${ACCESS_KEY_ID}",
		SecretKey: "${ACCESS_KEY_SECRET}",
	}
	assert.Equal(t, "alicloud", store.Provider)
	assert.Equal(t, "${ACCESS_KEY_ID}", store.AccessKey)
	assert.Equal(t, "${ACCESS_KEY_SECRET}", store.SecretKey)
}

// TestGenerator_FromProvisionResults validates generation from ResourceResult (B1-R3)
func TestGenerator_FromProvisionResults(t *testing.T) {
	g := NewGenerator(nil)
	require.NotNil(t, g)

	meta := mapper.BuildMeta{
		AppName:      "myapp",
		Services:     []string{"api"},
		Databases:    []string{"users"},
		Caches:       []string{"session"},
		ObjectStores: []string{"assets"},
	}

	// Real provision outputs (simulating actual provision results)
	results := []provisioner.ResourceResult{
		{
			Name:    "users",
			Type:    "database",
			Action:  "create",
			Success: true,
			Output: &providers.DatabaseOutput{
				ResourceID: "r-123",
				Endpoint:   "pg-xxx.pg.rds.aliyuncs.com",
				Port:       5432,
				Username:   "app",
				Password:   "${USERS_DB_PASSWORD}",
			},
		},
		{
			Name:    "session",
			Type:    "cache",
			Action:  "create",
			Success: true,
			Output: &providers.CacheOutput{
				ResourceID: "r-456",
				Endpoint:   "r-xxx.redis.rds.aliyuncs.com",
				Port:       6379,
				Password:   "${SESSION_REDIS_PASSWORD}",
			},
		},
		{
			Name:    "assets",
			Type:    "object_storage",
			Action:  "create",
			Success: true,
			Output: &providers.ObjectStorageOutput{
				ResourceID: "bucket-789",
				BucketName: "myapp-assets",
				Endpoint:   "https://oss-cn-hangzhou.aliyuncs.com",
				Region:     "cn-hangzhou",
			},
		},
	}

	cfg, err := g.Generate(results, meta)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify infracfg contains real provisioned endpoints (not hardcoded)
	assert.Equal(t, "pg-xxx.pg.rds.aliyuncs.com", cfg.SQLServers["users"].Host)
	assert.Equal(t, 5432, cfg.SQLServers["users"].Port)
	assert.Equal(t, "r-xxx.redis.rds.aliyuncs.com", cfg.Redis["session"].Host)
	assert.Equal(t, 6379, cfg.Redis["session"].Port)
	assert.Equal(t, "https://oss-cn-hangzhou.aliyuncs.com", cfg.ObjectStorage["assets"].Endpoint)
}

// TestGenerator_NoHardcodedValues validates no hardcoded localhost values (B1-R3)
func TestGenerator_NoHardcodedValues(t *testing.T) {
	g := NewGenerator(nil)
	require.NotNil(t, g)

	meta := mapper.BuildMeta{
		AppName:   "myapp",
		Databases: []string{"users"},
		Caches:    []string{"session"},
	}

	// Results with real cloud endpoints
	results := []provisioner.ResourceResult{
		{
			Name:    "users",
			Type:    "database",
			Action:  "create",
			Success: true,
			Output: &providers.DatabaseOutput{
				Endpoint: "pg-xxx.pg.rds.aliyuncs.com",
				Port:     5432,
			},
		},
		{
			Name:    "session",
			Type:    "cache",
			Action:  "create",
			Success: true,
			Output: &providers.CacheOutput{
				Endpoint: "r-xxx.redis.rds.aliyuncs.com",
				Port:     6379,
			},
		},
	}

	cfg, err := g.Generate(results, meta)
	require.NoError(t, err)

	// Verify no hardcoded localhost values
	for name, server := range cfg.SQLServers {
		assert.NotContains(t, server.Host, "localhost", "database %s should not use localhost", name)
		assert.NotContains(t, server.Host, "127.0.0.1", "database %s should not use 127.0.0.1", name)
	}

	for name, redis := range cfg.Redis {
		assert.NotContains(t, redis.Host, "localhost", "cache %s should not use localhost", name)
		assert.NotContains(t, redis.Host, "127.0.0.1", "cache %s should not use 127.0.0.1", name)
	}
}

// TestGenerator_SkipsFailedResults validates failed results are skipped (B1-R3)
func TestGenerator_SkipsFailedResults(t *testing.T) {
	g := NewGenerator(nil)
	require.NotNil(t, g)

	meta := mapper.BuildMeta{
		AppName:   "myapp",
		Databases: []string{"users", "failed_db"},
	}

	results := []provisioner.ResourceResult{
		{
			Name:    "users",
			Type:    "database",
			Action:  "create",
			Success: true,
			Output: &providers.DatabaseOutput{
				Endpoint: "pg-users.pg.rds.aliyuncs.com",
				Port:     5432,
			},
		},
		{
			Name:     "failed_db",
			Type:     "database",
			Action:   "create",
			Success:  false,
			ErrorMsg: "insufficient quota",
			// No Output - failed provision
		},
	}

	cfg, err := g.Generate(results, meta)
	require.NoError(t, err)

	// Should have successful resource
	assert.NotNil(t, cfg.SQLServers["users"])
	assert.Equal(t, "pg-users.pg.rds.aliyuncs.com", cfg.SQLServers["users"].Host)

	// Should NOT have failed resource
	assert.Empty(t, cfg.SQLServers["failed_db"].Host)
}

// TestGenerator_CacheAndStorageResults validates cache and storage generation (B1-R3)
func TestGenerator_CacheAndStorageResults(t *testing.T) {
	g := NewGenerator(nil)
	require.NotNil(t, g)

	meta := mapper.BuildMeta{
		AppName:      "myapp",
		Caches:       []string{"session", "cache2"},
		ObjectStores: []string{"uploads", "backups"},
	}

	results := []provisioner.ResourceResult{
		{
			Name:    "session",
			Type:    "cache",
			Action:  "create",
			Success: true,
			Output: &providers.CacheOutput{
				Endpoint: "r-session.redis.rds.aliyuncs.com",
				Port:     6379,
				Password: "${SESSION_PWD}",
			},
		},
		{
			Name:    "cache2",
			Type:    "cache",
			Action:  "create",
			Success: true,
			Output: &providers.CacheOutput{
				Endpoint: "r-cache2.redis.rds.aliyuncs.com",
				Port:     6380,
			},
		},
		{
			Name:    "uploads",
			Type:    "object_storage",
			Action:  "create",
			Success: true,
			Output: &providers.ObjectStorageOutput{
				BucketName: "myapp-uploads",
				Endpoint:   "https://oss-cn-hangzhou.aliyuncs.com",
				Region:     "cn-hangzhou",
			},
		},
		{
			Name:    "backups",
			Type:    "object_storage",
			Action:  "create",
			Success: true,
			Output: &providers.ObjectStorageOutput{
				BucketName: "myapp-backups",
				Endpoint:   "https://oss-cn-beijing.aliyuncs.com",
				Region:     "cn-beijing",
			},
		},
	}

	cfg, err := g.Generate(results, meta)
	require.NoError(t, err)

	// Verify all caches
	assert.Equal(t, "r-session.redis.rds.aliyuncs.com", cfg.Redis["session"].Host)
	assert.Equal(t, 6379, cfg.Redis["session"].Port)
	assert.Equal(t, "${SESSION_PWD}", cfg.Redis["session"].Password)
	assert.NotNil(t, cfg.Redis["session"].Auth)
	assert.True(t, cfg.Redis["session"].Auth.Enabled)

	assert.Equal(t, "r-cache2.redis.rds.aliyuncs.com", cfg.Redis["cache2"].Host)
	assert.Equal(t, 6380, cfg.Redis["cache2"].Port)

	// Verify all object storage
	assert.Equal(t, "myapp-uploads", cfg.ObjectStorage["uploads"].Bucket)
	assert.Equal(t, "cn-hangzhou", cfg.ObjectStorage["uploads"].Region)

	assert.Equal(t, "myapp-backups", cfg.ObjectStorage["backups"].Bucket)
	assert.Equal(t, "cn-beijing", cfg.ObjectStorage["backups"].Region)
}
