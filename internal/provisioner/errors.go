// Package provisioner provides infrastructure provisioning orchestration
package provisioner

import "errors"

// ProvisionError represents a structured provisioning error
type ProvisionError struct {
	ResourceName string // Resource that caused the error
	Code         string // Error code (EPROVxxx)
	Message      string // Human-readable message
	Retryable    bool   // Whether the error is retryable
	Cause        error  // Underlying cause
}

func (e *ProvisionError) Error() string {
	return e.Code + ": " + e.Message
}

// Unwrap returns the underlying error
func (e *ProvisionError) Unwrap() error {
	return e.Cause
}

// Error codes for provisioner module (EPROV001-EPROV010)
// Aligned with Tech Spec §3.5
var (
	// EPROV001: Credential fetch failed (from credentials module)
	ErrCredentialFetch = errors.New("EPROV001: failed to fetch credentials")

	// EPROV002: SDK retryable error (cloud provider SDK returned retryable error)
	ErrSDKRetryable = errors.New("EPROV002: cloud provider SDK retryable error")

	// EPROV003: Quota exceeded (insufficient cloud resource quota)
	ErrQuotaExceeded = errors.New("EPROV003: cloud resource quota exceeded")

	// EPROV004: Dependency conflict (resource dependencies cannot be satisfied)
	ErrDependencyConflict = errors.New("EPROV004: resource dependency conflict")

	// EPROV005: Invalid spec (invalid resource specification)
	ErrInvalidSpec = errors.New("EPROV005: invalid resource specification")

	// Legacy/Internal errors (EPROV006-010)
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

	// Legacy error (used internally)
	ErrInvalidResourceType = errors.New("invalid resource type")
)

// Retryable returns true if the error is retryable
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	// Check ProvisionError
	if pe, ok := err.(*ProvisionError); ok {
		return pe.Retryable
	}
	switch {
	case errors.Is(err, ErrCredentialFetch),
		errors.Is(err, ErrSDKRetryable),
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
	case errors.Is(err, ErrDestroyFailed):
		return true
	default:
		// Check if it's a ProvisionError with resource name (indicates partial failure)
		if pe, ok := err.(*ProvisionError); ok && pe.ResourceName != "" {
			return true
		}
		return false
	}
}
