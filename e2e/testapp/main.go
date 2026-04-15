package main

import "context"

type HealthResponse struct {
	Status string `json:"status"`
}

//encore:api public path=/health
func Health(_ context.Context) (*HealthResponse, error) {
	return &HealthResponse{Status: "ok"}, nil
}

func main() {}
