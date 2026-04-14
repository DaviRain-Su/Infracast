package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name: "valid config",
			content: `provider: alicloud
region: cn-hangzhou
`,
			wantErr: false,
		},
		{
			name:    "missing file",
			content: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "infracast.yaml")

			if tt.content != "" {
				if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
					t.Fatalf("Failed to write test config: %v", err)
				}
			}

			_, err := Load(configPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: Config{
				Provider: "alicloud",
				Region:   "cn-hangzhou",
			},
			wantErr: false,
		},
		{
			name: "missing provider",
			config: Config{
				Region: "cn-hangzhou",
			},
			wantErr: true,
			errMsg:  "provider is required",
		},
		{
			name: "missing region",
			config: Config{
				Provider: "alicloud",
			},
			wantErr: true,
			errMsg:  "region is required",
		},
		{
			name: "unsupported provider",
			config: Config{
				Provider: "aws",
				Region:   "us-east-1",
			},
			wantErr: true,
			errMsg:  "unsupported provider",
		},
		{
			name: "invalid environment name",
			config: Config{
				Provider: "alicloud",
				Region:   "cn-hangzhou",
				Environments: map[string]Environment{
					"invalid_env": {},
				},
			},
			wantErr: true,
			errMsg:  "invalid environment name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if err.Error() == "" || !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error message = %v, want containing %v", err, tt.errMsg)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TA06: Config Parser Enhancement Tests

func TestConfig_GetEnvironment(t *testing.T) {
	cfg := &Config{
		Provider: "alicloud",
		Region:   "cn-hangzhou",
		Environments: map[string]Environment{
			"production": {
				Provider: "alicloud",
				Region:   "cn-shanghai",
			},
			"staging": {
				Provider: "alicloud",
				Region:   "cn-hangzhou",
			},
		},
	}

	// Get existing environment
	env, err := cfg.GetEnvironment("production")
	require.NoError(t, err)
	assert.Equal(t, "cn-shanghai", env.Region)

	// Get non-existent environment - should return default
	env, err = cfg.GetEnvironment("dev")
	require.NoError(t, err)
	assert.Equal(t, "cn-hangzhou", env.Region) // Default from root config
}

func TestConfig_MergeWithDefaults(t *testing.T) {
	cfg := &Config{
		Provider: "alicloud",
		Region:   "cn-hangzhou",
		Environments: map[string]Environment{
			"production": {
				Provider: "alicloud",
				// Region intentionally missing - should inherit from root
			},
		},
	}

	merged := cfg.MergeWithDefaults()

	// Root config should remain unchanged
	assert.Equal(t, "cn-hangzhou", merged.Region)

	// Environment should inherit missing fields from root
	prodEnv := merged.Environments["production"]
	assert.Equal(t, "alicloud", prodEnv.Provider)
	assert.Equal(t, "cn-hangzhou", prodEnv.Region) // Inherited
}

func TestConfig_GetResourceOverride(t *testing.T) {
	cfg := &Config{
		Provider: "alicloud",
		Region:   "cn-hangzhou",
		Overrides: Overrides{
			Databases: map[string]DatabaseOverride{
				"mydb": {
					InstanceClass: "rds.mysql.s3.large",
					StorageGB:     100,
				},
			},
			Compute: map[string]ComputeOverride{
				"api": {
					Replicas: 3,
					CPU:      "2000m",
				},
			},
		},
	}

	// Get existing database override
	override, exists := cfg.GetDatabaseOverride("mydb")
	assert.True(t, exists)
	assert.Equal(t, "rds.mysql.s3.large", override.InstanceClass)

	// Get non-existent override
	_, exists = cfg.GetDatabaseOverride("otherdb")
	assert.False(t, exists)

	// Get compute override
	computeOverride, exists := cfg.GetComputeOverride("api")
	assert.True(t, exists)
	assert.Equal(t, 3, computeOverride.Replicas)
}

func TestConfig_ValidateEnvironmentName(t *testing.T) {
	tests := []struct {
		name    string
		envName string
		valid   bool
	}{
		{"valid lowercase", "production", true},
		{"valid with hyphen", "my-env", true},
		{"valid with number", "env123", true},
		{"invalid uppercase", "Production", false},
		{"invalid underscore", "my_env", false},
		{"invalid space", "my env", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Provider: "alicloud",
				Region:   "cn-hangzhou",
				Environments: map[string]Environment{
					tt.envName: {},
				},
			}
			err := cfg.Validate()
			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestConfig_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "infracast.yaml")

	cfg := &Config{
		Provider: "alicloud",
		Region:   "cn-hangzhou",
		Environments: map[string]Environment{
			"production": {
				Provider: "alicloud",
				Region:   "cn-shanghai",
			},
		},
		Overrides: Overrides{
			Databases: map[string]DatabaseOverride{
				"mydb": {
					InstanceClass: "rds.mysql.s3.large",
				},
			},
		},
	}

	// Save
	err := cfg.Save(configPath)
	require.NoError(t, err)

	// Load
	loaded, err := Load(configPath)
	require.NoError(t, err)

	// Verify
	assert.Equal(t, cfg.Provider, loaded.Provider)
	assert.Equal(t, cfg.Region, loaded.Region)
	assert.Equal(t, "cn-shanghai", loaded.Environments["production"].Region)
	assert.Equal(t, "rds.mysql.s3.large", loaded.Overrides.Databases["mydb"].InstanceClass)
}
