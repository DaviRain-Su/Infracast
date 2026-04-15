// Package deploy provides deployment pipeline for Infracast
package deploy

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"time"

	"github.com/DaviRain-Su/infracast/internal/mapper"
)

// Builder wraps encore build operations
type Builder struct {
	timeout time.Duration
}

// Default timeouts for build operations
const (
	DefaultBuildTimeout = 10 * time.Minute
)

// NewBuilder creates a new builder with default timeout
func NewBuilder() *Builder {
	return &Builder{
		timeout: DefaultBuildTimeout,
	}
}

// NewBuilderWithTimeout creates a builder with custom timeout
func NewBuilderWithTimeout(timeout time.Duration) *Builder {
	if timeout <= 0 {
		timeout = DefaultBuildTimeout
	}
	return &Builder{
		timeout: timeout,
	}
}

// BuildResult represents the result of a build operation
type BuildResult struct {
	Success   bool
	ImageTag  string
	BuildMeta mapper.BuildMeta
	Output    string
	Error     error
}

// Build executes `encore build docker <tag>` and parses output
func (b *Builder) Build(ctx context.Context, appName, commit string) (*BuildResult, error) {
	ctx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()

	imageTag := fmt.Sprintf("%s:%s", appName, shortCommit(commit))

	// Execute encore build
	cmd := exec.CommandContext(ctx, "encore", "build", "docker", imageTag)
	if appRoot := os.Getenv("ENCORE_APP_ROOT"); appRoot != "" {
		cmd.Dir = appRoot
	}

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("EDEPLOY001: build timeout after %v", b.timeout)
		}
		return nil, fmt.Errorf("EDEPLOY001: build failed: %w\nOutput: %s", err, outputStr)
	}

	// Parse image tag from output
	parsedTag := b.parseImageTag(outputStr)
	if parsedTag == "" {
		parsedTag = imageTag // Fallback to constructed tag
	}

	// Extract BuildMeta from build output
	buildMeta := b.extractBuildMeta(outputStr, appName, commit)
	buildMeta.BuildImage = parsedTag

	return &BuildResult{
		Success:   true,
		ImageTag:  parsedTag,
		BuildMeta: buildMeta,
		Output:    outputStr,
	}, nil
}

// parseImageTag extracts the image tag from encore build output
func (b *Builder) parseImageTag(output string) string {
	// Look for patterns like:
	// "Successfully built myapp:abc123"
	// "Image tag: myapp:abc123"
	patterns := []string{
		`Successfully built\s+(\S+)`,
		`Image tag:\s+(\S+)`,
		`tag:\s+(\S+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(output)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}

// extractBuildMeta extracts BuildMeta from build output
func (b *Builder) extractBuildMeta(output, appName, commit string) mapper.BuildMeta {
	meta := mapper.BuildMeta{
		AppName:     appName,
		BuildCommit: commit,
	}

	// Parse services from output
	// Look for lines like "Building service: api" or "Service: api"
	re := regexp.MustCompile(`(?i)(?:building service|service):\s*(\w+)`)
	matches := re.FindAllStringSubmatch(output, -1)
	for _, match := range matches {
		if len(match) > 1 {
			meta.Services = append(meta.Services, match[1])
		}
	}

	// If no services found in output, use defaults
	if len(meta.Services) == 0 {
		meta.Services = []string{appName}
	}

	return meta
}

// GetLocalImageName returns the local image name before pushing to registry
func (b *Builder) GetLocalImageName(appName, commit string) string {
	return fmt.Sprintf("%s:%s", appName, shortCommit(commit))
}

// ValidateBuildMeta validates that BuildMeta has required fields
func (b *Builder) ValidateBuildMeta(meta *mapper.BuildMeta) error {
	if meta.AppName == "" {
		return fmt.Errorf("EDEPLOY002: BuildMeta.AppName is required")
	}
	if meta.BuildCommit == "" {
		return fmt.Errorf("EDEPLOY002: BuildMeta.BuildCommit is required")
	}
	if len(meta.Services) == 0 {
		return fmt.Errorf("EDEPLOY002: BuildMeta.Services cannot be empty")
	}
	return nil
}

// BuildWithConfig executes build with custom configuration
func (b *Builder) BuildWithConfig(ctx context.Context, config *BuildConfig) (*BuildResult, error) {
	if config.Timeout > 0 {
		b.timeout = config.Timeout
	}

	return b.Build(ctx, config.AppName, config.Commit)
}

// BuildConfig holds build configuration
type BuildConfig struct {
	AppName string
	Commit  string
	Timeout time.Duration
}

// StreamBuild executes build and streams output to a channel
func (b *Builder) StreamBuild(ctx context.Context, appName, commit string, outputChan chan<- string) (*BuildResult, error) {
	ctx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()

	imageTag := fmt.Sprintf("%s:%s", appName, shortCommit(commit))
	cmd := exec.CommandContext(ctx, "encore", "build", "docker", imageTag)
	if appRoot := os.Getenv("ENCORE_APP_ROOT"); appRoot != "" {
		cmd.Dir = appRoot
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("EDEPLOY001: failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("EDEPLOY001: failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("EDEPLOY001: failed to start build: %w", err)
	}

	// Stream output
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			outputChan <- line
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			outputChan <- line
		}
	}()

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("EDEPLOY001: build failed: %w", err)
	}

	return &BuildResult{
		Success:  true,
		ImageTag: imageTag,
	}, nil
}

// shortCommit returns the first 7 characters of a commit hash, or the full
// string if shorter than 7, to avoid slice bounds panics on short inputs.
func shortCommit(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}
