// Package deploy provides deployment pipeline for Infracast
package deploy

import (
	"context"
	"fmt"
	"net/http"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HealthChecker checks deployment health status
type HealthChecker struct {
	k8sClient *K8sClient
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(k8sClient *K8sClient) *HealthChecker {
	return &HealthChecker{
		k8sClient: k8sClient,
	}
}

// DeploymentStatus represents the status of a deployment
type DeploymentStatus struct {
	Name               string
	Namespace          string
	Replicas           int32
	ReadyReplicas      int32
	UpdatedReplicas    int32
	AvailableReplicas  int32
	ObservedGeneration int64
	Conditions         []DeploymentCondition
}

// DeploymentCondition represents a deployment condition
type DeploymentCondition struct {
	Type               string
	Status             string
	Reason             string
	Message            string
	LastTransitionTime time.Time
}

// CheckStatus polls deployment status until ready or timeout
func (h *HealthChecker) CheckStatus(ctx context.Context, deploymentName string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Timeout reached - trigger rollback with fresh context
			rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer rollbackCancel()
			rollbackErr := h.rollbackOnTimeout(rollbackCtx, deploymentName)
			if rollbackErr != nil {
				return fmt.Errorf("EDEPLOY050: deployment timeout after %v and rollback failed: %w", timeout, rollbackErr)
			}
			return fmt.Errorf("EDEPLOY050: deployment timeout after %v, rollback triggered", timeout)

		case <-ticker.C:
			// Check deployment status
			status, err := h.getDeploymentStatus(ctx, deploymentName)
			if err != nil {
				return fmt.Errorf("failed to get deployment status: %w", err)
			}

			// Check if deployment is ready
			if h.isDeploymentReady(status) {
				return nil
			}

			// Check for failure conditions
			if failureReason := h.checkFailureConditions(status); failureReason != "" {
				// Trigger rollback on failure
				rollbackErr := h.rollbackOnFailure(ctx, deploymentName)
				if rollbackErr != nil {
					return fmt.Errorf("EDEPLOY050: deployment failed (%s) and rollback failed: %w", failureReason, rollbackErr)
				}
				return fmt.Errorf("EDEPLOY050: deployment failed (%s), rollback triggered", failureReason)
			}
		}
	}
}

// getDeploymentStatus retrieves current deployment status from K8s API
func (h *HealthChecker) getDeploymentStatus(ctx context.Context, deploymentName string) (*DeploymentStatus, error) {
	if h.k8sClient == nil || h.k8sClient.clientset == nil {
		return nil, fmt.Errorf("EDEPLOY051: K8s client not initialized")
	}

	// Get deployment from K8s API
	deployment, err := h.k8sClient.clientset.AppsV1().Deployments(h.k8sClient.namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("EDEPLOY052: failed to get deployment: %w", err)
	}

	// Convert to DeploymentStatus
	replicas := int32(1)
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}
	status := &DeploymentStatus{
		Name:               deployment.Name,
		Namespace:          deployment.Namespace,
		Replicas:           replicas,
		ReadyReplicas:      deployment.Status.ReadyReplicas,
		UpdatedReplicas:    deployment.Status.UpdatedReplicas,
		AvailableReplicas:  deployment.Status.AvailableReplicas,
		ObservedGeneration: deployment.Status.ObservedGeneration,
		Conditions:         make([]DeploymentCondition, len(deployment.Status.Conditions)),
	}

	// Convert conditions
	for i, cond := range deployment.Status.Conditions {
		status.Conditions[i] = DeploymentCondition{
			Type:               string(cond.Type),
			Status:             string(cond.Status),
			Reason:             cond.Reason,
			Message:            cond.Message,
			LastTransitionTime: cond.LastTransitionTime.Time,
		}
	}

	return status, nil
}

// isDeploymentReady checks if deployment is fully ready
func (h *HealthChecker) isDeploymentReady(status *DeploymentStatus) bool {
	// Deployment is ready when:
	// 1. ReadyReplicas >= Replicas
	// 2. AvailableReplicas >= Replicas
	// 3. No Progressing condition with Failed reason
	if status.ReadyReplicas < status.Replicas {
		return false
	}
	if status.AvailableReplicas < status.Replicas {
		return false
	}

	// Check conditions
	for _, cond := range status.Conditions {
		if cond.Type == "Progressing" && cond.Reason == "ProgressDeadlineExceeded" {
			return false
		}
	}

	return true
}

// checkFailureConditions checks for deployment failure conditions
func (h *HealthChecker) checkFailureConditions(status *DeploymentStatus) string {
	for _, cond := range status.Conditions {
		if cond.Type == "Progressing" && cond.Reason == "ProgressDeadlineExceeded" {
			return "progress deadline exceeded"
		}
		if cond.Type == "ReplicaFailure" && cond.Status == "True" {
			return fmt.Sprintf("replica failure: %s", cond.Message)
		}
	}

	// Check if replicas are consistently unavailable
	if status.ReadyReplicas == 0 && status.Replicas > 0 {
		// This could be normal during initial rollout, but if it persists
		// the timeout will catch it
		return ""
	}

	return ""
}

// rollbackOnTimeout triggers rollback when timeout is reached
func (h *HealthChecker) rollbackOnTimeout(ctx context.Context, deploymentName string) error {
	return h.k8sClient.RollbackUndo(ctx, deploymentName)
}

// rollbackOnFailure triggers rollback when deployment fails
func (h *HealthChecker) rollbackOnFailure(ctx context.Context, deploymentName string) error {
	return h.k8sClient.RollbackUndo(ctx, deploymentName)
}

// VerifyHealth performs a health check on the deployed application
func (h *HealthChecker) VerifyHealth(ctx context.Context, serviceName string, port int) error {
	if h.k8sClient == nil || h.k8sClient.clientset == nil {
		return fmt.Errorf("EDEPLOY053: K8s client not initialized")
	}

	// Get service endpoint
	service, err := h.k8sClient.clientset.CoreV1().Services(h.k8sClient.namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("EDEPLOY054: failed to get service: %w", err)
	}

	// Build health check URL (using cluster IP and port)
	var targetPort int32
	if len(service.Spec.Ports) > 0 {
		targetPort = service.Spec.Ports[0].Port
	} else {
		targetPort = int32(port)
	}

	healthURL := fmt.Sprintf("http://%s:%d/health", service.Spec.ClusterIP, targetPort)

	// Perform health check with timeout
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		return fmt.Errorf("EDEPLOY055: failed to create health check request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("EDEPLOY056: health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("EDEPLOY057: health check returned non-OK status: %d", resp.StatusCode)
	}

	return nil
}
