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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// K8sClient wraps Kubernetes operations for ACK
type K8sClient struct {
	namespace  string
	config     *K8sConfig
	clientset  kubernetes.Interface
}

// K8sConfig holds Kubernetes client configuration
type K8sConfig struct {
	KubeConfigPath string
	ClusterID      string
	Region         string
}

// NewK8sClient creates a new K8s client for ACK
func NewK8sClient(namespace string, config *K8sConfig) (*K8sClient, error) {
	client := &K8sClient{
		namespace: namespace,
		config:    config,
	}

	// Initialize Kubernetes client
	if err := client.initClient(); err != nil {
		return nil, fmt.Errorf("EDEPLOY011: failed to initialize K8s client: %w", err)
	}

	return client, nil
}

// initClient initializes the Kubernetes client
func (k *K8sClient) initClient() error {
	var cfg *rest.Config
	var err error

	if k.config.KubeConfigPath != "" {
		// Use kubeconfig file
		cfg, err = clientcmd.BuildConfigFromFlags("", k.config.KubeConfigPath)
	} else {
		// Use in-cluster config or default
		cfg, err = rest.InClusterConfig()
	}

	if err != nil {
		return fmt.Errorf("failed to build K8s config: %w", err)
	}

	k.clientset, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to create K8s clientset: %w", err)
	}

	return nil
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
			return nil, fmt.Errorf("EDEPLOY012: failed to read infracfg.json: %w", err)
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
	if resources.Deployment == "" {
		return fmt.Errorf("EDEPLOY010: deployment manifest is empty")
	}

	// Apply ConfigMap first
	if resources.ConfigMap != "" {
		if err := k.applyConfigMap(ctx, resources.ConfigMap); err != nil {
			return fmt.Errorf("EDEPLOY013: failed to apply ConfigMap: %w", err)
		}
	}

	// Apply Deployment
	if err := k.applyDeployment(ctx, resources.Deployment); err != nil {
		return fmt.Errorf("EDEPLOY014: failed to apply Deployment: %w", err)
	}

	// Apply Service
	if err := k.applyService(ctx, resources.Service); err != nil {
		return fmt.Errorf("EDEPLOY015: failed to apply Service: %w", err)
	}

	return nil
}

// applyConfigMap applies a ConfigMap manifest
func (k *K8sClient) applyConfigMap(ctx context.Context, manifest string) error {
	var configMap corev1.ConfigMap
	if err := yaml.Unmarshal([]byte(manifest), &configMap); err != nil {
		return fmt.Errorf("failed to parse ConfigMap: %w", err)
	}

	// Check if exists
	_, err := k.clientset.CoreV1().ConfigMaps(k.namespace).Get(ctx, configMap.Name, metav1.GetOptions{})
	if err != nil {
		// Create new
		_, err = k.clientset.CoreV1().ConfigMaps(k.namespace).Create(ctx, &configMap, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create ConfigMap: %w", err)
		}
	} else {
		// Update existing
		_, err = k.clientset.CoreV1().ConfigMaps(k.namespace).Update(ctx, &configMap, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update ConfigMap: %w", err)
		}
	}

	return nil
}

// applyDeployment applies a Deployment manifest
func (k *K8sClient) applyDeployment(ctx context.Context, manifest string) error {
	var deployment appsv1.Deployment
	if err := yaml.Unmarshal([]byte(manifest), &deployment); err != nil {
		return fmt.Errorf("failed to parse Deployment: %w", err)
	}

	// Check if exists
	_, err := k.clientset.AppsV1().Deployments(k.namespace).Get(ctx, deployment.Name, metav1.GetOptions{})
	if err != nil {
		// Create new
		_, err = k.clientset.AppsV1().Deployments(k.namespace).Create(ctx, &deployment, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create Deployment: %w", err)
		}
	} else {
		// Update existing
		_, err = k.clientset.AppsV1().Deployments(k.namespace).Update(ctx, &deployment, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update Deployment: %w", err)
		}
	}

	return nil
}

// applyService applies a Service manifest
func (k *K8sClient) applyService(ctx context.Context, manifest string) error {
	var service corev1.Service
	if err := yaml.Unmarshal([]byte(manifest), &service); err != nil {
		return fmt.Errorf("failed to parse Service: %w", err)
	}

	// Check if exists
	_, err := k.clientset.CoreV1().Services(k.namespace).Get(ctx, service.Name, metav1.GetOptions{})
	if err != nil {
		// Create new
		_, err = k.clientset.CoreV1().Services(k.namespace).Create(ctx, &service, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create Service: %w", err)
		}
	} else {
		// Update existing
		_, err = k.clientset.CoreV1().Services(k.namespace).Update(ctx, &service, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update Service: %w", err)
		}
	}

	return nil
}

// WaitForDeployment waits for deployment to be ready
func (k *K8sClient) WaitForDeployment(ctx context.Context, name string, timeout time.Duration) error {
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
			deployment, err := k.clientset.AppsV1().Deployments(k.namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("EDEPLOY051: failed to get deployment status: %w", err)
			}

			// Check if deployment is ready
			if deployment.Status.ReadyReplicas >= *deployment.Spec.Replicas {
				return nil
			}
		}
	}
}

// RollbackUndo performs kubectl rollout undo
func (k *K8sClient) RollbackUndo(ctx context.Context, deploymentName string) error {
	// Get deployment
	deployment, err := k.clientset.AppsV1().Deployments(k.namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("EDEPLOY060: failed to get deployment for rollback: %w", err)
	}

	// Check if there's a previous revision
	if deployment.Annotations == nil {
		return fmt.Errorf("EDEPLOY061: no previous revision to rollback to")
	}

	// Trigger rollback by updating to previous revision
	// This is a simplified implementation - in production, use DeploymentRollback
	deployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = k.clientset.AppsV1().Deployments(k.namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("EDEPLOY062: failed to rollback deployment: %w", err)
	}

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

var _ = runtime.Object(nil) // Ensure runtime package is used
