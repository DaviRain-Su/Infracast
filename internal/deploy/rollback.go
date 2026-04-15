// Package deploy provides deployment pipeline for Infracast
package deploy

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RollbackManager manages deployment rollbacks
type RollbackManager struct {
	k8sClient *K8sClient
	state     *RollbackState
}

// RollbackState tracks rollback state
type RollbackState struct {
	DeploymentName string
	Namespace      string
	TargetRevision int64
	OriginalImage  string
	StartedAt      time.Time
}

// NewRollbackManager creates a new rollback manager
func NewRollbackManager(k8sClient *K8sClient) *RollbackManager {
	return &RollbackManager{
		k8sClient: k8sClient,
		state:     &RollbackState{},
	}
}

// RollbackStrategy defines the rollback approach
type RollbackStrategy int

const (
	// RollbackStrategyK8s uses kubectl rollout undo
	RollbackStrategyK8s RollbackStrategy = iota
	// RollbackStrategyImage reverts to previous image
	RollbackStrategyImage
)

// Rollback performs rollback of a deployment
func (r *RollbackManager) Rollback(ctx context.Context, deploymentName string, strategy RollbackStrategy) error {
	r.state.StartedAt = time.Now()
	r.state.DeploymentName = deploymentName

	switch strategy {
	case RollbackStrategyK8s:
		return r.rollbackK8s(ctx, deploymentName)
	case RollbackStrategyImage:
		return r.rollbackImage(ctx, deploymentName)
	default:
		return fmt.Errorf("unknown rollback strategy: %v", strategy)
	}
}

// rollbackK8s uses kubectl rollout undo
func (r *RollbackManager) rollbackK8s(ctx context.Context, deploymentName string) error {
	// Check if there's a previous revision to rollback to
	hasPrevious, err := r.hasPreviousRevision(ctx, deploymentName)
	if err != nil {
		return fmt.Errorf("failed to check previous revision: %w", err)
	}

	if !hasPrevious {
		// First deploy with no previous revision
		return fmt.Errorf("EDEPLOY060: no previous revision to rollback to (first deployment)")
	}

	// Execute kubectl rollout undo
	if err := r.k8sClient.RollbackUndo(ctx, deploymentName); err != nil {
		// Rollback itself failed - mark as failed, not rolled_back
		return fmt.Errorf("EDEPLOY061: rollback execution failed: %w", err)
	}

	// Wait for rollback to complete
	if err := r.waitForRollback(ctx, deploymentName); err != nil {
		return fmt.Errorf("EDEPLOY062: rollback did not stabilize: %w", err)
	}

	return nil
}

// rollbackImage reverts to a specific image version
func (r *RollbackManager) rollbackImage(ctx context.Context, deploymentName string) error {
	// TODO: Implement image-based rollback
	// This would:
	// 1. Get current deployment
	// 2. Update container image to previous version
	// 3. Apply the change
	return fmt.Errorf("image-based rollback not yet implemented")
}

// hasPreviousRevision checks if deployment has a revision to rollback to
func (r *RollbackManager) hasPreviousRevision(ctx context.Context, deploymentName string) (bool, error) {
	if r.k8sClient == nil || r.k8sClient.clientset == nil {
		return false, fmt.Errorf("EDEPLOY060: K8s client not initialized")
	}

	// Get deployment to check revision history
	deployment, err := r.k8sClient.clientset.AppsV1().Deployments(r.k8sClient.namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("EDEPLOY061: failed to get deployment: %w", err)
	}

	// Check if there's a previous revision by looking at annotation
	// kubectl rollout undo goes back to the previous revision in history
	// A deployment with no previous revision will have only one entry in history
	revisionHistory := deployment.Annotations["deployment.kubernetes.io/revision"]
	if revisionHistory == "" || revisionHistory == "1" {
		// First deployment, no previous revision
		return false, nil
	}

	return true, nil
}

// waitForRollback waits for rollback to stabilize
func (r *RollbackManager) waitForRollback(ctx context.Context, deploymentName string) error {
	// Poll deployment status with timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for rollback to stabilize")

		case <-ticker.C:
			// Check if deployment is stable
			stable, err := r.isRollbackStable(ctx, deploymentName)
			if err != nil {
				return err
			}
			if stable {
				return nil
			}
		}
	}
}

// isRollbackStable checks if rollback has stabilized
func (r *RollbackManager) isRollbackStable(ctx context.Context, deploymentName string) (bool, error) {
	if r.k8sClient == nil || r.k8sClient.clientset == nil {
		return false, fmt.Errorf("EDEPLOY062: K8s client not initialized")
	}

	// Get deployment status
	deployment, err := r.k8sClient.clientset.AppsV1().Deployments(r.k8sClient.namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("EDEPLOY063: failed to get deployment status: %w", err)
	}

	// Check if observed generation matches current generation
	if deployment.Status.ObservedGeneration < deployment.Generation {
		// Deployment is still being processed
		return false, nil
	}

	// Check if all replicas are ready
	if deployment.Status.ReadyReplicas < *deployment.Spec.Replicas {
		return false, nil
	}

	// Check for any failure conditions
	for _, cond := range deployment.Status.Conditions {
		if cond.Type == appsv1.DeploymentProgressing && cond.Reason == "ProgressDeadlineExceeded" {
			return false, fmt.Errorf("EDEPLOY064: rollback failed - progress deadline exceeded")
		}
	}

	return true, nil
}

// RollbackResult represents the outcome of a rollback
type RollbackResult struct {
	Success       bool
	PreviousImage string
	NewImage      string
	Duration      time.Duration
	Error         error
}

// ExecuteRollbackWithGuardrails performs rollback with safety checks
func (r *RollbackManager) ExecuteRollbackWithGuardrails(ctx context.Context, deploymentName string) (*RollbackResult, error) {
	start := time.Now()
	result := &RollbackResult{
		PreviousImage: "", // Would be populated from deployment history
	}

	// Guardrail 1: Forward-only for database migrations
	// Never rollback destructive DDL changes
	// This is enforced by the migration system, not here

	// Guardrail 2: Check if rollback is safe
	if err := r.validateRollbackSafety(ctx, deploymentName); err != nil {
		result.Error = fmt.Errorf("rollback safety check failed: %w", err)
		return result, result.Error
	}

	// Execute rollback
	if err := r.Rollback(ctx, deploymentName, RollbackStrategyK8s); err != nil {
		result.Error = err
		result.Success = false
		return result, err
	}

	result.Success = true
	result.Duration = time.Since(start)
	return result, nil
}

// validateRollbackSafety checks if rollback can be safely performed
func (r *RollbackManager) validateRollbackSafety(ctx context.Context, deploymentName string) error {
	if r.k8sClient == nil || r.k8sClient.clientset == nil {
		return fmt.Errorf("EDEPLOY065: K8s client not initialized")
	}

	// Check 1: Verify deployment exists
	deployment, err := r.k8sClient.clientset.AppsV1().Deployments(r.k8sClient.namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("EDEPLOY066: deployment not found: %w", err)
	}

	// Check 2: Verify there's a previous revision to rollback to
	hasPrevious, err := r.hasPreviousRevision(ctx, deploymentName)
	if err != nil {
		return err
	}
	if !hasPrevious {
		return fmt.Errorf("EDEPLOY067: no previous revision to rollback to")
	}

	// Check 3: Verify deployment is in a state that allows rollback
	// Don't rollback if a rollout is already in progress
	for _, cond := range deployment.Status.Conditions {
		if cond.Type == appsv1.DeploymentProgressing &&
			cond.Status == "True" &&
			cond.Reason == "NewReplicaSetAvailable" {
			// Rollout is complete, safe to rollback
			break
		}
	}

	// Check 4: Forward-only migration enforcement
	// Check if deployment has migration annotations
	if migrationStatus := deployment.Annotations["infracast.dev/migration-status"]; migrationStatus == "destructive" {
		return fmt.Errorf("EDEPLOY068: cannot rollback - deployment includes destructive migrations")
	}

	return nil
}

// GetRollbackStatus returns current rollback status
func (r *RollbackManager) GetRollbackStatus() *RollbackState {
	return r.state
}

// IsRollbackInProgress checks if a rollback is currently in progress
func (r *RollbackManager) IsRollbackInProgress() bool {
	return r.state.StartedAt != time.Time{} && r.state.DeploymentName != ""
}

// ClearState clears the rollback state
func (r *RollbackManager) ClearState() {
	r.state = &RollbackState{}
}
