package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfig_ResolveEnv validates environment resolution
func TestConfig_ResolveEnv(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		envName string
		want    *ResolvedEnv
		wantErr error
	}{
		{
			name: "resolve existing environment",
			config: Config{
				Provider: "alicloud",
				Region:   "cn-hangzhou",
				Environments: map[string]Environment{
					"production": {
						Provider: "alicloud",
						Region:   "cn-shanghai",
					},
				},
			},
			envName: "production",
			want: &ResolvedEnv{
				Name:     "production",
				Provider: "alicloud",
				Region:   "cn-shanghai",
			},
			wantErr: nil,
		},
		{
			name: "resolve with fallback to root",
			config: Config{
				Provider: "alicloud",
				Region:   "cn-hangzhou",
			},
			envName: "staging",
			want: &ResolvedEnv{
				Name:     "staging",
				Provider: "alicloud",
				Region:   "cn-hangzhou",
			},
			wantErr: nil,
		},
		{
			name:    "empty environment name",
			config:  Config{},
			envName: "",
			want:    nil,
			wantErr: ErrMissingEnvName,
		},
		{
			name: "missing provider after resolution",
			config: Config{
				Environments: map[string]Environment{
					"test": {},
				},
			},
			envName: "test",
			want:    nil,
			wantErr: ErrMissingProvider,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.config.ResolveEnv(tt.envName)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestConfig_Validate_RegionFormat validates region format validation
func TestConfig_Validate_RegionFormat(t *testing.T) {
	tests := []struct {
		name    string
		region  string
		wantErr error
	}{
		{"valid cn-hangzhou", "cn-hangzhou", nil},
		{"valid cn-shanghai", "cn-shanghai", nil},
		{"valid us-west-1", "us-west-1", nil},
		{"invalid no hyphen", "cn", ErrInvalidRegionFormat},
		{"invalid uppercase", "CN-Hangzhou", ErrInvalidRegionFormat},
		{"invalid underscore", "cn_hangzhou", ErrInvalidRegionFormat},
		{"empty", "", ErrMissingRegion},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Provider: "alicloud",
				Region:   tt.region,
			}
			err := cfg.Validate()
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestConfig_Validate_Overrides validates override value validation
func TestConfig_Validate_Overrides(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr error
	}{
		{
			name: "valid database storage_gb",
			config: Config{
				Provider: "alicloud",
				Region:   "cn-hangzhou",
				Overrides: Overrides{
					Databases: map[string]DatabaseOverride{
						"mydb": {StorageGB: 100},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "invalid database storage_gb too small",
			config: Config{
				Provider: "alicloud",
				Region:   "cn-hangzhou",
				Overrides: Overrides{
					Databases: map[string]DatabaseOverride{
						"mydb": {StorageGB: 10},
					},
				},
			},
			wantErr: ErrInvalidStorageGB,
		},
		{
			name: "invalid database storage_gb too large",
			config: Config{
				Provider: "alicloud",
				Region:   "cn-hangzhou",
				Overrides: Overrides{
					Databases: map[string]DatabaseOverride{
						"mydb": {StorageGB: 50000},
					},
				},
			},
			wantErr: ErrInvalidStorageGB,
		},
		{
			name: "valid compute replicas",
			config: Config{
				Provider: "alicloud",
				Region:   "cn-hangzhou",
				Overrides: Overrides{
					Compute: map[string]ComputeOverride{
						"api": {Replicas: 3},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "invalid compute replicas too many",
			config: Config{
				Provider: "alicloud",
				Region:   "cn-hangzhou",
				Overrides: Overrides{
					Compute: map[string]ComputeOverride{
						"api": {Replicas: 200},
					},
				},
			},
			wantErr: ErrInvalidReplicas,
		},
		{
			name: "valid CPU format",
			config: Config{
				Provider: "alicloud",
				Region:   "cn-hangzhou",
				Overrides: Overrides{
					Compute: map[string]ComputeOverride{
						"api": {CPU: "1000m"},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "valid memory format",
			config: Config{
				Provider: "alicloud",
				Region:   "cn-hangzhou",
				Overrides: Overrides{
					Compute: map[string]ComputeOverride{
						"api": {Memory: "512Mi"},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "invalid memory format",
			config: Config{
				Provider: "alicloud",
				Region:   "cn-hangzhou",
				Overrides: Overrides{
					Compute: map[string]ComputeOverride{
						"api": {Memory: "512MB"},
					},
				},
			},
			wantErr: ErrInvalidMemoryFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestConfig_EnvironmentNameLength validates environment name max length
func TestConfig_EnvironmentNameLength(t *testing.T) {
	// Valid: 50 chars
	cfg := &Config{
		Provider: "alicloud",
		Region:   "cn-hangzhou",
		Environments: map[string]Environment{
			"this-is-a-very-long-environment-name-50chars-xxxxx": {},
		},
	}
	err := cfg.Validate()
	assert.NoError(t, err)

	// Invalid: 51 chars
	cfg = &Config{
		Provider: "alicloud",
		Region:   "cn-hangzhou",
		Environments: map[string]Environment{
			"this-is-a-very-long-environment-name-51chars-xxxxxx": {},
		},
	}
	err = cfg.Validate()
	assert.ErrorIs(t, err, ErrInvalidEnvNameLength)
}

// TestConfig_GetCacheOverride validates cache override getter
func TestConfig_GetCacheOverride(t *testing.T) {
	cfg := &Config{
		Overrides: Overrides{
			Cache: map[string]CacheOverride{
				"mycache": {
					Engine:         "redis",
					MemoryMB:       1024,
					EvictionPolicy: "allkeys-lru",
				},
			},
		},
	}

	// Get existing
	override, exists := cfg.GetCacheOverride("mycache")
	assert.True(t, exists)
	assert.Equal(t, "redis", override.Engine)
	assert.Equal(t, 1024, override.MemoryMB)

	// Get non-existent
	_, exists = cfg.GetCacheOverride("othercache")
	assert.False(t, exists)
}

// TestConfig_GetObjectStorageOverride validates object storage override getter
func TestConfig_GetObjectStorageOverride(t *testing.T) {
	cfg := &Config{
		Overrides: Overrides{
			ObjectStorage: map[string]ObjectStorageOverride{
				"mybucket": {
					ACL: "private",
				},
			},
		},
	}

	// Get existing
	override, exists := cfg.GetObjectStorageOverride("mybucket")
	assert.True(t, exists)
	assert.Equal(t, "private", override.ACL)

	// Get non-existent
	_, exists = cfg.GetObjectStorageOverride("otherbucket")
	assert.False(t, exists)
}

// TestErrorCodes validates error codes are defined
func TestErrorCodes(t *testing.T) {
	// Test that all error codes are defined
	tests := []struct {
		name string
		err  error
		code string
	}{
		{"ECFG001", ErrMissingProvider, "ECFG001"},
		{"ECFG002", ErrMissingRegion, "ECFG002"},
		{"ECFG003", ErrMissingEnvName, "ECFG003"},
		{"ECFG004", ErrUnsupportedProvider, "ECFG004"},
		{"ECFG005", ErrInvalidRegionFormat, "ECFG005"},
		{"ECFG006", ErrInvalidEnvName, "ECFG006"},
		{"ECFG007", ErrInvalidStorageGB, "ECFG007"},
		{"ECFG008", ErrInvalidReplicas, "ECFG008"},
		{"ECFG009", ErrInvalidCPUFormat, "ECFG009"},
		{"ECFG010", ErrInvalidMemoryFormat, "ECFG010"},
		{"ECFG011", ErrInvalidEngine, "ECFG011"},
		{"ECFG012", ErrInvalidVersion, "ECFG012"},
		{"ECFG013", ErrInvalidInstanceClass, "ECFG013"},
		{"ECFG014", ErrInvalidMemoryMB, "ECFG014"},
		{"ECFG015", ErrInvalidCacheEngine, "ECFG015"},
		{"ECFG016", ErrInvalidCacheVersion, "ECFG016"},
		{"ECFG017", ErrInvalidACL, "ECFG017"},
		{"ECFG018", ErrInvalidEvictionPolicy, "ECFG018"},
		{"ECFG019", ErrEnvironmentNotFound, "ECFG019"},
		{"ECFG020", ErrConfigLoadFailed, "ECFG020"},
		{"ECFG021", ErrInvalidEnvNameLength, "ECFG021"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.err)
			assert.Contains(t, tt.err.Error(), tt.code)
		})
	}
}

// TestIsValidCPUFormat validates CPU format validation
func TestIsValidCPUFormat(t *testing.T) {
	tests := []struct {
		cpu   string
		valid bool
	}{
		{"1000m", true},
		{"500m", true},
		{"2", true},
		{"16", true},
		{"1000M", false}, // uppercase M
		{"1.5", false},   // decimal
		{"", false},
		{"m", false},
		{"1000m500", false},
	}

	for _, tt := range tests {
		t.Run(tt.cpu, func(t *testing.T) {
			assert.Equal(t, tt.valid, isValidCPUFormat(tt.cpu))
		})
	}
}

// TestIsValidMemoryFormat validates memory format validation
func TestIsValidMemoryFormat(t *testing.T) {
	tests := []struct {
		memory string
		valid  bool
	}{
		{"512Mi", true},
		{"1Gi", true},
		{"16Gi", true},
		{"256Mi", true},
		{"512MB", false}, // wrong suffix
		{"512", false},   // no suffix
		{"Gi", false},    // no number
		{"512Gi256", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.memory, func(t *testing.T) {
			assert.Equal(t, tt.valid, isValidMemoryFormat(tt.memory))
		})
	}
}

// TestResolvedEnv_Fields validates ResolvedEnv struct
func TestResolvedEnv_Fields(t *testing.T) {
	env := &ResolvedEnv{
		Name:     "production",
		Provider: "alicloud",
		Region:   "cn-shanghai",
	}

	assert.Equal(t, "production", env.Name)
	assert.Equal(t, "alicloud", env.Provider)
	assert.Equal(t, "cn-shanghai", env.Region)
}
