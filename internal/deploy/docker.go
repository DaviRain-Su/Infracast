// Package deploy provides deployment pipeline for Infracast
package deploy

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/cr"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Default timeouts for ACR operations
const (
	DefaultACRPushTimeout = 10 * time.Minute
)

// ACRClient wraps AliCloud Container Registry operations
type ACRClient struct {
	client          *cr.Client
	region          string
	namespace       string
	timeout         time.Duration
	accessKeyID     string
	accessKeySecret string
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
		client:          client,
		region:          region,
		namespace:       namespace,
		timeout:         timeout,
		accessKeyID:     accessKeyID,
		accessKeySecret: accessKeySecret,
	}, nil
}

// PushImage pushes a Docker image to ACR with retry
func (a *ACRClient) PushImage(ctx context.Context, localImage, tag string) (string, error) {
	// ACR repository URL format: registry.cn-{region}.aliyuncs.com/{namespace}/{repo}:{tag}
	repo := extractRepoName(localImage)
	registryHost := strings.TrimSpace(os.Getenv("ACR_REGISTRY"))
	if registryHost == "" {
		registryHost = fmt.Sprintf("registry.%s.aliyuncs.com", getACREndpoint(a.region))
	}
	registryHost = strings.TrimPrefix(registryHost, "https://")
	registryHost = strings.TrimPrefix(registryHost, "http://")
	acrImage := fmt.Sprintf("%s/%s/%s:%s", registryHost, a.namespace, repo, tag)

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

// pushWithSDK pushes image using go-containerregistry with ACR authentication
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

	// Load source image from local Docker daemon first (encore build docker output),
	// then fallback to pulling from remote registry.
	srcImg, err := daemon.Image(srcRef)
	if err != nil {
		srcImg, err = remote.Image(srcRef, remote.WithContext(ctx))
		if err != nil {
			return fmt.Errorf("EDEPLOY045: failed to load source image from daemon or registry: %w", err)
		}
	}

	// Get ACR authentication token
	auth, err := a.getACRAuth(ctx)
	if err != nil {
		return fmt.Errorf("EDEPLOY047: failed to get ACR auth: %w", err)
	}

	// Push to ACR with authentication
	if err := remote.Write(dstRef, srcImg, remote.WithContext(ctx), remote.WithAuth(auth)); err != nil {
		return fmt.Errorf("EDEPLOY046: failed to push image to ACR: %w", err)
	}

	return nil
}

// getACRAuth creates ACR authenticator using access key credentials
// For ACR Personal Edition, the access key ID is used as username and
// access key secret as password
func (a *ACRClient) getACRAuth(ctx context.Context) (authn.Authenticator, error) {
	username := strings.TrimSpace(os.Getenv("ACR_USERNAME"))
	password := strings.TrimSpace(os.Getenv("ACR_PASSWORD"))
	if username != "" && password != "" {
		return authn.FromConfig(authn.AuthConfig{
			Username: username,
			Password: password,
		}), nil
	}

	if a.accessKeyID == "" || a.accessKeySecret == "" {
		return nil, fmt.Errorf("ACR credentials not initialized (set AK/SK or ACR_USERNAME/ACR_PASSWORD)")
	}

	// ACR Personal Edition authentication uses access key as credentials
	// Username: access key ID
	// Password: access key secret
	return authn.FromConfig(authn.AuthConfig{
		Username: a.accessKeyID,
		Password: a.accessKeySecret,
	}), nil
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
	AppName    string
	Env        string
	Image      string
	Commit     string
	Replicas   int
	Port       int
	EnvVars    map[string]string
	ConfigPath string // Path to infracfg.json
}

// K8sResources holds generated Kubernetes manifests
type K8sResources struct {
	Deployment string
	Service    string
	ConfigMap  string
}
