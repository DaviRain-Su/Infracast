package credentials

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockSTSClient implements STSClient for testing
type MockSTSClient struct {
	ShouldError bool
}

func (m *MockSTSClient) AssumeRole(ctx context.Context, roleARN, sessionName string, durationSeconds int) (*STSCredential, error) {
	if m.ShouldError {
		return nil, assert.AnError
	}
	return &STSCredential{
		RoleARN:         roleARN,
		SessionName:     sessionName,
		AccessKeyID:     "new-access-key",
		SecretAccessKey: "new-secret-key",
		SessionToken:    "new-session-token",
		Expiration:      3600,
	}, nil
}

func TestManager_Store(t *testing.T) {
	m := NewManager()

	tests := []struct {
		name      string
		provider  string
		accessKey string
		secretKey string
		region    string
		wantErr   bool
		errCode   string
	}{
		{
			name:      "valid credentials",
			provider:  "alicloud",
			accessKey: "AK123",
			secretKey: "SK456",
			region:    "cn-hangzhou",
			wantErr:   false,
		},
		{
			name:      "missing provider",
			provider:  "",
			accessKey: "AK123",
			secretKey: "SK456",
			wantErr:   true,
			errCode:   "ECRED001",
		},
		{
			name:      "missing access key",
			provider:  "alicloud",
			accessKey: "",
			secretKey: "SK456",
			wantErr:   true,
			errCode:   "ECRED002",
		},
		{
			name:      "missing secret key",
			provider:  "alicloud",
			accessKey: "AK123",
			secretKey: "",
			wantErr:   true,
			errCode:   "ECRED003",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.Store(tt.provider, tt.accessKey, tt.secretKey, tt.region)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errCode != "" {
					assert.Contains(t, err.Error(), tt.errCode)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestManager_StoreWithSTS(t *testing.T) {
	m := NewManager()

	sts := STSCredential{
		RoleARN:         "acs:ram::123456789012:role/MyRole",
		SessionName:     "session-1",
		AccessKeyID:     "STS123",
		SecretAccessKey: "STS456",
		SessionToken:    "token789",
	}

	err := m.StoreWithSTS("alicloud", sts, "cn-hangzhou")
	require.NoError(t, err)

	// Verify STS credentials stored
	cred, err := m.Get("alicloud")
	require.NoError(t, err)
	assert.NotNil(t, cred.STS)
	assert.Equal(t, "acs:ram::123456789012:role/MyRole", cred.STS.RoleARN)
	assert.True(t, m.IsSTS("alicloud"))
}

func TestManager_Get(t *testing.T) {
	m := NewManager()

	// Store credentials
	err := m.Store("alicloud", "AK123", "SK456", "cn-hangzhou")
	require.NoError(t, err)

	// Get existing
	cred, err := m.Get("alicloud")
	require.NoError(t, err)
	assert.Equal(t, "AK123", cred.AccessKey)
	assert.Equal(t, "SK456", cred.SecretKey)

	// Get non-existent
	_, err = m.Get("unknown")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ECRED005")
}

func TestManager_GetForRegion(t *testing.T) {
	m := NewManager()

	// Store with default region
	err := m.Store("alicloud", "AK123", "SK456", "cn-hangzhou")
	require.NoError(t, err)

	// Get with override region
	cred, err := m.GetForRegion("alicloud", "cn-shanghai")
	require.NoError(t, err)
	assert.Equal(t, "cn-shanghai", cred.Region)
}

func TestManager_Delete(t *testing.T) {
	m := NewManager()

	// Store and delete
	err := m.Store("alicloud", "AK123", "SK456", "cn-hangzhou")
	require.NoError(t, err)

	err = m.Delete("alicloud")
	require.NoError(t, err)

	// Verify deleted
	_, err = m.Get("alicloud")
	assert.Error(t, err)

	// Delete non-existent
	err = m.Delete("unknown")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ECRED005")
}

func TestManager_List(t *testing.T) {
	m := NewManager()

	// Initially empty
	list := m.List()
	assert.Empty(t, list)

	// Store multiple
	m.Store("alicloud", "AK1", "SK1", "cn-hangzhou")
	m.Store("huaweicloud", "AK2", "SK2", "cn-north-4")

	list = m.List()
	assert.Len(t, list, 2)
	assert.Contains(t, list, "alicloud")
	assert.Contains(t, list, "huaweicloud")
}

func TestManager_RefreshSTS(t *testing.T) {
	m := NewManager()

	// Store STS credentials
	sts := STSCredential{
		RoleARN:     "acs:ram::123456789012:role/MyRole",
		SessionName: "session-1",
	}
	err := m.StoreWithSTS("alicloud", sts, "cn-hangzhou")
	require.NoError(t, err)

	// Refresh with mock client
	mockClient := &MockSTSClient{}
	err = m.RefreshSTS(context.Background(), "alicloud", mockClient)
	require.NoError(t, err)

	// Verify refreshed
	cred, err := m.Get("alicloud")
	require.NoError(t, err)
	assert.Equal(t, "new-access-key", cred.STS.AccessKeyID)
	assert.Equal(t, "new-session-token", cred.STS.SessionToken)
}

func TestManager_RefreshSTS_NotFound(t *testing.T) {
	m := NewManager()
	mockClient := &MockSTSClient{}

	err := m.RefreshSTS(context.Background(), "unknown", mockClient)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ECRED005")
}

func TestManager_RefreshSTS_NoSTS(t *testing.T) {
	m := NewManager()

	// Store non-STS credentials
	m.Store("alicloud", "AK123", "SK456", "cn-hangzhou")

	mockClient := &MockSTSClient{}
	err := m.RefreshSTS(context.Background(), "alicloud", mockClient)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ECRED006")
}

func TestManager_IsSTS(t *testing.T) {
	m := NewManager()

	// Non-STS
	m.Store("alicloud", "AK123", "SK456", "cn-hangzhou")
	assert.False(t, m.IsSTS("alicloud"))

	// STS
	sts := STSCredential{RoleARN: "role-arn"}
	m.StoreWithSTS("huaweicloud", sts, "cn-north-4")
	assert.True(t, m.IsSTS("huaweicloud"))

	// Non-existent
	assert.False(t, m.IsSTS("unknown"))
}
