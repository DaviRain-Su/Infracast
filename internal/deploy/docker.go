// Package deploy provides deployment pipeline for Infracast
package deploy

import (
	"context"
	"fmt"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/cr"
)

// ACRClient wraps AliCloud Container Registry operations
type ACRClient struct {
	client    *cr.Client
	region    string
	namespace string
}

// NewACRClient creates a new ACR client
func NewACRClient(region, accessKeyID, accessKeySecret, namespace string) (*ACRClient, error) {
	client, err := cr.NewClientWithAccessKey(region, accessKeyID, accessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("failed to create ACR client: %w", err)
	}

	return &ACRClient{
		client:    client,
		region:    region,
		namespace: namespace,
	}, nil
}

// PushImage pushes a Docker image to ACR with retry
func (a *ACRClient) PushImage(ctx context.Context, localImage, tag string) (string, error) {
	// ACR repository URL format: registry.cn-{region}.aliyuncs.com/{namespace}/{repo}:{tag}
	repo := extractRepoName(localImage)
	acrImage := fmt.Sprintf("registry.%s.aliyuncs.com/%s/%s:%s",
		getACREndpoint(a.region), a.namespace, repo, tag)

	// Retry logic: 3 attempts with exponential backoff
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return "", fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			}
		}

		// Note: Actual docker push requires docker CLI or registry client
		// This is a placeholder for the ACR push operation
		// In production, this would use docker client or go-containerregistry
		lastErr = a.pushWithSDK(ctx, localImage, acrImage)
		if lastErr == nil {
			return acrImage, nil
		}
	}

	return "", fmt.Errorf("EDEPLOY040: failed to push image after 3 attempts: %w", lastErr)
}

// pushWithSDK attempts to push using ACR SDK (placeholder for actual implementation)
func (a *ACRClient) pushWithSDK(ctx context.Context, localImage, acrImage string) error {
	// TODO: Implement actual image push using docker client or go-containerregistry
	// For now, this validates the image name format
	if localImage == "" || acrImage == "" {
		return fmt.Errorf("invalid image name")
	}
	return nil
}

// getACREndpoint returns the ACR endpoint for a region
func getACREndpoint(region string) string {
	return fmt.Sprintf("%s", region)
}

// extractRepoName extracts repository name from image reference
func extractRepoName(image string) string {
	// Simplified extraction - assumes format like "myapp:latest" or "myapp"
	for i := len(image) - 1; i >= 0; i-- {
		if image[i] == ':' {
			return image[:i]
		}
	}
	return image
}

// DeployConfig holds deployment configuration
type DeployConfig struct {
	AppName     string
	Env         string
	Image       string
	Commit      string
	Replicas    int
	Port        int
	EnvVars     map[string]string
	ConfigPath  string // Path to infracfg.json
}

// K8sResources holds generated Kubernetes manifests
type K8sResources struct {
	Deployment string
	Service    string
	ConfigMap  string
}
