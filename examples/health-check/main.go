// Health Check Example — Minimal Observable Application for Infracast
//
// This example demonstrates:
//   - Health check endpoint with structured status reporting
//   - Dependency health checks (simulated DB/cache readiness)
//   - Failure mode demonstration (toggle via environment variable)
//   - Ready/Live probe endpoints for Kubernetes
//
// Use this to verify that your Infracast deploy pipeline works end-to-end,
// including health verification and failure detection.
package main

import (
	"context"
	"os"
	"runtime"
	"time"
)

var startTime = time.Now()

// LiveResponse is returned by the liveness probe
type LiveResponse struct {
	Status string `json:"status"`
	Uptime string `json:"uptime"`
}

// Livez is the Kubernetes liveness probe
//
//encore:api public path=/livez
func Livez(ctx context.Context) (*LiveResponse, error) {
	return &LiveResponse{
		Status: "ok",
		Uptime: time.Since(startTime).Round(time.Second).String(),
	}, nil
}

// ReadyResponse is returned by the readiness probe
type ReadyResponse struct {
	Status  string            `json:"status"`
	Checks  map[string]string `json:"checks"`
	Version string            `json:"version"`
	Env     string            `json:"env"`
	GoVer   string            `json:"go_version"`
}

// Readyz is the Kubernetes readiness probe — checks all dependencies
//
//encore:api public path=/readyz
func Readyz(ctx context.Context) (*ReadyResponse, error) {
	checks := map[string]string{}

	// Simulated dependency checks
	checks["self"] = "ok"

	// If SIMULATE_FAILURE is set, report unhealthy (for failure-mode demo)
	if os.Getenv("SIMULATE_FAILURE") == "true" {
		checks["self"] = "fail"
		return &ReadyResponse{
			Status:  "unhealthy",
			Checks:  checks,
			Version: os.Getenv("APP_VERSION"),
			Env:     os.Getenv("INFRA_ENV"),
			GoVer:   runtime.Version(),
		}, nil
	}

	return &ReadyResponse{
		Status:  "ready",
		Checks:  checks,
		Version: os.Getenv("APP_VERSION"),
		Env:     os.Getenv("INFRA_ENV"),
		GoVer:   runtime.Version(),
	}, nil
}

// DiagResponse is a diagnostic info dump
type DiagResponse struct {
	Hostname  string `json:"hostname"`
	GoVersion string `json:"go_version"`
	NumCPU    int    `json:"num_cpu"`
	Env       string `json:"env"`
	Uptime    string `json:"uptime"`
}

// Diag returns diagnostic information for troubleshooting
//
//encore:api public path=/diag
func Diag(ctx context.Context) (*DiagResponse, error) {
	hostname, _ := os.Hostname()
	return &DiagResponse{
		Hostname:  hostname,
		GoVersion: runtime.Version(),
		NumCPU:    runtime.NumCPU(),
		Env:       os.Getenv("INFRA_ENV"),
		Uptime:    time.Since(startTime).Round(time.Second).String(),
	}, nil
}

func main() {
	// Minimal observable service
	// No database, no cache — only health/readiness endpoints
	// Set SIMULATE_FAILURE=true to test failure detection
}
