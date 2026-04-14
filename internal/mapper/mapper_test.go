package mapper

import (
	"testing"

	"github.com/DaviRain-Su/infracast/internal/config"
	"github.com/DaviRain-Su/infracast/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapper_MapToResourceSpecs(t *testing.T) {
	cfg := &config.Config{
		Provider: "alicloud",
		Region:   "cn-hangzhou",
		Overrides: config.Overrides{
			Databases: map[string]config.DatabaseOverride{
				"mydb": {
					InstanceClass: "rds.mysql.s3.large",
					StorageGB:     100,
					HighAvail:     true,
				},
			},
		},
	}

	mapper := NewMapper(cfg)

	meta := BuildMeta{
		AppName:      "myapp",
		Services:     []string{"api", "worker"},
		Databases:    []string{"mydb", "otherdb"},
		Caches:       []string{"session", "cache"},
		ObjectStores: []string{"assets", "uploads"},
	}

	specs := mapper.MapToResourceSpecs(meta)

	// Should have 6 resources (2 db + 2 cache + 2 object storage)
	assert.Len(t, specs, 6)

	// Verify database with override applied
	var mydbSpec *providers.DatabaseSpec
	for _, spec := range specs {
		if spec.Type == "database" && spec.DatabaseSpec != nil && spec.DatabaseSpec.Name == "mydb" {
			mydbSpec = spec.DatabaseSpec
			break
		}
	}
	require.NotNil(t, mydbSpec)
	assert.Equal(t, "rds.mysql.s3.large", mydbSpec.InstanceClass)
	assert.Equal(t, 100, mydbSpec.StorageGB)
	assert.True(t, mydbSpec.HighAvail)
}

func TestMapper_ValidateBuildMeta(t *testing.T) {
	cfg := &config.Config{}
	mapper := NewMapper(cfg)

	tests := []struct {
		name    string
		meta    BuildMeta
		wantErr bool
		errCode string
	}{
		{
			name: "valid meta",
			meta: BuildMeta{
				AppName:  "myapp",
				Services: []string{"api"},
			},
			wantErr: false,
		},
		{
			name: "missing app_name",
			meta: BuildMeta{
				Services: []string{"api"},
			},
			wantErr: true,
			errCode: "EMAP001",
		},
		{
			name: "missing services",
			meta: BuildMeta{
				AppName: "myapp",
			},
			wantErr: true,
			errCode: "EMAP002",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mapper.ValidateBuildMeta(tt.meta)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errCode != "" {
					assert.Contains(t, err.Error(), tt.errCode)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMapper_mapDatabase(t *testing.T) {
	cfg := &config.Config{
		Overrides: config.Overrides{
			Databases: map[string]config.DatabaseOverride{
				"override-db": {
					InstanceClass: "rds.pg.s3.large",
					StorageGB:     200,
				},
			},
		},
	}

	mapper := NewMapper(cfg)

	// Test default values
	spec := mapper.mapDatabase("default-db")
	assert.Equal(t, "default-db", spec.DatabaseSpec.Name)
	assert.Equal(t, "postgresql", spec.DatabaseSpec.Engine)
	assert.Equal(t, "rds.pg.s1.small", spec.DatabaseSpec.InstanceClass)
	assert.Equal(t, 50, spec.DatabaseSpec.StorageGB)
	assert.False(t, spec.DatabaseSpec.HighAvail)

	// Test with override
	spec = mapper.mapDatabase("override-db")
	assert.Equal(t, "rds.pg.s3.large", spec.DatabaseSpec.InstanceClass)
	assert.Equal(t, 200, spec.DatabaseSpec.StorageGB)
}

func TestMapper_mapCache(t *testing.T) {
	cfg := &config.Config{}
	mapper := NewMapper(cfg)

	spec := mapper.mapCache("mycache")
	assert.Equal(t, "cache", spec.Type)
	assert.Equal(t, "mycache", spec.CacheSpec.Name)
	assert.Equal(t, "redis", spec.CacheSpec.Engine)
	assert.Equal(t, 1024, spec.CacheSpec.MemoryMB)
}

func TestMapper_mapObjectStorage(t *testing.T) {
	cfg := &config.Config{}
	mapper := NewMapper(cfg)

	spec := mapper.mapObjectStorage("mybucket")
	assert.Equal(t, "object_storage", spec.Type)
	assert.Equal(t, "mybucket", spec.ObjectStorageSpec.Name)
	assert.Equal(t, "private", spec.ObjectStorageSpec.ACL)
}

func TestMapper_GetResourceDependencies(t *testing.T) {
	cfg := &config.Config{}
	mapper := NewMapper(cfg)

	meta := BuildMeta{
		Databases: []string{"db1", "db2"},
		Caches:    []string{"cache1"},
	}

	deps, err := mapper.GetResourceDependencies("api", meta)
	require.NoError(t, err)
	assert.Len(t, deps, 3)
	assert.Contains(t, deps, "database:db1")
	assert.Contains(t, deps, "database:db2")
	assert.Contains(t, deps, "cache:cache1")
}

func TestMapper_ExtractFromConfig(t *testing.T) {
	cfg := &config.Config{
		Provider: "alicloud",
		Region:   "cn-hangzhou",
	}
	mapper := NewMapper(cfg)

	meta := mapper.ExtractFromConfig()
	assert.Equal(t, "alicloud", meta.AppName)
	assert.Equal(t, []string{"default"}, meta.Services)
}

func TestBuildMeta_Fields(t *testing.T) {
	meta := BuildMeta{
		AppName:      "myapp",
		Services:     []string{"api", "worker"},
		Databases:    []string{"users", "orders"},
		Caches:       []string{"session", "cache"},
		ObjectStores: []string{"assets"},
		PubSubTopics: []string{"events"},
		BuildCommit:  "abc123",
		BuildImage:   "myapp:v1.0.0",
	}

	assert.Equal(t, "myapp", meta.AppName)
	assert.Len(t, meta.Services, 2)
	assert.Len(t, meta.Databases, 2)
	assert.Len(t, meta.Caches, 2)
	assert.Len(t, meta.ObjectStores, 1)
	assert.Equal(t, "abc123", meta.BuildCommit)
}
