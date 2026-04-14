// Package infragen provides infrastructure configuration generation
package infragen

import "errors"

// Error codes for infragen module (EIGEN001-EIGEN003)
var (
	// ErrInvalidConfig indicates invalid configuration input
	ErrInvalidConfig = errors.New("EIGEN001: invalid configuration")
	// ErrMergeConflict indicates a merge conflict between base and override configs
	ErrMergeConflict = errors.New("EIGEN002: merge conflict")
	// ErrWriteFailed indicates failed to write configuration to file
	ErrWriteFailed = errors.New("EIGEN003: failed to write configuration")
)
