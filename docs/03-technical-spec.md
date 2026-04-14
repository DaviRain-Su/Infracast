# Infracast — Technical Specification

> **Version** 1.1 · **Date** 2026-04-14 · **Status** Frozen · **Author** @CC (Tech Review)
> **Phase**: dev-lifecycle Phase 3 (承接 PRD v1.1 Frozen → Architecture v1.1 Frozen)
> **Input**: PRD v1.1 (Frozen), Architecture v1.1 (Frozen), Task 1 Code

---

## 0. Conventions

- All struct fields marked `required` MUST be non-zero/non-empty. Validation returns error otherwise.
- All string lengths are in UTF-8 bytes unless noted.
- All time values are `time.Time` in UTC.
- Error codes use the format `E{MODULE}{NUMBER}` (e.g. `ECFG001`).
- JSON field names use `snake_case`. Go struct fields use `PascalCase`.
- `ctx context.Context` is the first parameter of all async/IO methods (omitted from tables for brevity).

---

## 1. Config Parser (`internal/config`)

### 1.1 Data Structures

#### Config (root)

| Field | Go Type | YAML Key | Required | Default | Constraints | Notes |
|-------|---------|----------|----------|---------|-------------|-------|
| Provider | `string` | `provider` | Yes | — | Must be in `AllowedProviders` | P0: `{"alicloud"}` |
| Region | `string` | `region` | Yes | — | Must match `^[a-z]{2}-[a-z]+-\d+$` | e.g. `cn-hangzhou` |
| Environments | `map[string]Environment` | `environments` | No | `{}` | Keys must match `^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`, max 50 chars | — |
| Overrides | `Overrides` | `overrides` | No | zero | — | — |

#### Environment

| Field | Go Type | YAML Key | Required | Default | Constraints |
|-------|---------|----------|----------|---------|-------------|
| Provider | `string` | `provider` | No | inherits root | Same as Config.Provider |
| Region | `string` | `region` | No | inherits root | Same as Config.Region |

#### Overrides

| Field | Go Type | YAML Key | Required | Default |
|-------|---------|----------|----------|---------|
| Databases | `map[string]DatabaseOverride` | `databases` | No | `{}` |
| Compute | `map[string]ComputeOverride` | `compute` | No | `{}` |
| Cache | `map[string]CacheOverride` | `cache` | No | `{}` |
| ObjectStorage | `map[string]ObjectStorageOverride` | `object_storage` | No | `{}` |

#### DatabaseOverride

| Field | Go Type | YAML Key | Required | Default | Constraints |
|-------|---------|----------|----------|---------|-------------|
| InstanceClass | `string` | `instance_class` | No | provider default | Max 100 chars |
| StorageGB | `int` | `storage_gb` | No | 20 | 20 <= x <= 32768 |
| HighAvail | `*bool` | `high_avail` | No | `false` | — |
| Engine | `string` | `engine` | No | `"postgresql"` | `{"postgresql", "mysql"}` |
| Version | `string` | `version` | No | provider default | — |

#### ComputeOverride

| Field | Go Type | YAML Key | Required | Default | Constraints |
|-------|---------|----------|----------|---------|-------------|
| Replicas | `int` | `replicas` | No | 1 | 1 <= x <= 100 |
| CPU | `string` | `cpu` | No | `"500m"` | K8s format: `^\d+m?$` |
| Memory | `string` | `memory` | No | `"512Mi"` | K8s format: `^\d+(Mi\|Gi)$` |

#### CacheOverride

| Field | Go Type | YAML Key | Required | Default | Constraints |
|-------|---------|----------|----------|---------|-------------|
| MemoryMB | `int` | `memory_mb` | No | 256 | 64 <= x <= 65536 |
| EvictionPolicy | `string` | `eviction_policy` | No | `"allkeys-lru"` | Redis eviction policies |

#### ObjectStorageOverride

| Field | Go Type | YAML Key | Required | Default | Constraints |
|-------|---------|----------|----------|---------|-------------|
| ACL | `string` | `acl` | No | `"private"` | `{"private", "public-read"}` |

### 1.2 Functions

#### `Load(path string) (*Config, error)`

| Aspect | Detail |
|--------|--------|
| Input | `path`: file path. Empty string defaults to `"infracast.yaml"` |
| Output | Parsed Config or error |
| Errors | `ECFG001`: file not found. `ECFG002`: YAML parse error. |
| Postcondition | Config fields populated from YAML. No validation yet. |

#### `(*Config) Validate() error`

| Aspect | Detail |
|--------|--------|
| Input | Parsed Config |
| Output | `nil` if valid, error otherwise |
| Errors | `ECFG010`: provider empty. `ECFG011`: provider not in allowed list. `ECFG012`: region empty. `ECFG013`: region format invalid. `ECFG014`: environment name invalid. `ECFG015`: override value out of range. |
| Postcondition | Config is safe to pass to Mapper. |

#### `(*Config) ResolveEnv(envName string) (*ResolvedEnv, error)`

| Aspect | Detail |
|--------|--------|
| Input | Environment name |
| Output | Merged config (root defaults + env-specific overrides) |
| Errors | `ECFG020`: environment not found. |
| Logic | If env has empty provider/region, inherit from root. |

```go
type ResolvedEnv struct {
    Name     string
    Provider string
    Region   string
    Overrides Overrides  // merged: env-level wins over root-level
}
```

### 1.3 Constants

```go
var AllowedProviders = map[string]bool{
    "alicloud": true,
    // P2: "huaweicloud": true,
    // P2: "tencentcloud": true,
}

var RegionPattern = regexp.MustCompile(`^[a-z]{2}-[a-z]+-\d+$`)

var EnvNamePattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)

const MaxEnvNameLen = 50
const MaxOverrideKeyLen = 100
```

### 1.4 Boundary Conditions

| # | Condition | Expected Behavior |
|---|-----------|-------------------|
| BC-1 | Empty YAML file | `ECFG010` (provider empty) |
| BC-2 | YAML with only `provider: alicloud` | `ECFG012` (region empty) |
| BC-3 | Provider = `"aws"` | `ECFG011` (not in allowed list) |
| BC-4 | Region = `"cn hangzhou"` (space) | `ECFG013` (format invalid) |
| BC-5 | Env name = `"-staging"` (leading hyphen) | `ECFG014` |
| BC-6 | Env name = `""` (empty string as map key) | `ECFG014` |
| BC-7 | Env name = 51-char string | `ECFG014` |
| BC-8 | `storage_gb: 0` | `ECFG015` (below minimum 20) |
| BC-9 | `storage_gb: 99999` | `ECFG015` (above maximum 32768) |
| BC-10 | `replicas: 0` | `ECFG015` (below minimum 1) |
| BC-11 | Duplicate env name (YAML allows?) | Last value wins (YAML spec behavior) |
| BC-12 | Missing `infracast.yaml` file | `ECFG001` |

---

## 2. Service Mapper (`internal/mapper`)

### 2.1 Data Structures

#### BuildMeta

| Field | Go Type | JSON Key | Required | Constraints | Source |
|-------|---------|----------|----------|-------------|--------|
| AppName | `string` | `app_name` | Yes | 1-100 chars, `^[a-z][a-z0-9-]*$` | `infracast.yaml` or directory name |
| Services | `[]ServiceMeta` | `services` | Yes | len >= 1 | Go source scan |
| Databases | `[]DatabaseMeta` | `databases` | No | — | Go source scan |
| Caches | `[]CacheMeta` | `caches` | No | — | Go source scan |
| ObjectStores | `[]ObjectStoreMeta` | `object_stores` | No | — | Go source scan |
| PubSubTopics | `[]PubSubMeta` | `pubsub_topics` | No | — | Ignored in P0 |
| BuildCommit | `string` | `build_commit` | Yes | 40-char hex SHA | `git rev-parse HEAD` |
| BuildImage | `string` | `build_image` | Yes | Valid Docker image ref | `encore build` output |

#### ServiceMeta

| Field | Go Type | JSON Key | Required | Constraints |
|-------|---------|----------|----------|-------------|
| Name | `string` | `name` | Yes | 1-63 chars, DNS label format |
| EntryPkg | `string` | `entry_pkg` | Yes | Valid Go package path |
| Port | `int` | `port` | No | Default 8080. Range 1-65535. |

#### DatabaseMeta

| Field | Go Type | JSON Key | Required | Constraints |
|-------|---------|----------|----------|-------------|
| Name | `string` | `name` | Yes | 1-63 chars, `^[a-z][a-z0-9_]*$` |
| MigrationDir | `string` | `migration_dir` | No | Relative path from project root |

#### CacheMeta

| Field | Go Type | JSON Key | Required | Constraints |
|-------|---------|----------|----------|-------------|
| Name | `string` | `name` | Yes | 1-63 chars |
| ClusterName | `string` | `cluster_name` | No | Defaults to Name |

#### ObjectStoreMeta

| Field | Go Type | JSON Key | Required | Constraints |
|-------|---------|----------|----------|-------------|
| BucketName | `string` | `bucket_name` | Yes | 3-63 chars, S3 naming rules |

#### PubSubMeta (P0: parsed but not provisioned)

| Field | Go Type | JSON Key | Required | Constraints |
|-------|---------|----------|----------|-------------|
| TopicName | `string` | `topic_name` | Yes | 1-100 chars |
| Subscribers | `[]string` | `subscribers` | No | Service names |

### 2.2 ResourceSpec (Mapper Output)

Each resource from BuildMeta + Overrides is converted to a typed ResourceSpec.

```go
type MappedResource struct {
    Type     ResourceType  // database | cache | object_storage | compute
    Name     string        // unique within environment
    Spec     interface{}   // *providers.DatabaseSpec | *providers.CacheSpec | ...
    Priority int           // provisioning order (lower = first)
    DependsOn []string     // resource names this depends on
}

type ResourceType string
const (
    RTDatabase      ResourceType = "database"
    RTCache         ResourceType = "cache"
    RTObjectStorage ResourceType = "object_storage"
    RTCompute       ResourceType = "compute"
)
```

**Priority rules** (DAG ordering):
1. Database (priority 10) — must exist before compute (app needs connection string)
2. Cache (priority 10) — same level as database
3. ObjectStorage (priority 10) — same level
4. Compute (priority 20) — depends on all data resources being provisioned

### 2.3 Functions

#### `NewMapper(registry *providers.Registry) *Mapper`

#### `(*Mapper) Map(config *config.ResolvedEnv, meta *BuildMeta) ([]MappedResource, error)`

| Aspect | Detail |
|--------|--------|
| Input | Resolved environment config + build metadata |
| Output | Ordered list of MappedResources |
| Errors | `EMAP001`: no services found. `EMAP002`: unknown resource type. `EMAP003`: override references nonexistent resource. |
| Logic | 1. Iterate meta.Databases → create DatabaseSpec with defaults. 2. Apply overrides from config. 3. Same for Caches, ObjectStores. 4. Create ComputeSpec per service. 5. Build dependency DAG. 6. Return topologically sorted list. |
| Postcondition | All resources have valid specs. Dependencies are resolvable. |

#### `(*Mapper) ScanSources(projectDir string) (*BuildMeta, error)`

| Aspect | Detail |
|--------|--------|
| Input | Project root directory |
| Output | BuildMeta populated from Go source scan |
| Errors | `EMAP010`: not a Go project. `EMAP011`: no Encore services found. |
| Logic | 1. Find `go.mod`. 2. Walk `.go` files. 3. Match Encore API patterns: `sqldb.NewDatabase(name, ...)`, `cache.NewCluster(name, ...)`, `objects.NewBucket(name, ...)`. 4. Extract resource names. 5. Populate BuildMeta. |

**P0 scan heuristics** (simple AST match):

| Encore Declaration | Pattern | Extracted Field |
|-------------------|---------|----------------|
| `sqldb.NewDatabase("mydb", ...)` | `sqldb\.NewDatabase\("([^"]+)"` | DatabaseMeta.Name = "mydb" |
| `sqldb.Named("mydb")` | `sqldb\.Named\("([^"]+)"` | DatabaseMeta.Name = "mydb" |
| `cache.NewCluster("mycache", ...)` | `cache\.NewCluster\("([^"]+)"` | CacheMeta.Name = "mycache" |
| `objects.NewBucket("mybucket", ...)` | `objects\.NewBucket\("([^"]+)"` | ObjectStoreMeta.BucketName = "mybucket" |

**Limitation**: P0 uses regex, not full AST. Cannot detect conditional resource creation, generated code, or resources declared in non-standard ways. Phase 3 Technical Spec for P1 may upgrade to AST-based scanning.

### 2.4 Default Spec Values (Alicloud P0)

| Resource | Field | Default Value |
|----------|-------|---------------|
| Database | Engine | `"postgresql"` |
| Database | Version | `"15"` |
| Database | InstanceClass | `"rds.pg.s1.small"` |
| Database | StorageGB | 20 |
| Database | HighAvail | false |
| Cache | Engine | `"redis"` |
| Cache | Version | `"7.0"` |
| Cache | MemoryMB | 256 |
| Cache | EvictionPolicy | `"allkeys-lru"` |
| ObjectStorage | ACL | `"private"` |
| Compute | Replicas | 1 |
| Compute | CPU | `"500m"` |
| Compute | Memory | `"512Mi"` |
| Compute | Port | 8080 |

### 2.5 Boundary Conditions

| # | Condition | Expected Behavior |
|---|-----------|-------------------|
| BC-1 | No services detected | `EMAP001` |
| BC-2 | Database declared in code but not in overrides | Use defaults |
| BC-3 | Override references resource not in code | `EMAP003` (warning, not error — user may have removed code) |
| BC-4 | Two databases with same name | Deduplicate, last declaration wins |
| BC-5 | Resource name contains uppercase | Normalize to lowercase |
| BC-6 | BuildCommit is empty (no git) | Use `"unknown"` |
| BC-7 | PubSubTopics present in scan | Parse and store in BuildMeta, but skip provisioning in P0 (log warning) |
| BC-8 | Project has 0 `.go` files | `EMAP010` |
| BC-9 | Database name > 63 chars | Truncate to 63 + warn |
| BC-10 | Circular dependency in resources | Not possible in P0 (compute depends on data, no cross-data deps) |

---

## 3. Provisioner (`internal/provisioner`)

### 3.1 Data Structures

#### ProvisionInput

| Field | Go Type | Required | Notes |
|-------|---------|----------|-------|
| EnvID | `string` | Yes | Environment identifier |
| Resources | `[]MappedResource` | Yes | From Mapper, topologically sorted |
| Provider | `providers.CloudProviderInterface` | Yes | From Registry |
| DryRun | `bool` | No | If true, only Plan, no Apply |

#### ProvisionResult

| Field | Go Type | Notes |
|-------|---------|-------|
| Outputs | `[]ResourceOutput` | Per-resource provisioning results |
| Created | `int` | Count of newly created resources |
| Updated | `int` | Count of updated resources (spec_hash changed) |
| Skipped | `int` | Count of skipped resources (spec_hash same) |
| Failed | `int` | Count of failed resources |
| Errors | `[]ProvisionError` | All errors encountered |

#### ProvisionError

| Field | Go Type | Notes |
|-------|---------|-------|
| ResourceName | `string` | Which resource failed |
| ResourceType | `ResourceType` | database / cache / ... |
| Code | `string` | Error code |
| Message | `string` | Human-readable message |
| Retryable | `bool` | Safe to retry? |
| Cause | `error` | Underlying error |

### 3.2 spec_hash Computation

```go
// pkg/hash/spec.go

// SpecHash computes SHA-256 of the resource spec's hashable fields.
// Excludes: name (part of unique key), resource IDs, timestamps, status, metadata.
func SpecHash(resourceType ResourceType, spec interface{}) (string, error)
```

**Algorithm**:
1. Extract hashable fields into a flat `map[string]interface{}`
2. Sort keys lexicographically
3. Marshal to canonical JSON (no whitespace, sorted keys)
4. SHA-256 hex encode

**Hashable fields per resource type**:

| Resource | Included Fields | Excluded Fields |
|----------|----------------|-----------------|
| Database | `engine`, `version`, `instance_class`, `storage_gb`, `high_avail` | `name`, resource IDs, timestamps |
| Cache | `engine`, `version`, `memory_mb`, `eviction_policy` | `name`, resource IDs, timestamps |
| ObjectStorage | `acl`, `cors_rules` (sorted by origin) | `name`, resource IDs, timestamps |
| Compute | `replicas`, `cpu`, `memory`, `port`, `env_vars` (sorted by key), `secret_refs` (sorted) | `service_name`, resource IDs, timestamps |

**Critical invariant**: `SpecHash(type, specA) == SpecHash(type, specB)` iff specA and specB would create identical cloud resources. Metadata changes (e.g. adding a tag) MUST NOT change the hash.

### 3.3 Idempotency Protocol

```
For each resource in topological order:
  1. current_hash = SpecHash(resource.Type, resource.Spec)
  2. record = StateStore.GetResource(envID, resource.Name)
  3. IF record == nil:
       → CREATE: call Provider.ProvisionXxx(spec)
       → StateStore.UpsertResource(new record, state_version=1)
     ELIF record.SpecHash == current_hash AND record.Status == "provisioned":
       → SKIP (noop)
     ELIF record.SpecHash != current_hash:
       → UPDATE: call Provider.ProvisionXxx(spec)  // idempotent create-or-update
       → StateStore.UpsertResource(record, state_version++)
     ELIF record.Status == "failed":
       → RETRY: call Provider.ProvisionXxx(spec)
       → StateStore.UpsertResource(record, state_version++)
```

### 3.4 Functions

#### `NewProvisioner(store state.Store, creds credentials.CredentialManager) *Provisioner`

#### `(*Provisioner) Provision(ctx context.Context, input ProvisionInput) (*ProvisionResult, error)`

| Aspect | Detail |
|--------|--------|
| Input | ProvisionInput |
| Output | ProvisionResult (partial success possible) |
| Errors | `EPROV001`: credential fetch failed. `EPROV002`: SDK call failed (retryable). `EPROV003`: SDK call failed (non-retryable). `EPROV004`: state store write failed. `EPROV005`: optimistic lock conflict (state_version mismatch). |
| Retry | SDK call errors (EPROV002): retry up to 3x, exponential backoff (1s, 2s, 4s). |
| Concurrency | Resources at same priority level MAY be provisioned in parallel (via `errgroup`). Resources at different priority levels are sequential. |

### 3.5 Error Classification

| Error Code | Retryable | Side Effect | Example |
|-----------|-----------|-------------|---------|
| `EPROV001` | No | None | Invalid credentials |
| `EPROV002` | Yes | Possible partial creation | Network timeout, rate limit, server 500 |
| `EPROV003` | No | None | Invalid parameter, quota exceeded, permission denied |
| `EPROV004` | Yes | Resource created but state not saved | SQLite write failed |
| `EPROV005` | Yes | None | Concurrent deploy detected |

### 3.6 Boundary Conditions

| # | Condition | Expected Behavior |
|---|-----------|-------------------|
| BC-1 | Empty resource list | Return success with all counts = 0 |
| BC-2 | All resources already provisioned with same hash | All skipped, no SDK calls |
| BC-3 | One resource fails, others succeed | Partial result: successful ones saved, failed one in Errors |
| BC-4 | STS token expires mid-provisioning | EPROV001, abort remaining. User re-runs. |
| BC-5 | Cloud resource exists but not in state store | CREATE call is idempotent (SDK returns existing resource) |
| BC-6 | State store has record but cloud resource was manually deleted | UPDATE call creates resource again |
| BC-7 | Two concurrent `infracast deploy` for same env | Second one gets EPROV005 on state_version conflict, retries |
| BC-8 | Provision succeeds but state write fails | Resource leaked. Next run: state says "not exist" → tries CREATE → SDK returns "already exists" → treat as success, save state |
| BC-9 | DryRun = true | Only compute hashes and compare, return plan without SDK calls |
| BC-10 | spec_hash of empty spec | Valid hash (hash of `{}`) |

---

## 4. State Store (`internal/state`)

### 4.1 Data Structures

#### ResourceRecord

| Field | Go Type | DB Column | Type | Constraints |
|-------|---------|-----------|------|-------------|
| ID | `string` | `id` | `TEXT PRIMARY KEY` | UUID v4 |
| EnvID | `string` | `env_id` | `TEXT NOT NULL` | — |
| ResourceType | `string` | `resource_type` | `TEXT NOT NULL` | `{"database", "cache", "object_storage", "compute"}` |
| ResourceName | `string` | `resource_name` | `TEXT NOT NULL` | — |
| ProviderResourceID | `string` | `provider_resource_id` | `TEXT` | Cloud vendor's ID |
| SpecHash | `string` | `spec_hash` | `TEXT NOT NULL` | 64-char hex (SHA-256) |
| StateVersion | `int` | `state_version` | `INTEGER NOT NULL DEFAULT 1` | Monotonically increasing |
| ConfigJSON | `[]byte` | `config_json` | `TEXT` | JSON snapshot of resource config |
| Status | `string` | `status` | `TEXT NOT NULL` | `{"pending", "provisioning", "provisioned", "updating", "failed", "destroyed"}` |
| ErrorMsg | `string` | `error_msg` | `TEXT` | Last error message if failed |
| CreatedAt | `time.Time` | `created_at` | `DATETIME NOT NULL` | — |
| UpdatedAt | `time.Time` | `updated_at` | `DATETIME NOT NULL` | — |

#### SQLite Schema

```sql
CREATE TABLE IF NOT EXISTS infra_resources (
    id                   TEXT PRIMARY KEY,
    env_id               TEXT NOT NULL,
    resource_type        TEXT NOT NULL CHECK(resource_type IN ('database','cache','object_storage','compute')),
    resource_name        TEXT NOT NULL,
    provider_resource_id TEXT,
    spec_hash            TEXT NOT NULL,
    state_version        INTEGER NOT NULL DEFAULT 1,
    config_json          TEXT,
    status               TEXT NOT NULL DEFAULT 'pending'
                         CHECK(status IN ('pending','provisioning','provisioned','updating','failed','destroyed')),
    error_msg            TEXT,
    created_at           DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at           DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_env_resource
ON infra_resources(env_id, resource_name);
```

### 4.2 Store Interface

```go
type Store interface {
    GetResource(ctx context.Context, envID, resourceName string) (*ResourceRecord, error)
    UpsertResource(ctx context.Context, record *ResourceRecord) error
    ListResources(ctx context.Context, envID string) ([]ResourceRecord, error)
    DeleteResource(ctx context.Context, envID, resourceName string) error
    ListEnvironments(ctx context.Context) ([]string, error)
}
```

### 4.3 UpsertResource — Optimistic Locking Protocol

```sql
-- INSERT new resource
INSERT INTO infra_resources (id, env_id, resource_type, resource_name, provider_resource_id,
    spec_hash, state_version, config_json, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?, datetime('now'), datetime('now'))
ON CONFLICT(env_id, resource_name) DO UPDATE SET
    provider_resource_id = excluded.provider_resource_id,
    spec_hash = excluded.spec_hash,
    state_version = infra_resources.state_version + 1,
    config_json = excluded.config_json,
    status = excluded.status,
    error_msg = excluded.error_msg,
    updated_at = datetime('now')
WHERE infra_resources.state_version = excluded.state_version - 1;
```

**Optimistic lock check**: If `WHERE` clause matches 0 rows on the UPDATE path, another process has modified this record. Return `ESTATE001` (version conflict).

### 4.4 Functions

| Function | Errors |
|----------|--------|
| `GetResource(ctx, envID, name)` | `ESTATE002`: not found (returns nil, nil — not an error) |
| `UpsertResource(ctx, record)` | `ESTATE001`: version conflict. `ESTATE003`: DB write error. |
| `ListResources(ctx, envID)` | `ESTATE003`: DB read error. |
| `DeleteResource(ctx, envID, name)` | `ESTATE003`: DB write error. |
| `ListEnvironments(ctx)` | `ESTATE003`: DB read error. |

### 4.5 Concurrency Test Specification

```go
// Test: concurrent UpsertResource on same (env_id, resource_name)
func TestUpsertResource_ConcurrentConflict(t *testing.T) {
    store := NewSQLiteStore(":memory:")
    
    // Create initial record
    rec := &ResourceRecord{
        ID: uuid.New(), EnvID: "env1", ResourceName: "mydb",
        ResourceType: "database", SpecHash: "aaa", StateVersion: 1,
        Status: "provisioned",
    }
    err := store.UpsertResource(ctx, rec)
    require.NoError(t, err)
    
    // Two goroutines try to update concurrently
    var wg sync.WaitGroup
    var err1, err2 error
    
    wg.Add(2)
    go func() {
        defer wg.Done()
        rec1 := *rec
        rec1.SpecHash = "bbb"
        rec1.StateVersion = 1  // expects current version = 1
        err1 = store.UpsertResource(ctx, &rec1)
    }()
    go func() {
        defer wg.Done()
        rec2 := *rec
        rec2.SpecHash = "ccc"
        rec2.StateVersion = 1  // expects current version = 1
        err2 = store.UpsertResource(ctx, &rec2)
    }()
    wg.Wait()
    
    // Exactly one should succeed, one should get ESTATE001
    successCount := 0
    if err1 == nil { successCount++ }
    if err2 == nil { successCount++ }
    assert.Equal(t, 1, successCount, "exactly one concurrent update should succeed")
}
```

### 4.6 Boundary Conditions

| # | Condition | Expected Behavior |
|---|-----------|-------------------|
| BC-1 | GetResource for nonexistent key | Return `(nil, nil)` — not an error |
| BC-2 | UpsertResource with state_version mismatch | `ESTATE001` |
| BC-3 | ListResources for env with 0 resources | Return empty slice, not nil |
| BC-4 | DeleteResource for nonexistent key | No error (idempotent) |
| BC-5 | SQLite file does not exist | Auto-create on first Open() |
| BC-6 | SQLite file is locked by another process | `ESTATE003` with "database is locked" |
| BC-7 | resource_type not in CHECK constraint | `ESTATE003` with constraint violation |
| BC-8 | Two records with same (env_id, resource_name) | Impossible due to UNIQUE index |

---

## 5. Config Generator (`internal/infragen`)

### 5.1 Data Structures

**All field names MUST match PRD v1.1 §5.4 and Encore `infracfg.rs` schema. Map keys = Encore resource names.**

#### InfraCfg (root)

| Field | Go Type | JSON Key | Notes |
|-------|---------|----------|-------|
| SQLServers | `map[string]SQLServer` | `sql_servers` | Key = database name from Encore code |
| Redis | `map[string]RedisServer` | `redis` | Key = cache cluster name |
| ObjectStorage | `map[string]ObjectStore` | `object_storage` | Key = bucket name |

#### SQLServer

| Field | Go Type | JSON Key | Required | Source |
|-------|---------|----------|----------|--------|
| Host | `string` | `host` | Yes | RDS endpoint |
| Port | `int` | `port` | No | Default: 5432 (PG) / 3306 (MySQL) |
| DBName | `string` | `database` | Yes | Same as resource name |
| User | `string` | `user` | Yes | Generated: `infracast_{name}` |
| Password | `string` | `password` | Yes | From K8s Secret ref |
| TLS | `*TLSConfig` | `tls` | No | P0: nil (VPC internal) |

#### RedisServer

| Field | Go Type | JSON Key | Required | Source |
|-------|---------|----------|----------|--------|
| Host | `string` | `host` | Yes | Redis/Tair endpoint |
| Port | `int` | `port` | No | Default: 6379 |
| Auth | `string` | `auth` | No | From K8s Secret ref |
| KeyPrefix | `string` | `key_prefix` | No | `{app_name}:{cache_name}:` |
| TLS | `*TLSConfig` | `tls` | No | P0: nil |

#### ObjectStore

| Field | Go Type | JSON Key | Required | Source |
|-------|---------|----------|----------|--------|
| Provider | `string` | `provider` | Yes | `"s3-compatible"` (OSS is S3-compatible) |
| Endpoint | `string` | `endpoint` | Yes | OSS endpoint |
| Bucket | `string` | `bucket` | Yes | Bucket name |
| Region | `string` | `region` | Yes | From config |
| AccessKey | `string` | `access_key` | Yes | From K8s Secret ref |
| SecretKey | `string` | `secret_key` | Yes | From K8s Secret ref |

### 5.2 Functions

#### `NewGenerator() *Generator`

#### `(*Generator) Generate(outputs []providers.ResourceOutput, meta *mapper.BuildMeta) (*InfraCfg, error)`

| Aspect | Detail |
|--------|--------|
| Input | Resource provisioning outputs + build metadata |
| Output | InfraCfg ready to serialize |
| Errors | `EIGEN001`: unsupported resource type in output. `EIGEN002`: missing required field in output (e.g. no endpoint). |
| Logic | 1. Iterate outputs. 2. Match by type. 3. Map cloud-specific fields to Encore schema. 4. Set map key = resource name. |

#### `(*Generator) Write(cfg *InfraCfg, path string) error`

| Aspect | Detail |
|--------|--------|
| Input | InfraCfg + file path |
| Output | Writes JSON with 2-space indent |
| Errors | `EIGEN003`: file write error. |

#### `(*Generator) Merge(base *InfraCfg, override *InfraCfg) *InfraCfg`

| Aspect | Detail |
|--------|--------|
| Input | Base (generated) + override (user's `infra/infracfg.json`) |
| Output | Merged config. Per-resource-name: override fields win if non-zero. |
| Logic | Deep merge at resource level. If override has `sql_servers.mydb.port = 3307`, only port is overridden, other fields kept from base. |

### 5.3 Boundary Conditions

| # | Condition | Expected Behavior |
|---|-----------|-------------------|
| BC-1 | No resources provisioned | Return empty InfraCfg (`{}`) |
| BC-2 | Database output missing endpoint | `EIGEN002` |
| BC-3 | User override file doesn't exist | No merge, use generated as-is |
| BC-4 | User override has resource not in generated | Added to output (user may have external resources) |
| BC-5 | Password contains special characters | JSON-escaped correctly |
| BC-6 | Resource name contains dots or slashes | Map key preserved as-is (Encore uses it) |

---

## 6. Credential Manager (`internal/credentials`)

### 6.1 Data Structures

#### Credentials

| Field | Go Type | Notes |
|-------|---------|-------|
| AccessKeyID | `string` | — |
| AccessKeySecret | `string` | — |
| SecurityToken | `string` | Empty if direct mode |
| Expiration | `time.Time` | Zero if direct mode |

#### CredentialConfig

| Field | Go Type | Source | Notes |
|-------|---------|--------|-------|
| Mode | `string` | `--credentials-mode` flag or env | `"sts"` (default) or `"direct"` |
| AccessKeyID | `string` | env `ALICLOUD_ACCESS_KEY_ID` | — |
| AccessKeySecret | `string` | env `ALICLOUD_ACCESS_KEY_SECRET` | — |
| RoleArn | `string` | env `ALICLOUD_ROLE_ARN` | For STS AssumeRole |
| RoleSessionName | `string` | auto-generated | `"infracast-{timestamp}"` |
| DurationSeconds | `int` | default 3600 | 900-3600 |

### 6.2 Functions

#### `NewManager(cfg CredentialConfig) (CredentialManager, error)`

| Aspect | Detail |
|--------|--------|
| Errors | `ECRED001`: missing access key. `ECRED002`: missing secret key. `ECRED003`: STS mode but missing role ARN. |

#### `(*Manager) GetCredentials(ctx context.Context, provider string) (*Credentials, error)`

| Aspect | Detail |
|--------|--------|
| Logic | **Direct mode**: return AK/SK directly. **STS mode**: call STS AssumeRole API, cache result until `Expiration - 5min`. |
| Errors | `ECRED010`: STS call failed (retryable). `ECRED011`: STS call failed (permission denied, non-retryable). `ECRED012`: unsupported provider. |
| Cache | In-memory. If cached token expires in < 5 minutes, refresh. |

### 6.3 Encryption (at-rest)

```go
// internal/credentials/encrypt.go

// Encrypt encrypts plaintext using AES-256-GCM with the given key.
func Encrypt(plaintext []byte, key []byte) ([]byte, error)
// Returns: nonce (12 bytes) || ciphertext || tag (16 bytes)

// Decrypt decrypts ciphertext using AES-256-GCM with the given key.
func Decrypt(ciphertext []byte, key []byte) ([]byte, error)

// DeriveKey derives a 32-byte key from passphrase using PBKDF2.
// Salt: 16 bytes random, stored alongside ciphertext.
// Iterations: 600000 (OWASP 2024 recommendation).
func DeriveKey(passphrase string, salt []byte) []byte
```

### 6.4 Boundary Conditions

| # | Condition | Expected Behavior |
|---|-----------|-------------------|
| BC-1 | STS token expired during provisioning | EPROV001 from Provisioner. User re-runs deploy. |
| BC-2 | Role ARN has wrong format | ECRED003 at init time |
| BC-3 | Network timeout calling STS | ECRED010 (retryable, up to 3x) |
| BC-4 | AK/SK env vars not set | ECRED001 at init time |
| BC-5 | Duration < 900 or > 3600 | Clamp to valid range, log warning |
| BC-6 | Concurrent GetCredentials calls | Only one STS call; others wait for cached result (sync.Once per refresh) |
| BC-7 | Provider != "alicloud" in P0 | ECRED012 |

---

## 7. Deploy Pipeline (`internal/deploy`)

### 7.1 Pipeline State Machine

```
idle → building → mapping → provisioning → configuring → deploying → verifying → notifying → done
                                                                        │
                                                                   fail │
                                                                        ▼
                                                                   rolling_back → rolled_back
```

**Any step failure** → jump to `rolling_back` (if deployable state reached) or `done` with error (if pre-deploy).

### 7.2 Pipeline Interface

```go
type Pipeline struct {
    Config     *config.Config
    Registry   *providers.Registry
    Store      state.Store
    Creds      credentials.CredentialManager
    Generator  *infragen.Generator
    Mapper     *mapper.Mapper
    Logger     *slog.Logger
}

type Input struct {
    EnvName    string            // target environment
    ProjectDir string            // project root
    Verbose    bool
}

type Result struct {
    Status     PipelineStatus    // done | failed | rolled_back
    Steps      []StepResult      // per-step outcomes
    Resources  []ResourceResult
    InfraCfgPath string          // path to generated infracfg.json
    DeployedAt time.Time
    Duration   time.Duration
}

type StepResult struct {
    Step      PipelineStep
    Status    string           // success | failed | skipped
    Duration  time.Duration
    Error     *PipelineError
}

type PipelineStatus string
const (
    StatusDone       PipelineStatus = "done"
    StatusFailed     PipelineStatus = "failed"
    StatusRolledBack PipelineStatus = "rolled_back"
)
```

### 7.3 Step Specifications

#### Step 1: Build

| Aspect | Detail |
|--------|--------|
| Action | Run `encore build docker <image_tag>` in project dir |
| Input | ProjectDir, BuildCommit |
| Output | Docker image tag, BuildMeta |
| Timeout | 5 minutes |
| Errors | `EDEPLOY001`: encore build failed (non-retryable). `EDEPLOY002`: docker build failed. |
| Rollback | None needed. |

#### Step 2: Map

| Aspect | Detail |
|--------|--------|
| Action | Call `Mapper.Map(resolvedEnv, buildMeta)` |
| Input | Config, BuildMeta |
| Output | `[]MappedResource` |
| Timeout | 10 seconds |
| Errors | `EDEPLOY010`: mapping failed (see EMAP codes). |
| Rollback | None needed. |

#### Step 3: Provision

| Aspect | Detail |
|--------|--------|
| Action | Call `Provisioner.Provision(input)` |
| Input | MappedResources, Provider, EnvID |
| Output | `ProvisionResult` |
| Timeout | 10 minutes |
| Errors | `EDEPLOY020`: provision failed (see EPROV codes). Partial success possible. |
| Rollback | None — provisioned resources persist for next run. |
| Retry | Per-resource retry handled by Provisioner (3x). Pipeline does not retry this step. |

#### Step 4: Configure

| Aspect | Detail |
|--------|--------|
| Action | Generate `infracfg.json` + merge user override + write to `.infracast/infracfg.json` |
| Input | ResourceOutputs, BuildMeta |
| Output | File path to generated infracfg.json |
| Timeout | 10 seconds |
| Errors | `EDEPLOY030`: config generation failed (see EIGEN codes). |
| Rollback | None — file is local. |

#### Step 5: Deploy

| Aspect | Detail |
|--------|--------|
| Action | 1. Push image to ACR. 2. Generate K8s Deployment + Service YAML. 3. Create K8s ConfigMap from infracfg.json. 4. Apply to ACK Serverless. |
| Input | Docker image, infracfg.json, ComputeSpecs |
| Output | K8s Deployment name, Service name |
| Timeout | 5 minutes |
| Errors | `EDEPLOY040`: ACR push failed (retryable). `EDEPLOY041`: K8s apply failed (retryable). `EDEPLOY042`: K8s apply failed (non-retryable — invalid manifest). |
| Retry | ACR push: 3x. K8s apply: 2x. |

**K8s YAML generation**:

```yaml
# Generated Deployment (per service)
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {app_name}-{service_name}
  namespace: {env_namespace}
  labels:
    app.kubernetes.io/name: {service_name}
    app.kubernetes.io/managed-by: infracast
    infracast.dev/env: {env_name}
    infracast.dev/commit: {build_commit}
spec:
  replicas: {compute.replicas}
  selector:
    matchLabels:
      app.kubernetes.io/name: {service_name}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: {service_name}
    spec:
      containers:
      - name: {service_name}
        image: {acr_registry}/{app_name}:{build_commit}
        ports:
        - containerPort: {compute.port}
        resources:
          requests:
            cpu: {compute.cpu}
            memory: {compute.memory}
          limits:
            cpu: {compute.cpu}
            memory: {compute.memory}
        volumeMounts:
        - name: infracfg
          mountPath: /etc/infracast
          readOnly: true
        env:
        - name: INFRACFG_PATH
          value: /etc/infracast/infracfg.json
        readinessProbe:
          httpGet:
            path: /healthz
            port: {compute.port}
          initialDelaySeconds: 5
          periodSeconds: 10
        livenessProbe:
          httpGet:
            path: /healthz
            port: {compute.port}
          initialDelaySeconds: 15
          periodSeconds: 20
      volumes:
      - name: infracfg
        configMap:
          name: {app_name}-{service_name}-infracfg
---
apiVersion: v1
kind: Service
metadata:
  name: {service_name}
  namespace: {env_namespace}
spec:
  selector:
    app.kubernetes.io/name: {service_name}
  ports:
  - port: 80
    targetPort: {compute.port}
  type: ClusterIP
```

#### Step 6: Verify

| Aspect | Detail |
|--------|--------|
| Action | Poll K8s Deployment status until all pods Ready |
| Input | Deployment name, namespace |
| Output | success or timeout |
| Timeout | **5 minutes** (frozen decision) |
| Poll interval | 10 seconds |
| Errors | `EDEPLOY050`: health check timeout. |
| On failure | Trigger rollback: `kubectl rollout undo deployment/{name}` |

#### Step 7: Notify

| Aspect | Detail |
|--------|--------|
| Action | Send deploy result to configured webhooks (Feishu/DingTalk) |
| Input | Result, Config.Notifications |
| Output | None (fire-and-forget) |
| Timeout | 10 seconds per webhook |
| Errors | `EDEPLOY060`: webhook failed (non-blocking, logged only). |
| Rollback | None. Notification failure never blocks deploy. |

### 7.4 Rollback Protocol

```
IF step 5 or 6 fails AND a K8s Deployment was created/updated:
    1. kubectl rollout undo deployment/{name} -n {namespace}
    2. Wait for rollback Deployment to be Ready (timeout: 3 min)
    3. Set pipeline status = "rolled_back"
    4. Log: "Rolled back to previous revision"

IF step 3 fails (provision):
    No rollback. Resources remain. Next deploy picks up from state.

IF step 1/2/4 fails:
    No rollback needed. Nothing was deployed.
```

### 7.5 Forward-Only Database Migration (不可变更条款)

**此条款为冻结决议（PRD v1.1 Decision 9），不可修改。**

数据库迁移执行遵循 **forward-only** 策略：

1. **迁移只前进，不回退**: Pipeline 执行 `migration_N.sql` 后，无论后续步骤成功与否，**不执行** `DROP TABLE`、`DROP COLUMN`、`ALTER TABLE ... DROP` 等破坏性 DDL 作为回滚手段。
2. **迁移失败时的行为**:
   - 数据库保留当前状态（可能是 `migration_N` 部分执行后的状态）
   - 应用回滚到上一个 K8s Deployment revision（该版本兼容 `migration_N-1` schema）
   - Pipeline 状态设为 `rolled_back`
   - 修复方式：开发者编写 `migration_N+1.sql` 修正问题，重新部署
3. **开发者契约**: Encore 应用的每个版本 **必须** 向后兼容上一个 schema 版本（即 app v2 能运行在 migration_001 和 migration_002 的 schema 上）
4. **禁止的操作**: Pipeline **永远不会**自动执行以下 SQL：
   - `DROP TABLE`
   - `DROP COLUMN`
   - `TRUNCATE`
   - `DELETE FROM` (非 migration 的数据删除)
   - 任何以 "rollback" 命名的迁移文件

| 场景 | 数据库状态 | 应用状态 | 用户操作 |
|------|-----------|---------|---------|
| 迁移成功 + 部署成功 | migration_N applied | v_new running | 无 |
| 迁移成功 + 部署失败(health check) | migration_N applied | v_old rolled back | 修复代码，push v_new_fix |
| 迁移失败(SQL error) | migration_N partial | v_old still running | 修复 migration_N+1，push |
| 迁移失败(连接超时) | migration_N not applied | v_old still running | 重新 deploy（幂等） |

### 7.6 Full Error Code Table

| Code | Step | Retryable | Side Effect | Description |
|------|------|-----------|-------------|-------------|
| `EDEPLOY001` | Build | No | None | `encore build` failed |
| `EDEPLOY002` | Build | Yes | None | Docker build failed |
| `EDEPLOY010` | Map | No | None | Service mapping failed |
| `EDEPLOY020` | Provision | Partial | Resources may be created | Cloud resource provisioning failed |
| `EDEPLOY030` | Configure | No | None | infracfg.json generation failed |
| `EDEPLOY040` | Deploy | Yes | None | ACR push failed |
| `EDEPLOY041` | Deploy | Yes | Possible partial apply | K8s apply failed (transient) |
| `EDEPLOY042` | Deploy | No | None | K8s apply failed (bad manifest) |
| `EDEPLOY050` | Verify | No | Deployment exists | Health check timeout → triggers rollback |
| `EDEPLOY060` | Notify | No | None | Webhook failed (non-blocking) |

### 7.7 Boundary Conditions

| # | Condition | Expected Behavior |
|---|-----------|-------------------|
| BC-1 | First deploy (no previous Deployment exists) | Rollback has nothing to undo → status = "failed" |
| BC-2 | Deploy with 0 resources (only compute) | Skip provision step, go directly to configure |
| BC-3 | User cancels (Ctrl+C) during provision | Context cancelled. Partial resources saved to state. Resumable. |
| BC-4 | K8s namespace doesn't exist | Create namespace before applying |
| BC-5 | ACR registry not accessible | EDEPLOY040, retry 3x |
| BC-6 | Health check endpoint returns 503 | Keep polling until timeout |
| BC-7 | Multiple services in one app | Each service gets its own Deployment + Service |
| BC-8 | Notification config is empty | Skip step 7 entirely |
| BC-9 | Rollback itself fails | Log error, status = "failed" (not "rolled_back") |
| BC-10 | Deploy with `--verbose` | Log each step's input/output to stderr |

---

## 8. Alicloud Adapter (`providers/alicloud`)

### 8.1 SDK Dependencies

| Resource | Alicloud SDK Package | API Version |
|----------|---------------------|-------------|
| RDS | `github.com/alibabacloud-go/rds-20140815/v3` | 2014-08-15 |
| Redis/Tair | `github.com/alibabacloud-go/r-kvstore-20150101/v3` | 2015-01-01 |
| OSS | `github.com/aliyun/aliyun-oss-go-sdk/oss` | v2 |
| ACK | `github.com/alibabacloud-go/cs-20151215/v5` | 2015-12-15 |
| ACR | `github.com/alibabacloud-go/cr-20181201/v2` | 2018-12-01 |
| STS | `github.com/alibabacloud-go/sts-20150401/v2` | 2015-04-01 |

### 8.2 Resource Mapping (Alicloud Specifics)

| ResourceSpec Field | Alicloud API Parameter | Notes |
|-------------------|----------------------|-------|
| DatabaseSpec.Engine="postgresql" | `Engine="PostgreSQL"` | Case difference |
| DatabaseSpec.Version="15" | `EngineVersion="15.0"` | Alicloud requires minor version |
| DatabaseSpec.InstanceClass | `DBInstanceClass` | Direct passthrough |
| DatabaseSpec.StorageGB | `DBInstanceStorage` | In GB |
| DatabaseSpec.HighAvail=true | `Category="HighAvailability"` | false → `"Basic"` |
| CacheSpec.MemoryMB=256 | `Capacity=256` | In MB |
| ObjectStorageSpec.ACL="private" | `ACL=oss.ACLPrivate` | SDK enum |
| ComputeSpec.CPU="500m" | K8s resource (not Alicloud API) | Passed to K8s YAML |

### 8.3 Boundary Conditions

| # | Condition | Expected Behavior |
|---|-----------|-------------------|
| BC-1 | RDS instance name already exists | SDK returns existing instance. Treat as success. Save state. |
| BC-2 | Region doesn't have requested instance class | EPROV003 (non-retryable) with "instance class not available in region" |
| BC-3 | Quota exceeded (max RDS instances) | EPROV003 with "quota exceeded" |
| BC-4 | VPC/VSwitch not found | Auto-create default VPC/VSwitch for the env (P0 simplification) |
| BC-5 | OSS bucket name already taken globally | EPROV003 with "bucket name already taken". User must choose different name. |

---

## 9. Cross-Cutting Concerns

### 9.1 Logging

```go
// All modules use slog.Logger with structured fields
logger.Info("resource provisioned",
    "env_id", envID,
    "resource_name", name,
    "resource_type", rtype,
    "action", "create",
    "duration_ms", elapsed.Milliseconds(),
)

// NEVER log credentials
// NEVER log full spec_hash (log first 8 chars for traceability)
```

### 9.2 Timeouts (consolidated)

| Operation | Timeout | Source |
|-----------|---------|--------|
| `encore build` | 5 min | Step 1 |
| Service mapping | 10 sec | Step 2 |
| Single SDK call | 30 sec | Provisioner |
| Total provisioning | 10 min | Step 3 |
| Config generation | 10 sec | Step 4 |
| ACR push | 5 min | Step 5 |
| K8s apply | 2 min | Step 5 |
| Health check | 5 min | Step 6 |
| Webhook | 10 sec | Step 7 |
| STS AssumeRole | 10 sec | Credentials |

### 9.3 Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Config error (ECFG*) |
| 3 | Credential error (ECRED*) |
| 4 | Provision error (EPROV*) |
| 5 | Deploy error (EDEPLOY*) |
| 10 | Rolled back (deploy succeeded partially, app reverted) |

---

## Acceptance Criteria (Phase 3)

- [ ] Every interface/function has: parameters, return values, error codes, pre/postconditions
- [ ] Every data structure has: field name, type, constraints, default, required flag
- [ ] State machines for resource lifecycle and deployment lifecycle are defined
- [ ] spec_hash: included vs excluded fields are exhaustively listed per resource type
- [ ] At least 10 boundary conditions per module
- [ ] Error codes are unique and classified as retryable vs non-retryable
- [ ] Concurrency test specification for State Store
- [ ] K8s YAML template is complete and deployable
- [ ] All field names align with PRD v1.1 frozen decisions
- [ ] infracfg.json uses map semantics (not array) per Architecture ADR-5

---

*— End of Document —*

*Infracast Technical Specification v1.1 (Frozen) | Phase 3 of dev-lifecycle | Confidential*
