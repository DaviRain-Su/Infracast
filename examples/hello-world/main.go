// Hello World Example Application for Infracast
// This demonstrates the simplest possible deployment with zero resources
package main

import (
	"context"
	"os"
	"time"
)

// Response represents a simple response
type Response struct {
	Message   string `json:"message"`
	Version   string `json:"version"`
	Env       string `json:"env"`
}

// Hello returns a greeting message
//encore:api public path=/hello
func Hello(ctx context.Context) (*Response, error) {
	return &Response{
		Message: "Hello, Infracast!",
		Version: os.Getenv("APP_VERSION"),
		Env:     os.Getenv("INFRA_ENV"),
	}, nil
}

// HealthCheck returns the service health status
//encore:api public path=/health
func HealthCheck(ctx context.Context) (*HealthResponse, error) {
	return &HealthResponse{
		Status:    "ok",
		Timestamp: time.Now().Unix(),
	}, nil
}

// HealthResponse represents health check response
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
}

func main() {
	// Minimal hello-world service
	// No database, no cache, no object storage
	// Just a simple HTTP service
}
