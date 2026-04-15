package deploy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNewRollbackManager validates rollback manager creation
func TestNewRollbackManager(t *testing.T) {
	k8sClient := &K8sClient{}
	manager := NewRollbackManager(k8sClient)

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.k8sClient)
	assert.NotNil(t, manager.state)
}

// TestRollbackStateFields validates rollback state struct
func TestRollbackStateFields(t *testing.T) {
	state := &RollbackState{
		DeploymentName: "myapp",
		Namespace:      "default",
		TargetRevision: 3,
		OriginalImage:  "myapp:v1.0.0",
		StartedAt:      time.Now(),
	}

	assert.Equal(t, "myapp", state.DeploymentName)
	assert.Equal(t, "default", state.Namespace)
	assert.Equal(t, int64(3), state.TargetRevision)
	assert.Equal(t, "myapp:v1.0.0", state.OriginalImage)
}

// TestRollbackResultFields validates rollback result struct
func TestRollbackResultFields(t *testing.T) {
	result := &RollbackResult{
		Success:       true,
		PreviousImage: "myapp:v1.0.0",
		NewImage:      "myapp:v1.0.1",
		Duration:      30 * time.Second,
		Error:         nil,
	}

	assert.True(t, result.Success)
	assert.Equal(t, "myapp:v1.0.0", result.PreviousImage)
	assert.Equal(t, "myapp:v1.0.1", result.NewImage)
	assert.Equal(t, 30*time.Second, result.Duration)
}

// TestIsRollbackInProgress validates in-progress detection
func TestIsRollbackInProgress(t *testing.T) {
	k8sClient := &K8sClient{}
	manager := NewRollbackManager(k8sClient)

	// No rollback in progress
	assert.False(t, manager.IsRollbackInProgress())

	// Simulate rollback started
	manager.state.StartedAt = time.Now()
	manager.state.DeploymentName = "myapp"
	assert.True(t, manager.IsRollbackInProgress())

	// Clear state
	manager.ClearState()
	assert.False(t, manager.IsRollbackInProgress())
}

// TestGetRollbackStatus validates status retrieval
func TestGetRollbackStatus(t *testing.T) {
	k8sClient := &K8sClient{}
	manager := NewRollbackManager(k8sClient)

	// Initially empty
	status := manager.GetRollbackStatus()
	assert.Empty(t, status.DeploymentName)

	// After setting state
	manager.state.DeploymentName = "myapp"
	status = manager.GetRollbackStatus()
	assert.Equal(t, "myapp", status.DeploymentName)
}

// TestRollbackStrategies validates strategy constants
func TestRollbackStrategies(t *testing.T) {
	assert.Equal(t, RollbackStrategy(0), RollbackStrategyK8s)
	assert.Equal(t, RollbackStrategy(1), RollbackStrategyImage)
}

// TestHasPreviousRevision validates revision check
func TestHasPreviousRevision(t *testing.T) {
	k8sClient := &K8sClient{}
	manager := NewRollbackManager(k8sClient)

	// Returns error when K8s client is not initialized
	_, err := manager.hasPreviousRevision(nil, "myapp")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EDEPLOY060")
}

// TestValidateRollbackSafety validates safety checks
func TestValidateRollbackSafety(t *testing.T) {
	k8sClient := &K8sClient{}
	manager := NewRollbackManager(k8sClient)

	// Returns error when K8s client is not initialized
	err := manager.validateRollbackSafety(nil, "myapp")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EDEPLOY065")
}

// TestIsRollbackStable validates stable check
func TestIsRollbackStable(t *testing.T) {
	k8sClient := &K8sClient{}
	manager := NewRollbackManager(k8sClient)

	// Returns error when K8s client is not initialized
	_, err := manager.isRollbackStable(nil, "myapp")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EDEPLOY062")
}
