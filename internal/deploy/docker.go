// Package deploy provides deployment pipeline for Infracast
package deploy

import (
	"context"
	"fmt"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/cr"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Default timeouts for ACR operations
const (
	DefaultACRPushTimeout = 10 * time.Minute
)

// ACRClient wraps AliCloud Container Registry operations
type ACRClient struct {
	client    *cr.Client
	region    string
	namespace string
	timeout   time.Duration
}

// NewACRClient creates a new ACR client
func NewACRClient(region, accessKeyID, accessKeySecret, namespace string) (*ACRClient, error) {
	return NewACRClientWithTimeout(region, accessKeyID, accessKeySecret, namespace, DefaultACRPushTimeout)
}

// NewACRClientWithTimeout creates a new ACR client with custom timeout
func NewACRClientWithTimeout(region, accessKeyID, accessKeySecret, namespace string, timeout time.Duration) (*ACRClient, error) {
	if timeout <= 0 {
		timeout = DefaultACRPushTimeout
	}
	
	client, err := cr.NewClientWithAccessKey(region, accessKeyID, accessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("failed to create ACR client: %w", err)
	}

	return &ACRClient{
		client:    client,
		region:    region,
		namespace: namespace,
		timeout:   timeout,
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
				return "", fmt.Errorf("EDEPLOY041: context cancelled during retry: %w", ctx.Err())
			}
		}

		lastErr = a.pushWithSDK(ctx, localImage, acrImage)
		if lastErr == nil {
			return acrImage, nil
		}
	}

	return "", fmt.Errorf("EDEPLOY040: failed to push image after 3 attempts: %w", lastErr)
}

// pushWithSDK pushes image using go-containerregistry
func (a *ACRClient) pushWithSDK(ctx context.Context, localImage, acrImage string) error {
	if localImage == "" || acrImage == "" {
		return fmt.Errorf("EDEPLOY042: invalid image name")
	}

	// Parse source image reference
	srcRef, err := name.ParseReference(localImage)
	if err != nil {
		return fmt.Errorf("EDEPLOY043: failed to parse source image: %w", err)
	}

	// Parse destination image reference
	dstRef, err := name.ParseReference(acrImage)
	if err != nil {
		return fmt.Errorf("EDEPLOY044: failed to parse destination image: %w", err)
	}

	// Pull source image
	srcImg, err := remote.Image(srcRef, remote.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("EDEPLOY045: failed to pull source image: %w", err)
	}

	// Push to ACR (using anonymous auth for now, should use ACR credentials)
	if err := remote.Write(dstRef, srcImg, remote.WithContext(ctx)); err != nil {
		return fmt.Errorf("EDEPLOY046: failed to push image to ACR: %w", err)
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
