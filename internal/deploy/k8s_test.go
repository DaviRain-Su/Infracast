package deploy

import (
	"testing"

	"github.com/DaviRain-Su/infracast/pkg/infragen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenerateManifests validates manifest generation
func TestGenerateManifests(t *testing.T) {
	client := NewK8sClient("test-ns", &K8sConfig{
		KubeConfigPath: "~/.kube/config",
		ClusterID:      "c123",
		Region:         "cn-hangzhou",
	})

	cfg := &DeployConfig{
		AppName:  "myapp",
		Env:      "staging",
		Image:    "registry.cn-hangzhou.aliyuncs.com/test/myapp:v1.0.0",
		Commit:   "abc123",
		Replicas: 2,
		Port:     8080,
		EnvVars: map[string]string{
			"LOG_LEVEL": "info",
		},
	}

	infraCfg := &infragen.InfraCfg{
		SQLServers: map[string]infragen.SQLServer{
			"users": {Host: "localhost", Port: 5432},
		},
	}

	resources, err := client.GenerateManifests(cfg, infraCfg)
	require.NoError(t, err)
	assert.NotNil(t, resources)

	// Verify deployment contains expected content
	assert.Contains(t, resources.Deployment, "name: myapp")
	assert.Contains(t, resources.Deployment, "namespace: test-ns")
	assert.Contains(t, resources.Deployment, "infracast.dev/env: \"staging\"")
	assert.Contains(t, resources.Deployment, "infracast.dev/commit: \"abc123\"")
	assert.Contains(t, resources.Deployment, "replicas: 2")
	assert.Contains(t, resources.Deployment, "containerPort: 8080")
	assert.Contains(t, resources.Deployment, "image: registry.cn-hangzhou.aliyuncs.com/test/myapp:v1.0.0")

	// Verify service contains expected content
	assert.Contains(t, resources.Service, "name: myapp")
	assert.Contains(t, resources.Service, "targetPort: 8080")

	// Verify env vars
	assert.Contains(t, resources.Deployment, "name: LOG_LEVEL")
	assert.Contains(t, resources.Deployment, "value: \"info\"")
}

// TestFormatLabels validates label formatting
func TestFormatLabels(t *testing.T) {
	labels := map[string]string{
		"app":     "myapp",
		"version": "v1",
	}

	result := formatLabels(labels)
	assert.Contains(t, result, `app: "myapp"`)
	assert.Contains(t, result, `version: "v1"`)
}

// TestNewK8sClient validates client creation
func TestNewK8sClient(t *testing.T) {
	config := &K8sConfig{
		KubeConfigPath: "~/.kube/config",
		ClusterID:      "c123",
		Region:         "cn-hangzhou",
	}

	client := NewK8sClient("test-ns", config)
	assert.NotNil(t, client)
	assert.Equal(t, "test-ns", client.namespace)
	assert.Equal(t, config, client.config)
}
