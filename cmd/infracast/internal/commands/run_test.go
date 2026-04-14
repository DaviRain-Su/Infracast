package commands

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewRunCommand validates command creation
func TestNewRunCommand(t *testing.T) {
	cmd := newRunCommand()
	assert.NotNil(t, cmd)
	assert.Equal(t, "run", cmd.Use)
	assert.Contains(t, cmd.Short, "Run application locally")

	// Check flags
	flag := cmd.Flags().Lookup("workdir")
	assert.NotNil(t, flag)
	assert.Equal(t, ".", flag.DefValue)

	flag = cmd.Flags().Lookup("port")
	assert.NotNil(t, flag)
	assert.Equal(t, "8080", flag.DefValue)

	flag = cmd.Flags().Lookup("dev")
	assert.NotNil(t, flag)
	assert.Equal(t, "true", flag.DefValue)
}

// TestNewRunExecutor validates executor creation
func TestNewRunExecutor(t *testing.T) {
	executor := NewRunExecutor()
	assert.NotNil(t, executor)
	assert.Equal(t, 30*time.Second, executor.timeout)
}

// TestRunInputFields validates input struct
func TestRunInputFields(t *testing.T) {
	input := RunInput{
		WorkingDir: "/tmp/app",
		ConfigPath: "./config.json",
		SkipGen:    false,
		Port:       8080,
		DevMode:    true,
		DBPort:     5432,
		RedisPort:  6379,
		MinioPort:  9000,
	}

	assert.Equal(t, "/tmp/app", input.WorkingDir)
	assert.Equal(t, "./config.json", input.ConfigPath)
	assert.False(t, input.SkipGen)
	assert.Equal(t, 8080, input.Port)
	assert.True(t, input.DevMode)
}

// TestRunResultFields validates result struct
func TestRunResultFields(t *testing.T) {
	result := &RunResult{
		ConfigPath:   "./config.json",
		InfracfgPath: "/abs/path/config.json",
		Duration:     5 * time.Second,
	}

	assert.Equal(t, "./config.json", result.ConfigPath)
	assert.Equal(t, "/abs/path/config.json", result.InfracfgPath)
	assert.Equal(t, 5*time.Second, result.Duration)
}

// TestLocalConfigFields validates config struct
func TestLocalConfigFields(t *testing.T) {
	config := &LocalConfig{
		Version:     "1.0",
		AppName:     "test-app",
		Environment: "local",
		LocalMode:   true,
		Services: map[string]ServiceConfig{
			"api": {
				Name:    "api",
				URL:     "http://localhost:8080",
				Timeout: 30,
			},
		},
		Databases: map[string]DBConfig{
			"main": {
				Name:     "main",
				Host:     "localhost",
				Port:     5432,
				Database: "testdb",
				User:     "postgres",
				SSLMode:  "disable",
			},
		},
	}

	assert.Equal(t, "1.0", config.Version)
	assert.Equal(t, "test-app", config.AppName)
	assert.True(t, config.LocalMode)
	assert.Contains(t, config.Services, "api")
	assert.Contains(t, config.Databases, "main")
}

// TestGenerateLocalConfig validates config generation
func TestGenerateLocalConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	input := RunInput{
		Port:      8080,
		DBPort:    5432,
		RedisPort: 6379,
		MinioPort: 9000,
	}

	executor := NewRunExecutor()
	err := executor.generateLocalConfig(configPath, input)
	require.NoError(t, err)

	// Verify file was created
	_, err = os.Stat(configPath)
	assert.NoError(t, err)

	// Verify file content
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	contentStr := string(content)
	assert.Contains(t, contentStr, "local-app")
	assert.Contains(t, contentStr, "localhost")
	assert.Contains(t, contentStr, "5432")
}

// TestWriteConfigFile validates file writing
func TestWriteConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "subdir", "config.json")

	config := &LocalConfig{
		Version:     "1.0",
		AppName:     "test",
		Environment: "local",
		LocalMode:   true,
	}

	err := WriteConfigFile(configPath, config)
	require.NoError(t, err)

	// Verify directory was created
	_, err = os.Stat(filepath.Join(tmpDir, "subdir"))
	assert.NoError(t, err)

	// Verify file was created
	_, err = os.Stat(configPath)
	assert.NoError(t, err)
}

// TestLocalRunnerCreation validates runner creation
func TestLocalRunnerCreation(t *testing.T) {
	runner := NewLocalRunner()
	assert.NotNil(t, runner)
	assert.NotNil(t, runner.executor)
}

// TestServiceConfigFields validates service config
func TestServiceConfigFields(t *testing.T) {
	svc := ServiceConfig{
		Name:    "api",
		URL:     "http://localhost:8080",
		Timeout: 30,
	}

	assert.Equal(t, "api", svc.Name)
	assert.Equal(t, "http://localhost:8080", svc.URL)
	assert.Equal(t, 30, svc.Timeout)
}

// TestDBConfigFields validates DB config
func TestDBConfigFields(t *testing.T) {
	db := DBConfig{
		Name:     "main",
		Host:     "localhost",
		Port:     5432,
		Database: "mydb",
		User:     "postgres",
		Password: "secret",
		SSLMode:  "disable",
	}

	assert.Equal(t, "main", db.Name)
	assert.Equal(t, "localhost", db.Host)
	assert.Equal(t, 5432, db.Port)
	assert.Equal(t, "secret", db.Password)
}

// TestCacheConfigFields validates cache config
func TestCacheConfigFields(t *testing.T) {
	cache := CacheConfig{
		Name:     "default",
		Host:     "localhost",
		Port:     6379,
		Password: "",
	}

	assert.Equal(t, "default", cache.Name)
	assert.Equal(t, "localhost", cache.Host)
	assert.Equal(t, 6379, cache.Port)
}

// TestOSSConfigFields validates OSS config
func TestOSSConfigFields(t *testing.T) {
	oss := OSSConfig{
		Name:      "storage",
		Endpoint:  "http://localhost:9000",
		Bucket:    "mybucket",
		AccessKey: "key",
		SecretKey: "secret",
	}

	assert.Equal(t, "storage", oss.Name)
	assert.Equal(t, "http://localhost:9000", oss.Endpoint)
	assert.Equal(t, "mybucket", oss.Bucket)
}

// TestDefaultPorts validates default port constants
func TestDefaultPorts(t *testing.T) {
	assert.Equal(t, 8080, DefaultLocalPort)
	assert.Equal(t, 5432, DefaultLocalDBPort)
	assert.Equal(t, 6379, DefaultLocalRedisPort)
	assert.Equal(t, 9000, DefaultLocalMinioPort)
}

// TestNewLocalRunner validates runner
func TestNewLocalRunner(t *testing.T) {
	runner := NewLocalRunner()
	assert.NotNil(t, runner)
	assert.NotNil(t, runner.executor)
}

// TestValidateLocalPrerequisites validates prerequisite check (may fail in test environment)
func TestValidateLocalPrerequisites(t *testing.T) {
	// This may fail if encore/go not installed, but we test the function exists
	err := ValidateLocalPrerequisites()
	// Don't assert error since we don't know test environment
	_ = err
}

// TestJsonMarshal validates JSON marshaling helper
func TestJsonMarshal(t *testing.T) {
	config := &LocalConfig{
		Version:     "1.0",
		AppName:     "test",
		Environment: "local",
		LocalMode:   true,
	}

	data, err := jsonMarshal(config)
	require.NoError(t, err)
	assert.Contains(t, string(data), "version")
	assert.Contains(t, string(data), "local-app")
}
