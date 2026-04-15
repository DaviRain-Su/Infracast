# Milestone A Definition of Done (DoD)

**Version**: 1.2  
**Date**: 2026-04-15  
**Status**: ✅ Completed  

## Overview

Milestone A (Week 1-2) establishes the foundational infrastructure for Infracast. This document defines the acceptance criteria and quality gates for completion.

## 1. Project Skeleton (Task 1)

### Requirements
- [x] Go module initialized with path `github.com/DaviRain-Su/infracast`
- [x] Directory structure following monorepo layout:
  - `cmd/infracast/` - CLI entry point
  - `internal/` - Internal packages
  - `providers/` - Cloud provider adapters
  - `pkg/` - Public packages
  - `docs/` - Documentation
- [x] Build system with Makefile supporting: `build`, `test`, `lint`, `clean`
- [x] CI workflow placeholder (.github/workflows/ci.yml)
- [x] .gitignore properly configured

### Acceptance Criteria
```bash
$ make build
Building infracast...
Built: bin/infracast

$ ./bin/infracast version
infracast version dev
  commit: unknown
  build time: unknown

$ make test
Running tests...
ok      github.com/DaviRain-Su/infracast/...    0.001s
```

## 2. CLI Command Structure (Tasks 1, 8)

### Required Commands
- [x] `infracast version` - Display version information
- [x] `infracast init --provider <p> --region <r>` - Initialize project
- [x] `infracast run` - Run locally (mock mode)
- [x] `infracast deploy --env <e>` - Deploy to environment
- [x] `infracast env list/create/destroy` - Environment management

### Acceptance Criteria
```bash
$ infracast --help
Infracast - Code-First infrastructure automation for Chinese clouds

Usage:
  infracast [command]

Available Commands:
  deploy      Deploy application to cloud environment
  env         Manage environments
  help        Help about any command
  init        Initialize a new infracast project
  run         Run application locally
  version     Print version information
```

## 3. Configuration Model (Task 2)

### Schema Alignment
`infracast.yaml` must support:

```yaml
provider: alicloud        # Required: cloud provider
region: cn-hangzhou       # Required: cloud region

environments:
  production:
    provider: alicloud
    region: cn-shanghai
  staging:
    provider: alicloud
    region: cn-hangzhou

overrides:
  databases:
    mydb:
      instance_class: rds.mysql.s3.large
  compute:
    api-service:
      replicas: 3
```

### Validation Requirements
- [x] `provider` is required and must be in whitelist (alicloud)
- [x] `region` is required and non-empty
- [x] Environment names must be valid (alphanumeric + hyphen)

### Acceptance Criteria
```go
// Test case: Valid config passes
config := &Config{
    Provider: "alicloud",
    Region:   "cn-hangzhou",
}
err := Validate(config) // err == nil

// Test case: Missing provider fails
config := &Config{}
err := Validate(config) // err != nil, contains "provider is required"
```

## 4. Infra Config Generator (Task 5)

### Schema Alignment with Encore

The generated `infracfg.json` must align with Encore's self-host configuration schema:

```json
{
  "sql_databases": {
    "mydb": {
      "host": "rm-xxx.mysql.rds.aliyuncs.com",
      "port": 3306,
      "database": "mydb",
      "username": "app",
      "password": "${DB_PASSWORD}"
    }
  },
  "redis": {
    "cache": {
      "host": "r-xxx.redis.rds.aliyuncs.com",
      "port": 6379
    }
  },
  "object_storage": {
    "assets": {
      "type": "s3",
      "endpoint": "https://oss-cn-hangzhou.aliyuncs.com",
      "bucket": "mybucket",
      "region": "cn-hangzhou"
    }
  }
}
```

### Key Fields to Support
- [x] `sql_databases` - Map of database name to connection config
- [x] `redis` - Map of cache name to connection config  
- [x] `object_storage` - Map of bucket name to S3-compatible config

### Acceptance Criteria
```go
// Given ResourceSpec, generate valid infracfg
spec := &ResourceSpec{
    Databases: []DatabaseSpec{{Name: "mydb", ...}},
}
config := GenerateInfracfg(spec)

// Output must be valid JSON matching Encore schema
// Must pass json.Unmarshal without error
```

## 5. Provider Interface (Task 4, 6)

### CloudProviderInterface

```go
type CloudProviderInterface interface {
    Name() string
    DisplayName() string
    Regions() []Region
    
    // P0 Resources Only
    ProvisionDatabase(spec DatabaseSpec) (*DatabaseOutput, error)
    ProvisionCache(spec CacheSpec) (*CacheOutput, error)
    ProvisionObjectStorage(spec ObjectStorageSpec) (*ObjectStorageOutput, error)
    ProvisionCompute(spec ComputeSpec) (*ComputeOutput, error)
    
    // Lifecycle
    Plan(specs []ResourceSpec) (*PlanResult, error)
    Apply(plan *PlanResult) (*ApplyResult, error)
    Destroy(env EnvironmentID) error
}
```

### Requirements
- [x] Interface defined with all P0 methods
- [x] Provider Registry with registration/routing
- [x] AlicloudAdapter mock implementation
- [x] STS/AssumeRole support in credential model

### STS/AssumeRole Design
```go
type Credential struct {
    Provider    string
    AccessKey   string           // For development/testing
    SecretKey   string           // For development/testing
    STS         *STSCredential   // Preferred for production
}

type STSCredential struct {
    RoleARN         string
    SessionName     string
    DurationSeconds int
}
```

## 6. State Management (Task 7)

### Database Schema

```sql
CREATE TABLE infra_resources (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    env_id               UUID NOT NULL,
    resource_name        VARCHAR(100) NOT NULL,
    resource_type        VARCHAR(50) NOT NULL,
    provider_resource_id VARCHAR(500),
    config_hash          VARCHAR(64) NOT NULL,  -- SHA-256 of spec
    state_version        INT NOT NULL DEFAULT 1,
    config_json          JSONB,
    status               VARCHAR(20) NOT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    
    UNIQUE(env_id, resource_name)
);
```

### Idempotency Rules

1. **Create Before Check**: Before creating, query by `(env_id, resource_name)`
2. **Hash Comparison**: Calculate `spec_hash(spec)` and compare with stored `config_hash`
3. **Skip if Same**: If hash matches, skip update (no-op)
4. **Update if Different**: If hash differs, increment `state_version` and update
5. **Create if Missing**: If not found, create new record with `state_version = 1`

### spec_hash Calculation

Only hash fields that affect resource creation:

```go
func spec_hash(spec ResourceSpec) string {
    // Include: resource type, name, engine, version, instance_class, storage, etc.
    // Exclude: metadata, tags, timestamps, status fields
    h := sha256.New()
    h.Write([]byte(spec.Type))
    h.Write([]byte(spec.Name))
    h.Write([]byte(spec.Engine))
    // ... other creation-critical fields
    return hex.EncodeToString(h.Sum(nil))
}
```

### Acceptance Criteria
```go
// Test: Same spec -> skip update
spec1 := DatabaseSpec{Name: "mydb", Engine: "mysql", ...}
state.Ensure(spec1) // Creates record with version 1
state.Ensure(spec1) // Same hash -> skip, version still 1

// Test: Different spec -> update
spec2 := DatabaseSpec{Name: "mydb", Engine: "mysql", StorageGB: 100}
state.Ensure(spec2) // Different hash -> update, version 2
```

## 7. K8s Manifest Output (Task 9)

### Standard Output Format

Even in mock mode (Milestone A), the deploy plan must output valid K8s manifests:

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-service
  namespace: infracast-dev
spec:
  replicas: 3
  selector:
    matchLabels:
      app: api-service
  template:
    metadata:
      labels:
        app: api-service
    spec:
      containers:
      - name: app
        image: registry.cn-hangzhou.aliyuncs.com/myapp:latest
        ports:
        - containerPort: 8080
        env:
        - name: DB_HOST
          valueFrom:
            secretKeyRef:
              name: db-credentials
              key: host
---
apiVersion: v1
kind: Service
metadata:
  name: api-service
  namespace: infracast-dev
spec:
  selector:
    app: api-service
  ports:
  - port: 80
    targetPort: 8080
  type: ClusterIP
```

### Validation Requirements
- [x] Output must be valid YAML
- [x] Must pass `kubectl apply --dry-run=client` validation
- [x] Must include Deployment and Service resources

## 8. Success Gates (Milestone A Completion)

All of the following must pass:

- [x] **Configuration Chain**: `infracast.yaml` read/write and validation passes
- [x] **State Chain**: Idempotency table constraints and version update logic passes
- [x] **Infra Generation**: `infracfg.json` can be read back and matches expected schema
- [x] **CLI**: `init/run/deploy` commands are executable (deploy may be mock)
- [x] **Documentation**: Milestone A delivery notes (scope/limitations/next steps)

## 9. Out of Scope (Explicit)

The following are NOT included in Milestone A:

- ❌ Real Alicloud API calls (mock only)
- ❌ Actual database/RDS creation
- ❌ Actual OSS bucket creation  
- ❌ Real ACK deployment
- ❌ Native Pub/Sub provider
- ❌ Cron job support
- ❌ Web Portal UI
- ❌ Terraform integration

## 10. Interface Freeze List

At the end of Milestone A, the following interfaces will be frozen:

1. **`infracast.yaml` Schema**: All fields and validation rules
2. **`infracfg.json` Schema**: Alignment with Encore self-host config
3. **`CloudProviderInterface`**: Method signatures and return types
4. **State Idempotency Rules**: Hash calculation and version logic

Any changes to these interfaces after Milestone A will require version bump and migration path.

---

**Next**: Milestone B will implement real Alicloud SDK integration based on these frozen interfaces.
