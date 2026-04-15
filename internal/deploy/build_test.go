package deploy

import (
	"testing"
	"time"

	"github.com/DaviRain-Su/infracast/internal/mapper"
	"github.com/stretchr/testify/assert"
)

// TestNewBuilder validates builder creation
func TestNewBuilder(t *testing.T) {
	builder := NewBuilder()
	assert.NotNil(t, builder)
	assert.Equal(t, 10*time.Minute, builder.timeout)
}

// TestBuildResultFields validates result struct
func TestBuildResultFields(t *testing.T) {
	result := &BuildResult{
		Success:  true,
		ImageTag: "myapp:abc123",
		BuildMeta: mapper.BuildMeta{
			AppName:   "myapp",
			Services:  []string{"api"},
			Databases: []string{"users"},
		},
		Output: "Build successful",
	}

	assert.True(t, result.Success)
	assert.Equal(t, "myapp:abc123", result.ImageTag)
	assert.Equal(t, "myapp", result.BuildMeta.AppName)
}

// TestParseImageTag validates tag parsing
func TestParseImageTag(t *testing.T) {
	builder := NewBuilder()

	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "Successfully built format",
			output:   "Successfully built myapp:abc123",
			expected: "myapp:abc123",
		},
		{
			name:     "Image tag format",
			output:   "Image tag: myapp:v1.0.0",
			expected: "myapp:v1.0.0",
		},
		{
			name:     "No match",
			output:   "Some random output",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builder.parseImageTag(tt.output)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestExtractBuildMeta validates meta extraction
func TestExtractBuildMeta(t *testing.T) {
	builder := NewBuilder()

	output := `
Building service: api
Building service: worker
Done
`

	meta := builder.extractBuildMeta(output, "myapp", "abc123")
	assert.Equal(t, "myapp", meta.AppName)
	assert.Equal(t, "abc123", meta.BuildCommit)
	assert.Contains(t, meta.Services, "api")
	assert.Contains(t, meta.Services, "worker")
}

// TestExtractBuildMetaDefaults validates default services
func TestExtractBuildMetaDefaults(t *testing.T) {
	builder := NewBuilder()

	// No services in output - should use appName as default
	meta := builder.extractBuildMeta("", "myapp", "abc123")
	assert.Equal(t, "myapp", meta.AppName)
	assert.Equal(t, []string{"myapp"}, meta.Services)
}

// TestGetLocalImageName validates image name generation
func TestGetLocalImageName(t *testing.T) {
	builder := NewBuilder()
	name := builder.GetLocalImageName("myapp", "abc123def456")
	assert.Equal(t, "myapp:abc123d", name)
}

// TestValidateBuildMeta validates meta validation
func TestValidateBuildMeta(t *testing.T) {
	builder := NewBuilder()

	// Valid meta
	validMeta := &mapper.BuildMeta{
		AppName:     "myapp",
		BuildCommit: "abc123",
		Services:    []string{"api"},
	}
	assert.NoError(t, builder.ValidateBuildMeta(validMeta))

	// Missing app name
	invalidMeta := &mapper.BuildMeta{
		BuildCommit: "abc123",
		Services:    []string{"api"},
	}
	assert.Error(t, builder.ValidateBuildMeta(invalidMeta))

	// Missing commit
	invalidMeta = &mapper.BuildMeta{
		AppName:  "myapp",
		Services: []string{"api"},
	}
	assert.Error(t, builder.ValidateBuildMeta(invalidMeta))

	// Missing services
	invalidMeta = &mapper.BuildMeta{
		AppName:     "myapp",
		BuildCommit: "abc123",
	}
	assert.Error(t, builder.ValidateBuildMeta(invalidMeta))
}

// TestBuildConfigFields validates config struct
func TestBuildConfigFields(t *testing.T) {
	config := &BuildConfig{
		AppName: "myapp",
		Commit:  "abc123",
		Timeout: 10 * time.Minute,
	}

	assert.Equal(t, "myapp", config.AppName)
	assert.Equal(t, "abc123", config.Commit)
	assert.Equal(t, 10*time.Minute, config.Timeout)
}

// TestBuilderTimeout validates custom timeout
func TestBuilderTimeout(t *testing.T) {
	builder := NewBuilder()
	assert.Equal(t, 10*time.Minute, builder.timeout)
}
