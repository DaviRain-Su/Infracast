package deploy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExtractRepoName validates image name extraction
func TestExtractRepoName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"myapp:latest", "myapp"},
		{"myapp", "myapp"},
		{"registry.example.com/myapp:v1.0.0", "registry.example.com/myapp"},
		{"my-app:1.2.3", "my-app"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractRepoName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGetACREndpoint validates ACR endpoint generation
func TestGetACREndpoint(t *testing.T) {
	endpoint := getACREndpoint("cn-hangzhou")
	assert.Equal(t, "cn-hangzhou", endpoint)
}

// TestNewACRClient validates client creation
func TestNewACRClient(t *testing.T) {
	// Skip if no real credentials
	t.Skip("Skipping ACR client test - requires real AliCloud credentials")

	client, err := NewACRClient("cn-hangzhou", "test-ak", "test-sk", "my-namespace")
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, "cn-hangzhou", client.region)
	assert.Equal(t, "my-namespace", client.namespace)
}
