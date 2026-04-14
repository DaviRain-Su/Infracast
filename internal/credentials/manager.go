// Package credentials provides credential management for cloud providers
package credentials

import (
	"context"
	"fmt"
	"time"
)

// Credential represents cloud provider credentials
type Credential struct {
	Provider   string
	AccessKey  string
	SecretKey  string
	Region     string
	// STS credentials (optional, preferred for production)
	STS *STSCredential
}

// STSCredential represents temporary STS credentials
type STSCredential struct {
	RoleARN         string
	SessionName     string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Expiration      int64 // Unix timestamp
}

// CredentialConfig provides configuration for credential retrieval
type CredentialConfig struct {
	Provider    string        // Cloud provider name (e.g., "alicloud")
	Region      string        // Target region
	RefreshWindow time.Duration // Auto-refresh before expiration (default: 5min)
}

// Manager provides credential management
type Manager struct {
	// In-memory storage only (credentials never persisted to disk)
	credentials map[string]Credential
}

// NewManager creates a new credential manager
func NewManager() *Manager {
	return &Manager{
		credentials: make(map[string]Credential),
	}
}

// Store stores credentials in memory
func (m *Manager) Store(provider, accessKey, secretKey, region string) error {
	if provider == "" {
		return fmt.Errorf("ECRED001: provider is required")
	}
	if accessKey == "" {
		return fmt.Errorf("ECRED002: access key is required")
	}
	if secretKey == "" {
		return fmt.Errorf("ECRED003: secret key is required")
	}

	m.credentials[provider] = Credential{
		Provider:  provider,
		AccessKey: accessKey,
		SecretKey: secretKey,
		Region:    region,
	}
	return nil
}

// StoreWithSTS stores STS credentials in memory
func (m *Manager) StoreWithSTS(provider string, sts STSCredential, region string) error {
	if provider == "" {
		return fmt.Errorf("ECRED001: provider is required")
	}
	if sts.RoleARN == "" {
		return fmt.Errorf("ECRED004: role ARN is required")
	}

	m.credentials[provider] = Credential{
		Provider: provider,
		STS:      &sts,
		Region:   region,
	}
	return nil
}

// Get retrieves credentials for a provider
func (m *Manager) Get(provider string) (Credential, error) {
	cred, exists := m.credentials[provider]
	if !exists {
		return Credential{}, fmt.Errorf("ECRED005: credentials not found for provider: %s", provider)
	}
	return cred, nil
}

// GetForRegion retrieves credentials for a provider and region
func (m *Manager) GetForRegion(provider, region string) (Credential, error) {
	cred, exists := m.credentials[provider]
	if !exists {
		return Credential{}, fmt.Errorf("ECRED005: credentials not found for provider: %s", provider)
	}
	
	// If region specified, override
	if region != "" {
		cred.Region = region
	}
	
	return cred, nil
}

// Delete removes credentials for a provider
func (m *Manager) Delete(provider string) error {
	if _, exists := m.credentials[provider]; !exists {
		return fmt.Errorf("ECRED005: credentials not found for provider: %s", provider)
	}
	delete(m.credentials, provider)
	return nil
}

// List returns all stored provider names
func (m *Manager) List() []string {
	providers := make([]string, 0, len(m.credentials))
	for provider := range m.credentials {
		providers = append(providers, provider)
	}
	return providers
}

// STSClient interface for STS operations
type STSClient interface {
	AssumeRole(ctx context.Context, roleARN, sessionName string, durationSeconds int) (*STSCredential, error)
}

// RefreshSTS refreshes STS credentials for a provider
func (m *Manager) RefreshSTS(ctx context.Context, provider string, client STSClient) error {
	cred, exists := m.credentials[provider]
	if !exists {
		return fmt.Errorf("ECRED005: credentials not found for provider: %s", provider)
	}
	
	if cred.STS == nil {
		return fmt.Errorf("ECRED006: no STS configuration for provider: %s", provider)
	}

	newSTS, err := client.AssumeRole(ctx, cred.STS.RoleARN, cred.STS.SessionName, 3600)
	if err != nil {
		return fmt.Errorf("ECRED007: failed to refresh STS credentials: %w", err)
	}

	cred.STS = newSTS
	m.credentials[provider] = cred
	return nil
}

// IsSTS returns true if the provider uses STS credentials
func (m *Manager) IsSTS(provider string) bool {
	cred, exists := m.credentials[provider]
	if !exists {
		return false
	}
	return cred.STS != nil
}

// GetCredentials retrieves credentials with optional auto-refresh for STS
// Unified interface for credential retrieval (Tech Spec §4.2)
func (m *Manager) GetCredentials(config CredentialConfig) (*Credential, error) {
	cred, exists := m.credentials[config.Provider]
	if !exists {
		return nil, fmt.Errorf("ECRED005: credentials not found for provider: %s", config.Provider)
	}

	// Apply region override if specified
	if config.Region != "" {
		cred.Region = config.Region
	}

	// Check STS expiration and auto-refresh if needed
	if cred.STS != nil {
		refreshWindow := config.RefreshWindow
		if refreshWindow == 0 {
			refreshWindow = 5 * time.Minute // Default: refresh 5 min before expiration
		}

		expiration := time.Unix(cred.STS.Expiration, 0)
		if time.Until(expiration) < refreshWindow {
			// STS credentials need refresh - return error to trigger refresh flow
			return nil, fmt.Errorf("ECRED018: STS credentials expired or expiring soon for provider: %s", config.Provider)
		}
	}

	return &cred, nil
}
