// Package infragen provides infrastructure configuration generation
package infragen

// InfraConfig represents the complete infrastructure configuration
// Aligns with Encore's infracfg.rs schema
type InfraConfig struct {
	SQLServers    map[string]SQLServer   `json:"sql_servers,omitempty"`
	Redis         map[string]RedisServer `json:"redis,omitempty"`
	ObjectStorage map[string]ObjectStore `json:"object_storage,omitempty"`
}

// SQLServer represents a SQL database server configuration
type SQLServer struct {
	Host     string     `json:"host"`
	Port     int        `json:"port"`
	Database string     `json:"database"`
	User     string     `json:"user"`
	Password string     `json:"password"` // Reference to environment variable
	TLS      *TLSConfig `json:"tls,omitempty"`
}

// RedisServer represents a Redis cache server configuration
type RedisServer struct {
	Host      string      `json:"host"`
	Port      int         `json:"port"`
	Password  string      `json:"password,omitempty"` // Reference to environment variable
	Auth      *AuthConfig `json:"auth,omitempty"`
	KeyPrefix string      `json:"key_prefix,omitempty"`
	Database  int         `json:"db,omitempty"`
	TLS       *TLSConfig  `json:"tls,omitempty"`
}

// ObjectStore represents an object storage configuration (S3-compatible)
type ObjectStore struct {
	Type      string `json:"type"` // S3
	Endpoint  string `json:"endpoint"`
	Bucket    string `json:"bucket"`
	Region    string `json:"region"`
	Provider  string `json:"provider,omitempty"`   // alicloud, aws, etc.
	AccessKey string `json:"access_key,omitempty"` // Reference to env var
	SecretKey string `json:"secret_key,omitempty"` // Reference to env var
}

// TLSConfig represents TLS configuration
type TLSConfig struct {
	Enabled            bool   `json:"enabled"`
	CAFile             string `json:"ca_file,omitempty"`
	CertFile           string `json:"cert_file,omitempty"`
	KeyFile            string `json:"key_file,omitempty"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify,omitempty"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Enabled  bool   `json:"enabled"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"` // Reference to env var
}
