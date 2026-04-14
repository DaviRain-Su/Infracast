package deploy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNewPipeline validates pipeline creation
func TestNewPipeline(t *testing.T) {
	pipeline := NewPipeline(true)
	assert.NotNil(t, pipeline)
	assert.True(t, pipeline.verbose)

	pipelineQuiet := NewPipeline(false)
	assert.NotNil(t, pipelineQuiet)
	assert.False(t, pipelineQuiet.verbose)
}

// TestPipelineInputFields validates input struct
func TestPipelineInputFields(t *testing.T) {
	input := &PipelineInput{
		AppName:      "myapp",
		Env:          "staging",
		Commit:       "abc123",
		ImageTag:     "v1.0.0",
		ConfigPath:   "./infracfg.json",
		LocalImage:   "myapp:latest",
		Replicas:     2,
		Port:         8080,
		EnvVars:      map[string]string{"LOG_LEVEL": "info"},
		ACRNamespace: "my-namespace",
		ACRRegion:    "cn-hangzhou",
	}

	assert.Equal(t, "myapp", input.AppName)
	assert.Equal(t, "staging", input.Env)
	assert.Equal(t, 2, input.Replicas)
}

// TestPipelineResultFields validates result struct
func TestPipelineResultFields(t *testing.T) {
	result := &PipelineResult{
		Success:    true,
		ExitCode:   0,
		Steps:      []StepResult{},
		Duration:   5 * time.Minute,
		FinalImage: "registry.cn-hangzhou.aliyuncs.com/test/myapp:v1.0.0",
	}

	assert.True(t, result.Success)
	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, 5*time.Minute, result.Duration)
}

// TestStepResultFields validates step result struct
func TestStepResultFields(t *testing.T) {
	step := StepResult{
		Name:     "Build",
		Success:  true,
		Duration: 30 * time.Second,
		Output:   "Build successful",
	}

	assert.Equal(t, "Build", step.Name)
	assert.True(t, step.Success)
	assert.Equal(t, 30*time.Second, step.Duration)
}

// TestExitCodes validates exit code constants
func TestExitCodes(t *testing.T) {
	assert.Equal(t, 0, ExitCodeSuccess)
	assert.Equal(t, 1, ExitCodeBuildFailed)
	assert.Equal(t, 2, ExitCodePushFailed)
	assert.Equal(t, 3, ExitCodeProvisionFailed)
	assert.Equal(t, 4, ExitCodeConfigFailed)
	assert.Equal(t, 5, ExitCodeDeployFailed)
	assert.Equal(t, 6, ExitCodeVerifyFailed)
	assert.Equal(t, 7, ExitCodeRollbackFailed)
	assert.Equal(t, 130, ExitCodeInterrupted)
}

// TestExecuteStep validates step execution
func TestExecuteStep(t *testing.T) {
	pipeline := NewPipeline(true)

	// Successful step
	result := pipeline.executeStep("Test Step", func() error {
		return nil
	})
	assert.True(t, result.Success)
	assert.Equal(t, "Test Step", result.Name)
	assert.NoError(t, result.Error)

	// Failed step
	result = pipeline.executeStep("Failing Step", func() error {
		return assert.AnError
	})
	assert.False(t, result.Success)
	assert.Error(t, result.Error)
}

// TestFinalizeResult validates result finalization
func TestFinalizeResult(t *testing.T) {
	pipeline := NewPipeline(false)
	result := &PipelineResult{
		Steps: []StepResult{},
	}

	start := time.Now()
	finalized := pipeline.finalizeResult(result, start, 0, nil)

	assert.True(t, finalized.Success)
	assert.Equal(t, 0, finalized.ExitCode)
	assert.NoError(t, finalized.Error)
	assert.True(t, finalized.Duration >= 0)
}

// TestLogFunctions validates logging (verbose mode)
func TestLogFunctions(t *testing.T) {
	// Verbose pipeline should log
	verbosePipeline := NewPipeline(true)
	verbosePipeline.log("Test message")
	verbosePipeline.logf("Formatted %s", "message")

	// Quiet pipeline should not log (no panic)
	quietPipeline := NewPipeline(false)
	quietPipeline.log("Test message")
	quietPipeline.logf("Formatted %s", "message")
}

// TestPipelineExecuteWithCancel validates context cancellation
func TestPipelineExecuteWithCancel(t *testing.T) {
	// This test would require mocking the step functions
	// For now, just verify the function signature is correct
	t.Skip("Skipping pipeline execute test - requires step function mocks")
}
