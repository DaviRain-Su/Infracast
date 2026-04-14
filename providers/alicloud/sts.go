// Package alicloud provides Aliyun Cloud provider adapter
package alicloud

import (
	"fmt"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/auth/credentials"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/sts"
)

// STSClient wraps STS operations for AssumeRole
type STSClient struct {
	client *sts.Client
	region string
}

// NewSTSClient creates a new STS client
func NewSTSClient(region, accessKeyID, accessKeySecret string) (*STSClient, error) {
	config := sdk.NewConfig()
	cred := credentials.NewAccessKeyCredential(accessKeyID, accessKeySecret)
	
	client, err := sts.NewClientWithOptions(region, config, cred)
	if err != nil {
		return nil, fmt.Errorf("failed to create STS client: %w", err)
	}
	
	return &STSClient{
		client: client,
		region: region,
	}, nil
}

// AssumeRole assumes a RAM role and returns temporary credentials
func (s *STSClient) AssumeRole(roleARN, sessionName string, durationSeconds int) (*TemporaryCredentials, error) {
	if durationSeconds == 0 {
		durationSeconds = 3600 // Default 1 hour
	}
	if durationSeconds > 43200 {
		durationSeconds = 43200 // Max 12 hours
	}
	
	request := sts.CreateAssumeRoleRequest()
	request.RoleArn = roleARN
	request.RoleSessionName = sessionName
	request.DurationSeconds = requests.NewInteger(durationSeconds)
	
	response, err := s.client.AssumeRole(request)
	if err != nil {
		return nil, fmt.Errorf("failed to assume role: %w", err)
	}
	
	expiration, err := time.Parse("2006-01-02T15:04:05Z", response.Credentials.Expiration)
	if err != nil {
		return nil, fmt.Errorf("failed to parse expiration: %w", err)
	}
	
	return &TemporaryCredentials{
		AccessKeyID:     response.Credentials.AccessKeyId,
		AccessKeySecret: response.Credentials.AccessKeySecret,
		SecurityToken:   response.Credentials.SecurityToken,
		Expiration:      expiration,
		RoleARN:         roleARN,
	}, nil
}

// TemporaryCredentials holds STS temporary credentials
type TemporaryCredentials struct {
	AccessKeyID     string
	AccessKeySecret string
	SecurityToken   string
	Expiration      time.Time
	RoleARN         string
}

// IsExpired checks if the credentials are expired (with 5-minute buffer)
func (t *TemporaryCredentials) IsExpired() bool {
	return time.Now().After(t.Expiration.Add(-5 * time.Minute))
}

// RefreshableCredentials wraps temporary credentials with auto-refresh capability
type RefreshableCredentials struct {
	stsClient       *STSClient
	roleARN         string
	sessionName     string
	durationSeconds int
	current         *TemporaryCredentials
}

// NewRefreshableCredentials creates credentials that auto-refresh
func NewRefreshableCredentials(stsClient *STSClient, roleARN, sessionName string, durationSeconds int) *RefreshableCredentials {
	return &RefreshableCredentials{
		stsClient:       stsClient,
		roleARN:         roleARN,
		sessionName:     sessionName,
		durationSeconds: durationSeconds,
	}
}

// GetCredentials returns valid credentials, refreshing if necessary
func (r *RefreshableCredentials) GetCredentials() (*TemporaryCredentials, error) {
	if r.current == nil || r.current.IsExpired() {
		creds, err := r.stsClient.AssumeRole(r.roleARN, r.sessionName, r.durationSeconds)
		if err != nil {
			return nil, fmt.Errorf("failed to refresh credentials: %w", err)
		}
		r.current = creds
	}
	return r.current, nil
}
