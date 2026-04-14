// Package provisioner provides infrastructure provisioning orchestration
package provisioner

import "errors"

// Error codes for provisioner module (EPROV001-EPROV010)
var (
	// EPROV001: Plan generation failed
	ErrPlanFailed = errors.New("EPROV001: failed to generate provisioning plan")

	// EPROV002: Provider not found
	ErrProviderNotFound = errors.New("EPROV002: cloud provider not found")

	// EPROV003: State update failed
	ErrStateUpdateFailed = errors.New("EPROV003: failed to update resource state")

	// EPROV004: Resource provisioning failed
	ErrProvisionFailed = errors.New("EPROV004: resource provisioning failed")

	// EPROV005: Invalid resource type
	ErrInvalidResourceType = errors.New("EPROV005: invalid resource type")

	// EPROV006: Dependency not met
	ErrDependencyNotMet = errors.New("EPROV006: resource dependency not met")

	// EPROV007: Destroy operation failed
	ErrDestroyFailed = errors.New("EPROV007: resource destruction failed")

	// EPROV008: Spec hash computation failed
	ErrHashFailed = errors.New("EPROV008: failed to compute spec hash")

	// EPROV009: Resource not found
	ErrResourceNotFound = errors.New("EPROV009: resource not found")

	// EPROV010: Concurrency conflict
	ErrConcurrencyConflict = errors.New("EPROV010: concurrent update detected")
)

// Retryable returns true if the error is retryable
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, ErrPlanFailed),
		errors.Is(err, ErrStateUpdateFailed),
		errors.Is(err, ErrConcurrencyConflict):
		return true
	default:
		return false
	}
}

// SideEffect returns true if the error may have caused side effects
func HasSideEffect(err error) bool {
	if err == nil {
		return false
	}
	// Most provision errors have side effects (partial provisioning)
	switch {
	case errors.Is(err, ErrProvisionFailed),
		errors.Is(err, ErrDestroyFailed):
		return true
	default:
		return false
	}
}
