package deploy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsDeploymentReady validates deployment readiness check
func TestIsDeploymentReady(t *testing.T) {
	checker := &HealthChecker{}

	// Ready deployment
	readyStatus := &DeploymentStatus{
		Replicas:          2,
		ReadyReplicas:     2,
		AvailableReplicas: 2,
		Conditions: []DeploymentCondition{
			{Type: "Available", Status: "True"},
		},
	}
	assert.True(t, checker.isDeploymentReady(readyStatus))

	// Not ready - insufficient ready replicas
	notReadyStatus := &DeploymentStatus{
		Replicas:          2,
		ReadyReplicas:     1,
		AvailableReplicas: 2,
	}
	assert.False(t, checker.isDeploymentReady(notReadyStatus))

	// Not ready - insufficient available replicas
	notAvailableStatus := &DeploymentStatus{
		Replicas:          2,
		ReadyReplicas:     2,
		AvailableReplicas: 1,
	}
	assert.False(t, checker.isDeploymentReady(notAvailableStatus))

	// Not ready - deadline exceeded
	deadlineStatus := &DeploymentStatus{
		Replicas:          2,
		ReadyReplicas:     2,
		AvailableReplicas: 2,
		Conditions: []DeploymentCondition{
			{Type: "Progressing", Reason: "ProgressDeadlineExceeded"},
		},
	}
	assert.False(t, checker.isDeploymentReady(deadlineStatus))
}

// TestCheckFailureConditions validates failure detection
func TestCheckFailureConditions(t *testing.T) {
	checker := &HealthChecker{}

	// No failure
	okStatus := &DeploymentStatus{
		Conditions: []DeploymentCondition{
			{Type: "Available", Status: "True"},
		},
	}
	assert.Empty(t, checker.checkFailureConditions(okStatus))

	// Progress deadline exceeded
	deadlineStatus := &DeploymentStatus{
		Conditions: []DeploymentCondition{
			{Type: "Progressing", Reason: "ProgressDeadlineExceeded"},
		},
	}
	assert.Contains(t, checker.checkFailureConditions(deadlineStatus), "progress deadline exceeded")

	// Replica failure
	replicaFailureStatus := &DeploymentStatus{
		Conditions: []DeploymentCondition{
			{Type: "ReplicaFailure", Status: "True", Message: "insufficient quota"},
		},
	}
	assert.Contains(t, checker.checkFailureConditions(replicaFailureStatus), "replica failure")
}

// TestDeploymentStatus struct fields
func TestDeploymentStatusFields(t *testing.T) {
	status := &DeploymentStatus{
		Name:               "myapp",
		Namespace:          "default",
		Replicas:           3,
		ReadyReplicas:      2,
		UpdatedReplicas:    3,
		AvailableReplicas:  2,
		ObservedGeneration: 5,
		Conditions: []DeploymentCondition{
			{
				Type:   "Available",
				Status: "True",
				Reason: "MinimumReplicasAvailable",
			},
		},
	}

	assert.Equal(t, "myapp", status.Name)
	assert.Equal(t, "default", status.Namespace)
	assert.Equal(t, int32(3), status.Replicas)
	assert.Equal(t, int32(2), status.ReadyReplicas)
}

// TestNewHealthChecker validates health checker creation
func TestNewHealthChecker(t *testing.T) {
	k8sClient := &K8sClient{}
	checker := NewHealthChecker(k8sClient)

	assert.NotNil(t, checker)
	assert.Equal(t, k8sClient, checker.k8sClient)
}

// TestCheckStatusTimeout validates timeout handling
func TestCheckStatusTimeout(t *testing.T) {
	// This test would require mocking the k8s client
	// For now, just verify the function signature
	t.Skip("Skipping timeout test - requires K8s client mock")
}
