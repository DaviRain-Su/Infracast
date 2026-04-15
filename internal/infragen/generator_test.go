package infragen

import (
	"testing"

	"github.com/DaviRain-Su/infracast/internal/mapper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewGenerator validates generator creation
func TestNewGenerator(t *testing.T) {
	g := NewGenerator()
	assert.NotNil(t, g)
	assert.Equal(t, "1.0", g.version)
}

// TestGeneratorGenerate validates config generation
func TestGeneratorGenerate(t *testing.T) {
	g := NewGenerator()

	outputs := []ResourceOutput{
		{
			Type: "sql_server",
			Name: "main",
			Output: map[string]string{
				"host":     "rm-xxx.mysql.rds.aliyuncs.com",
				"port":     "3306",
				"database": "myapp",
				"username": "admin",
				"password": "secret",
				"ssl_mode": "require",
			},
		},
		{
			Type: "redis",
			Name: "cache",
			Output: map[string]string{
				"host":     "r-xxx.redis.rds.aliyuncs.com",
				"port":     "6379",
				"password": "redis_secret",
			},
		},
		{
			Type: "object_storage",
			Name: "assets",
			Output: map[string]string{
				"endpoint":  "oss-cn-hangzhou.aliyuncs.com",
				"bucket":    "myapp-assets",
				"access_key": "AKxxx",
				"secret_key": "SKxxx",
			},
		},
	}

	meta := mapper.BuildMeta{
		AppName: "myapp",
	}

	cfg := g.Generate(outputs, meta, "dev")

	require.NotNil(t, cfg)
	assert.Equal(t, "1.0", cfg.Version)
	assert.Equal(t, "myapp", cfg.AppName)
	assert.Equal(t, "dev", cfg.Environment)

	// Verify SQL server
	assert.Len(t, cfg.SQLServers, 1)
	sql, ok := cfg.SQLServers["main"]
	assert.True(t, ok)
	assert.Equal(t, "rm-xxx.mysql.rds.aliyuncs.com", sql.Host)
	assert.Equal(t, 3306, sql.Port)
	assert.Equal(t, "myapp", sql.Database)

	// Verify Redis
	assert.Len(t, cfg.Redis, 1)
	redis, ok := cfg.Redis["cache"]
	assert.True(t, ok)
	assert.Equal(t, "r-xxx.redis.rds.aliyuncs.com", redis.Host)
	assert.Equal(t, 6379, redis.Port)

	// Verify Object Storage
	assert.Len(t, cfg.ObjectStorage, 1)
	obj, ok := cfg.ObjectStorage["assets"]
	assert.True(t, ok)
	assert.Equal(t, "oss-cn-hangzhou.aliyuncs.com", obj.Endpoint)
	assert.Equal(t, "myapp-assets", obj.Bucket)
}

// TestGeneratorMerge validates config merging
func TestGeneratorMerge(t *testing.T) {
	g := NewGenerator()

	base := &InfraCfg{
		Version:     "1.0",
		AppName:     "myapp",
		Environment: "dev",
		SQLServers: map[string]SQLServer{
			"main": {Name: "main", Host: "old-host"},
		},
		Redis: map[string]Redis{
			"cache": {Name: "cache", Host: "old-redis"},
		},
	}

	override := &InfraCfg{
		Version:     "1.0",
		AppName:     "myapp",
		Environment: "dev",
		SQLServers: map[string]SQLServer{
			"main": {Name: "main", Host: "new-host"},
		},
	}

	result := g.Merge(base, override)

	require.NotNil(t, result)
	// Override should take precedence
	assert.Equal(t, "new-host", result.SQLServers["main"].Host)
	// Base values should be preserved if not overridden
	assert.Equal(t, "old-redis", result.Redis["cache"].Host)
}

// TestGeneratorToJSON validates JSON marshaling
func TestGeneratorToJSON(t *testing.T) {
	g := NewGenerator()

	cfg := &InfraCfg{
		Version:     "1.0",
		AppName:     "test",
		Environment: "dev",
		SQLServers: map[string]SQLServer{
			"main": {Name: "main", Host: "localhost", Port: 5432},
		},
	}

	data, err := g.ToJSON(cfg)
	require.NoError(t, err)
	assert.Contains(t, string(data), "test")
	assert.Contains(t, string(data), "localhost")
}

// TestParseInt validates integer parsing
func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		defaultV int
		expected int
	}{
		{"5432", 3306, 5432},
		{"", 3306, 3306},
		{"invalid", 3306, 3306},
		{"0", 3306, 3306},
	}

	for _, tt := range tests {
		result := parseInt(tt.input, tt.defaultV)
		assert.Equal(t, tt.expected, result)
	}
}

// TestInfraCfgFields validates config struct fields
func TestInfraCfgFields(t *testing.T) {
	cfg := &InfraCfg{
		Version:     "1.0",
		AppName:     "myapp",
		Environment: "production",
		SQLServers:  make(map[string]SQLServer),
		Redis:       make(map[string]Redis),
		ObjectStorage: make(map[string]ObjectStore),
	}

	assert.Equal(t, "1.0", cfg.Version)
	assert.Equal(t, "myapp", cfg.AppName)
	assert.Equal(t, "production", cfg.Environment)
}

// TestSQLServerFields validates SQL server struct
func TestSQLServerFields(t *testing.T) {
	sql := SQLServer{
		Name:     "main",
		Host:     "localhost",
		Port:     5432,
		Database: "mydb",
		Username: "admin",
		Password: "secret",
		SSLMode:  "require",
	}

	assert.Equal(t, "main", sql.Name)
	assert.Equal(t, "localhost", sql.Host)
	assert.Equal(t, 5432, sql.Port)
}

// TestRedisFields validates Redis struct
func TestRedisFields(t *testing.T) {
	redis := Redis{
		Name:     "cache",
		Host:     "localhost",
		Port:     6379,
		Password: "secret",
	}

	assert.Equal(t, "cache", redis.Name)
	assert.Equal(t, 6379, redis.Port)
}

// TestObjectStoreFields validates object store struct
func TestObjectStoreFields(t *testing.T) {
	obj := ObjectStore{
		Name:      "assets",
		Endpoint:  "oss.example.com",
		Bucket:    "mybucket",
		AccessKey: "AKxxx",
		SecretKey: "SKxxx",
	}

	assert.Equal(t, "assets", obj.Name)
	assert.Equal(t, "oss.example.com", obj.Endpoint)
}
