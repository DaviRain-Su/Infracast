// Package deploy provides deployment pipeline for Infracast
package deploy

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/DaviRain-Su/infracast/pkg/infragen"
)

// K8sClient wraps Kubernetes operations for ACK
type K8sClient struct {
	namespace string
	config    *K8sConfig
}

// K8sConfig holds Kubernetes client configuration
type K8sConfig struct {
	KubeConfigPath string
	ClusterID      string
	Region         string
}

// NewK8sClient creates a new K8s client for ACK
func NewK8sClient(namespace string, config *K8sConfig) *K8sClient {
	return &K8sClient{
		namespace: namespace,
		config:    config,
	}
}

// GenerateManifests creates Kubernetes Deployment and Service YAML
func (k *K8sClient) GenerateManifests(cfg *DeployConfig, infraCfg *infragen.InfraConfig) (*K8sResources, error) {
	// Generate labels
	labels := map[string]string{
		"app":                  cfg.AppName,
		"infracast.dev/env":    cfg.Env,
		"infracast.dev/commit": cfg.Commit,
	}

	// Generate env vars
	envVars := k.generateEnvVars(cfg.EnvVars)

	// Generate Deployment YAML
	deployment := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  labels:
    %s
spec:
  replicas: %d
  selector:
    matchLabels:
      app: %s
  template:
    metadata:
      labels:
        %s
    spec:
      containers:
      - name: app
        image: %s
        ports:
        - containerPort: %d
        env:
        %s
        envFrom:
        - configMapRef:
            name: %s-config
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
`,
		cfg.AppName,
		k.namespace,
		formatLabels(labels),
		cfg.Replicas,
		cfg.AppName,
		formatLabels(labels),
		cfg.Image,
		cfg.Port,
		envVars,
		cfg.AppName,
	)

	// Generate Service YAML
	service := fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s
  namespace: %s
  labels:
    %s
spec:
  selector:
    app: %s
  ports:
  - port: 80
    targetPort: %d
  type: ClusterIP
`,
		cfg.AppName,
		k.namespace,
		formatLabels(labels),
		cfg.AppName,
		cfg.Port,
	)

	// Generate ConfigMap from infracfg.json
	configMap := ""
	if cfg.ConfigPath != "" {
		configData, err := os.ReadFile(cfg.ConfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read infracfg.json: %w", err)
		}

		configMap = fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: %s-config
  namespace: %s
data:
  infracfg.json: |
    %s
`,
			cfg.AppName,
			k.namespace,
			indentYAML(string(configData)),
		)
	}

	return &K8sResources{
		Deployment: deployment,
		Service:    service,
		ConfigMap:  configMap,
	}, nil
}

// Apply applies Kubernetes manifests to the cluster
func (k *K8sClient) Apply(ctx context.Context, resources *K8sResources) error {
	// TODO: Implement actual kubectl apply using client-go or exec
	// For now, this is a placeholder that validates the manifests

	if resources.Deployment == "" {
		return fmt.Errorf("EDEPLOY010: deployment manifest is empty")
	}

	// In production, this would:
	// 1. Create ConfigMap first (so it's available to pods)
	// 2. Create Deployment
	// 3. Create Service

	return nil
}

// WaitForDeployment waits for deployment to be ready
func (k *K8sClient) WaitForDeployment(ctx context.Context, name string, timeout time.Duration) error {
	// TODO: Implement actual status check using client-go
	// Poll deployment status every 10s until timeout
	
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("EDEPLOY050: deployment timeout after %v", timeout)
		case <-ticker.C:
			// Check deployment status
			// In production, use k8s client to get deployment status
			return nil // Placeholder
		}
	}
}

// RollbackUndo performs kubectl rollout undo
func (k *K8sClient) RollbackUndo(ctx context.Context, deploymentName string) error {
	// TODO: Implement actual rollback using client-go
	// kubectl rollout undo deployment/<name> -n <namespace>
	return nil
}

// generateEnvVars generates env var YAML from map
func (k *K8sClient) generateEnvVars(envVars map[string]string) string {
	if len(envVars) == 0 {
		return ""
	}

	var result strings.Builder
	for key, value := range envVars {
		result.WriteString(fmt.Sprintf("        - name: %s\n", key))
		result.WriteString(fmt.Sprintf("          value: %q\n", value))
	}
	return result.String()
}

// formatLabels formats labels map for YAML
func formatLabels(labels map[string]string) string {
	var parts []string
	for k, v := range labels {
		parts = append(parts, fmt.Sprintf("%s: %q", k, v))
	}
	return strings.Join(parts, "\n    ")
}

// indentYAML indents multi-line YAML string
func indentYAML(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = "    " + line
		}
	}
	return strings.Join(lines, "\n")
}

// encodeBase64 base64 encodes a string
func encodeBase64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}
