package mapper

import (
	"os"
	"path/filepath"
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
	assert.Equal(t, 20, spec.DatabaseSpec.StorageGB)
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
	assert.Equal(t, 256, spec.CacheSpec.MemoryMB)
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

// TestMapper_ScanSources validates source code scanning for resource declarations
func TestMapper_ScanSources(t *testing.T) {
	cfg := &config.Config{}
	mapper := NewMapper(cfg)

	// Create temporary test directory with sample Go files
	tmpDir := t.TempDir()

	// Create a file with database declaration
	dbFile := filepath.Join(tmpDir, "db.go")
	dbContent := `package main

import "encore.dev/storage/sql"

// Define databases
var UsersDB = &sql.SQLDatabase{Name: "users"}
var OrdersDB = &sql.SQLDatabase{Name: "orders"}
`
	require.NoError(t, os.WriteFile(dbFile, []byte(dbContent), 0644))

	// Create a file with cache declaration
	cacheFile := filepath.Join(tmpDir, "cache.go")
	cacheContent := `package main

import "encore.dev/storage/cache"

// Define caches
var SessionCache = &cache.Cache{Name: "session"}
`
	require.NoError(t, os.WriteFile(cacheFile, []byte(cacheContent), 0644))

	// Scan sources
	declarations, err := mapper.ScanSources(tmpDir)
	require.NoError(t, err)

	// Should find 3 declarations (2 databases + 1 cache)
	assert.Len(t, declarations, 3)

	// Verify database declarations
	var dbNames []string
	var cacheNames []string
	for _, decl := range declarations {
		if decl.Type == "database" {
			dbNames = append(dbNames, decl.Name)
		} else if decl.Type == "cache" {
			cacheNames = append(cacheNames, decl.Name)
		}
		assert.NotEmpty(t, decl.Location)
		assert.Greater(t, decl.Line, 0)
	}
	assert.Contains(t, dbNames, "users")
	assert.Contains(t, dbNames, "orders")
	assert.Contains(t, cacheNames, "session")
}

// TestMapper_MapToMappedResources validates DAG resource mapping
func TestMapper_MapToMappedResources(t *testing.T) {
	cfg := &config.Config{}
	mapper := NewMapper(cfg)

	specs := []providers.ResourceSpec{
		{Type: "database", DatabaseSpec: &providers.DatabaseSpec{Name: "users"}},
		{Type: "cache", CacheSpec: &providers.CacheSpec{Name: "session"}},
		{Type: "object_storage", ObjectStorageSpec: &providers.ObjectStorageSpec{Name: "assets"}},
	}

	mapped := mapper.MapToMappedResources(specs)

	// Should have 3 mapped resources
	require.Len(t, mapped, 3)

	// Verify priorities (database=1, cache=2, object_storage=3)
	for _, mr := range mapped {
		switch mr.Type {
		case "database":
			assert.Equal(t, 1, mr.Priority)
		case "cache":
			assert.Equal(t, 2, mr.Priority)
		case "object_storage":
			assert.Equal(t, 3, mr.Priority)
		}
	}
}

// TestMappedResource_Fields validates MappedResource struct fields
func TestMappedResource_Fields(t *testing.T) {
	mr := MappedResource{
		ResourceSpec: providers.ResourceSpec{
			Type: "database",
			DatabaseSpec: &providers.DatabaseSpec{
				Name: "users",
			},
		},
		Priority:  1,
		DependsOn: []string{"cache:session"},
	}

	assert.Equal(t, "database", mr.Type)
	assert.Equal(t, "users", mr.DatabaseSpec.Name)
	assert.Equal(t, 1, mr.Priority)
	assert.Equal(t, []string{"cache:session"}, mr.DependsOn)
}

// TestResourceDeclaration_Fields validates ResourceDeclaration struct fields
func TestResourceDeclaration_Fields(t *testing.T) {
	decl := ResourceDeclaration{
		Type:     "database",
		Name:     "users",
		Location: "/app/db.go",
		Line:     42,
	}

	assert.Equal(t, "database", decl.Type)
	assert.Equal(t, "users", decl.Name)
	assert.Equal(t, "/app/db.go", decl.Location)
	assert.Equal(t, 42, decl.Line)
}
