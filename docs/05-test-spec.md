# Infracast — Test Specification

> **Version** 1.1 · **Date** 2026-04-14 · **Status** Frozen · **Author** @CC (Tech Review)
> **Phase**: dev-lifecycle Phase 5 (承接 Task Breakdown v1.1 Frozen)
> **Input**: Technical Spec v1.1 (Frozen), Task Breakdown v1.1 (Frozen)

---

## 0. Test Rules

- **TDD**: Test skeleton MUST be written before implementation code
- **Three categories** per function: Happy Path, Boundary, Error/Attack
- **Table-driven tests**: Use Go subtests (`t.Run`) with test tables
- **Isolation**: Tests must not depend on external services (use mock/in-memory)
- **Naming**: `Test{Function}_{Scenario}` (e.g. `TestValidate_MissingProvider`)
- **Coverage target**: >= 80% line coverage per package (Milestone A)

---

## 1. Config Parser (`internal/config`) — TA06

### 1.1 TestLoad

| # | Category | Scenario | Input | Expected |
|---|----------|----------|-------|----------|
| 1 | Happy | Valid YAML file | `provider: alicloud\nregion: cn-hangzhou` | Config{Provider:"alicloud", Region:"cn-hangzhou"}, nil |
| 2 | Happy | Full config with envs + overrides | Complete YAML | All fields populated |
| 3 | Error | File not found | path="nonexistent.yaml" | ECFG001 |
| 4 | Error | Invalid YAML syntax | `provider: [broken` | ECFG002 |
| 5 | Error | Empty file | 0 bytes | Config with empty fields (validation catches later) |
| 6 | Boundary | Very large file (1MB) | Repeated env entries | Parses successfully |

### 1.2 TestValidate

| # | Category | Scenario | Input | Expected |
|---|----------|----------|-------|----------|
| 1 | Happy | Valid minimal config | provider=alicloud, region=cn-hangzhou | nil |
| 2 | Happy | Valid with environments | + staging env | nil |
| 3 | Error | Missing provider | provider="" | ECFG010 |
| 4 | Error | Unsupported provider | provider="aws" | ECFG011 |
| 5 | Error | Missing region | region="" | ECFG012 |
| 6 | Boundary | Region with invalid format | region="cn hangzhou" (space) | ECFG013 |
| 7 | Boundary | Region valid format | region="cn-hangzhou-1" | nil |
| 8 | Boundary | Env name leading hyphen | env="-staging" | ECFG014 |
| 9 | Boundary | Env name 50 chars | env="a{49 chars}" | nil |
| 10 | Boundary | Env name 51 chars | env="a{50 chars}" | ECFG014 |
| 11 | Boundary | Env name empty string | env="" | ECFG014 |
| 12 | Boundary | storage_gb = 19 | override | ECFG015 |
| 13 | Boundary | storage_gb = 20 | override | nil |
| 14 | Boundary | storage_gb = 32768 | override | nil |
| 15 | Boundary | storage_gb = 32769 | override | ECFG015 |
| 16 | Boundary | replicas = 0 | override | ECFG015 |
| 17 | Boundary | replicas = 1 | override | nil |
| 18 | Boundary | replicas = 100 | override | nil |
| 19 | Boundary | replicas = 101 | override | ECFG015 |

### 1.3 TestResolveEnv

| # | Category | Scenario | Input | Expected |
|---|----------|----------|-------|----------|
| 1 | Happy | Env exists with overrides | envName="staging" | Merged config |
| 2 | Happy | Env inherits root provider/region | env has empty provider | Root values used |
| 3 | Error | Env not found | envName="nonexistent" | ECFG020 |
| 4 | Boundary | Env overrides root provider | env.provider="alicloud" (same) | Uses env value |

---

## 2. Service Mapper (`internal/mapper`) — TA04

### 2.1 TestScanSources

| # | Category | Scenario | Input | Expected |
|---|----------|----------|-------|----------|
| 1 | Happy | Project with 1 DB + 1 cache | Go files with `sqldb.NewDatabase("mydb")`, `cache.NewCluster("mycache")` | BuildMeta with 1 DB, 1 cache |
| 2 | Happy | Project with OSS bucket | `objects.NewBucket("mybucket")` | BuildMeta with 1 object store |
| 3 | Happy | Multiple services | 3 service directories | 3 ServiceMeta entries |
| 4 | Error | Not a Go project | No go.mod | EMAP010 |
| 5 | Error | No Encore services | go.mod exists but no services | EMAP011 |
| 6 | Boundary | Database name with underscore | `sqldb.NewDatabase("my_db")` | Parsed correctly |
| 7 | Boundary | PubSub topic present | `pubsub.NewTopic("events")` | Parsed into PubSubMeta, logged as P0 skip |
| 8 | Boundary | Duplicate DB names | Two `sqldb.NewDatabase("mydb")` | Deduplicated, last wins |
| 9 | Boundary | Database name > 63 chars | Long name | Truncated to 63 + warning |
| 10 | Boundary | No git (BuildCommit) | No .git directory | BuildCommit = "unknown" |

### 2.2 TestMap

| # | Category | Scenario | Input | Expected |
|---|----------|----------|-------|----------|
| 1 | Happy | 1 DB + 1 service | Standard BuildMeta + config | 2 MappedResources (DB priority 10, compute priority 20) |
| 2 | Happy | With overrides | config overrides storage_gb=100 | DatabaseSpec.StorageGB=100 |
| 3 | Happy | Defaults applied | No overrides | Tech Spec §2.4 defaults |
| 4 | Error | No services in meta | services=[] | EMAP001 |
| 5 | Boundary | Override references unknown resource | override for "unknowndb" | EMAP003 warning |
| 6 | Boundary | Resource name uppercase | "MyDB" | Normalized to "mydb" |
| 7 | Boundary | Topological order | DB + Cache + Compute | [DB(10), Cache(10), Compute(20)] |

---

## 3. spec_hash (`pkg/hash`) — TA03

### 3.1 TestSpecHash

| # | Category | Scenario | Input | Expected |
|---|----------|----------|-------|----------|
| 1 | Happy | Database spec | engine=pg, version=15, class=small, storage=20, ha=false | 64-char hex string |
| 2 | Happy | Same spec twice | Identical inputs | Same hash |
| 3 | Happy | Different spec | storage=20 vs storage=100 | Different hash |
| 4 | Boundary | Metadata change only | Same spec, different name | Same hash (name excluded) |
| 5 | Boundary | Empty spec | All zero values | Valid hash (hash of `{}`) |
| 6 | Boundary | Cache spec | engine=redis, version=7.0, memory=256, eviction=allkeys-lru | Correct hash |
| 7 | Boundary | ObjectStorage spec | acl=private, no CORS | Correct hash |
| 8 | Boundary | ObjectStorage with CORS | acl=private, 2 CORS rules | Different hash from no-CORS |
| 9 | Boundary | Compute with env_vars | env_vars={"A":"1","B":"2"} | Sorted keys → deterministic |
| 10 | Boundary | Compute env_vars order | {"B":"2","A":"1"} vs {"A":"1","B":"2"} | Same hash (sorted) |
| 11 | Attack | Unknown resource type | type="unknown" | Error returned |
| 12 | Boundary | Compute with secret_refs | ["s1","s2"] vs ["s2","s1"] | Same hash (sorted) |

---

## 4. State Store (`internal/state`) — TA02

### 4.1 TestGetResource

| # | Category | Scenario | Input | Expected |
|---|----------|----------|-------|----------|
| 1 | Happy | Existing resource | env1/mydb (after insert) | ResourceRecord |
| 2 | Boundary | Nonexistent resource | env1/unknown | nil, nil (not error) |
| 3 | Boundary | Different env, same name | env2/mydb (only env1 has it) | nil, nil |

### 4.2 TestUpsertResource

| # | Category | Scenario | Input | Expected |
|---|----------|----------|-------|----------|
| 1 | Happy | Insert new resource | New record, version=1 | Saved, version=1 |
| 2 | Happy | Update existing | Changed spec_hash, correct version | Saved, version=2 |
| 3 | Error | Version conflict | Two updates with same base version | One succeeds, one ESTATE001 |
| 4 | Boundary | Same env+name insert | Duplicate unique key | Handled by ON CONFLICT |
| 5 | Error | Invalid resource_type | type="invalid" | ESTATE003 (CHECK constraint) |
| 6 | Attack | SQL injection in resource_name | name=`'; DROP TABLE--` | Parameterized query, no injection |

### 4.3 TestUpsertResource_ConcurrentConflict

```go
func TestUpsertResource_ConcurrentConflict(t *testing.T) {
    store := NewSQLiteStore(":memory:")
    ctx := context.Background()
    
    // Insert initial record
    rec := &ResourceRecord{
        ID: uuid.New().String(), EnvID: "env1", ResourceName: "mydb",
        ResourceType: "database", SpecHash: "aaa", StateVersion: 1,
        Status: "provisioned",
    }
    require.NoError(t, store.UpsertResource(ctx, rec))
    
    // Two goroutines update concurrently
    var wg sync.WaitGroup
    results := make([]error, 2)
    
    for i := 0; i < 2; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            update := *rec
            update.SpecHash = fmt.Sprintf("hash_%d", idx)
            update.StateVersion = 1 // both expect version 1
            results[idx] = store.UpsertResource(ctx, &update)
        }(i)
    }
    wg.Wait()
    
    // Exactly one should succeed
    successes := 0
    for _, err := range results {
        if err == nil {
            successes++
        }
    }
    assert.Equal(t, 1, successes, "exactly one concurrent update should succeed")
    
    // Verify final version = 2
    final, _ := store.GetResource(ctx, "env1", "mydb")
    assert.Equal(t, 2, final.StateVersion)
}
```

### 4.4 TestListResources

| # | Category | Scenario | Input | Expected |
|---|----------|----------|-------|----------|
| 1 | Happy | Env with 3 resources | env1 with db, cache, oss | 3 records |
| 2 | Boundary | Env with 0 resources | env2 (empty) | Empty slice, not nil |
| 3 | Boundary | Multiple envs | env1 + env2 | Only env-specific results |

### 4.5 TestDeleteResource

| # | Category | Scenario | Input | Expected |
|---|----------|----------|-------|----------|
| 1 | Happy | Delete existing | env1/mydb | Deleted, GetResource returns nil |
| 2 | Boundary | Delete nonexistent | env1/unknown | No error (idempotent) |

---

## 5. Config Generator (`internal/infragen`) — TA05

### 5.1 TestGenerate

| # | Category | Scenario | Input | Expected |
|---|----------|----------|-------|----------|
| 1 | Happy | DB + Cache + OSS outputs | 3 ResourceOutputs | InfraCfg with sql_servers, redis, object_storage maps |
| 2 | Happy | DB only | 1 DatabaseOutput | InfraCfg with sql_servers only |
| 3 | Boundary | No resources | Empty outputs | InfraCfg = `{}` |
| 4 | Error | Missing endpoint in DB output | endpoint="" | EIGEN002 |
| 5 | Boundary | Map key correctness | DB name="mydb" | `sql_servers["mydb"]` |
| 6 | Boundary | Password with special chars | password=`p@ss"w0rd` | JSON-escaped correctly |

### 5.2 TestMerge

| # | Category | Scenario | Input | Expected |
|---|----------|----------|-------|----------|
| 1 | Happy | Override port | base.port=5432, override.port=3307 | Merged port=3307 |
| 2 | Boundary | Override adds new resource | base has mydb, override has newdb | Both in output |
| 3 | Boundary | No override file | override=nil | Base unchanged |
| 4 | Boundary | Override with empty fields | override.host="" | Base host preserved (empty = no override) |

### 5.3 TestWrite

| # | Category | Scenario | Input | Expected |
|---|----------|----------|-------|----------|
| 1 | Happy | Write to temp file | Valid InfraCfg | File exists, valid JSON, 2-space indent |
| 2 | Error | Invalid path | path="/nonexistent/dir/file" | EIGEN003 |
| 3 | Boundary | Empty InfraCfg | {} | File contains `{}` |

---

## 6. Credential Manager (`internal/credentials`) — TA07

### 6.1 TestNewManager

| # | Category | Scenario | Input | Expected |
|---|----------|----------|-------|----------|
| 1 | Happy | Direct mode with AK/SK | mode=direct, AK+SK set | Manager created |
| 2 | Happy | STS mode with role ARN | mode=sts, AK+SK+RoleArn set | Manager created |
| 3 | Error | Missing access key | AK="" | ECRED001 |
| 4 | Error | Missing secret key | SK="" | ECRED002 |
| 5 | Error | STS mode without role ARN | mode=sts, RoleArn="" | ECRED003 |

### 6.2 TestGetCredentials

| # | Category | Scenario | Input | Expected |
|---|----------|----------|-------|----------|
| 1 | Happy | Direct mode | provider=alicloud | Credentials with AK/SK, empty SecurityToken |
| 2 | Happy | STS mode (mocked) | provider=alicloud | Credentials with SecurityToken, valid Expiration |
| 3 | Boundary | STS token cache hit | Two calls within TTL | Only 1 STS API call (cached) |
| 4 | Boundary | STS token near expiry | token expires in 3 min | Refresh triggered |
| 5 | Error | Unsupported provider | provider="gcp" | ECRED012 |
| 6 | Error | STS call fails | Mock STS returns error | ECRED010 (retryable) |
| 7 | Boundary | Concurrent GetCredentials | 5 goroutines | Only 1 STS call (sync.Once) |
| 8 | Boundary | Duration clamping | duration=100 | Clamped to 900, warning logged |

### 6.3 TestEncryptDecrypt

| # | Category | Scenario | Input | Expected |
|---|----------|----------|-------|----------|
| 1 | Happy | Round-trip | Encrypt then Decrypt | Original plaintext recovered |
| 2 | Boundary | Empty plaintext | plaintext=[] | Encrypts/decrypts successfully |
| 3 | Boundary | Large plaintext | 1MB data | Encrypts/decrypts successfully |
| 4 | Attack | Wrong key | Encrypt with key1, Decrypt with key2 | Error (authentication failed) |
| 5 | Attack | Tampered ciphertext | Flip 1 bit in ciphertext | Error (authentication failed) |
| 6 | Attack | Truncated ciphertext | Remove last byte | Error |

### 6.4 TestDeriveKey

| # | Category | Scenario | Input | Expected |
|---|----------|----------|-------|----------|
| 1 | Happy | Deterministic | Same passphrase + salt | Same 32-byte key |
| 2 | Boundary | Different salt | Same passphrase, different salt | Different key |
| 3 | Boundary | Empty passphrase | passphrase="" | Valid 32-byte key (not recommended but not error) |

---

## 7. Provisioner (`internal/provisioner`) — TA08

### 7.1 TestProvision

| # | Category | Scenario | Input | Expected |
|---|----------|----------|-------|----------|
| 1 | Happy | Create 2 resources | New DB + Cache, mock provider | Created=2, Skipped=0, Updated=0 |
| 2 | Happy | All skipped (idempotent) | Same specs, already provisioned | Created=0, Skipped=2, Updated=0 |
| 3 | Happy | Update 1 resource | DB spec changed (storage_gb) | Created=0, Skipped=1, Updated=1 |
| 4 | Happy | Retry failed resource | Resource in "failed" state | Retried, Created=0, Updated=1 |
| 5 | Boundary | Empty resource list | resources=[] | All counts = 0, no SDK calls |
| 6 | Error | SDK call fails (retryable) | Mock returns error 2x, success 3rd | Created=1 after retries |
| 7 | Error | SDK call fails (non-retryable) | Mock returns permission denied | Failed=1, EPROV003 |
| 8 | Error | Partial failure | 2 resources, 1st succeeds, 2nd fails | Created=1, Failed=1, partial result |
| 9 | Boundary | DryRun mode | DryRun=true | No SDK calls, plan returned |
| 10 | Error | State write fails | Mock store returns error | EPROV004 (retryable) |
| 11 | Error | Optimistic lock conflict | Concurrent state update | EPROV005 |

### 7.2 TestProvision_TopologicalOrder

| # | Category | Scenario | Input | Expected |
|---|----------|----------|-------|----------|
| 1 | Happy | DB before Compute | DB(priority=10) + Compute(priority=20) | DB provisioned first |
| 2 | Boundary | Same priority parallel | DB(10) + Cache(10) | Both provisioned (order doesn't matter) |

### 7.3 TestIdempotencyProtocol

```go
func TestIdempotencyProtocol(t *testing.T) {
    store := state.NewSQLiteStore(":memory:")
    mockProvider := mock.NewProvider()
    prov := NewProvisioner(store, mockCreds)
    
    dbSpec := providers.DatabaseSpec{
        Engine: "postgresql", Version: "15",
        InstanceClass: "small", StorageGB: 20, HighAvail: false,
    }
    
    input := ProvisionInput{
        EnvID: "env1",
        Resources: []mapper.MappedResource{{
            Type: "database", Name: "mydb",
            Spec: &dbSpec, Priority: 10,
        }},
        Provider: mockProvider,
    }
    
    // Run 1: CREATE
    r1, err := prov.Provision(ctx, input)
    require.NoError(t, err)
    assert.Equal(t, 1, r1.Created)
    assert.Equal(t, 0, r1.Skipped)
    
    // Run 2: SKIP (same hash)
    r2, err := prov.Provision(ctx, input)
    require.NoError(t, err)
    assert.Equal(t, 0, r2.Created)
    assert.Equal(t, 1, r2.Skipped)
    
    // Run 3: UPDATE (changed spec)
    dbSpec.StorageGB = 100
    r3, err := prov.Provision(ctx, input)
    require.NoError(t, err)
    assert.Equal(t, 0, r3.Created)
    assert.Equal(t, 1, r3.Updated)
    
    // Verify state_version incremented
    rec, _ := store.GetResource(ctx, "env1", "mydb")
    assert.Equal(t, 3, rec.StateVersion) // 1(create) + 1(update) + initial
}
```

---

## 8. Integration Test — Full Pipeline (TA09)

### 8.1 TestPipeline_MockProvider_FullCycle

```go
func TestPipeline_MockProvider_FullCycle(t *testing.T) {
    // Setup
    registry := providers.NewRegistry()
    registry.Register(mock.NewProvider())
    store := state.NewSQLiteStore(":memory:")
    
    cfg := &config.Config{Provider: "mock", Region: "mock-region-1"}
    meta := &mapper.BuildMeta{
        AppName:     "testapp",
        Services:    []mapper.ServiceMeta{{Name: "api", Port: 8080}},
        Databases:   []mapper.DatabaseMeta{{Name: "mydb"}},
        Caches:      []mapper.CacheMeta{{Name: "mycache"}},
        BuildCommit: "abc1234567890123456789012345678901234567",
        BuildImage:  "testapp:abc1234",
    }
    
    // Test: Map → Provision → Generate
    m := mapper.NewMapper(registry)
    resources, err := m.Map(&config.ResolvedEnv{
        Name: "test", Provider: "mock", Region: "mock-region-1",
    }, meta)
    require.NoError(t, err)
    assert.Len(t, resources, 3) // db + cache + compute
    
    // Provision
    prov := provisioner.NewProvisioner(store, mockCreds)
    result, err := prov.Provision(ctx, provisioner.ProvisionInput{
        EnvID: "env-test", Resources: resources,
        Provider: registry.MustGet("mock"),
    })
    require.NoError(t, err)
    assert.Equal(t, 3, result.Created)
    
    // Generate infracfg
    gen := infragen.NewGenerator()
    cfg, err := gen.Generate(result.Outputs, meta)
    require.NoError(t, err)
    assert.Contains(t, cfg.SQLServers, "mydb")
    assert.Contains(t, cfg.Redis, "mycache")
    
    // Verify state
    recs, _ := store.ListResources(ctx, "env-test")
    assert.Len(t, recs, 3)
    
    // Run again: all skipped (idempotent)
    r2, err := prov.Provision(ctx, provisioner.ProvisionInput{
        EnvID: "env-test", Resources: resources,
        Provider: registry.MustGet("mock"),
    })
    require.NoError(t, err)
    assert.Equal(t, 0, r2.Created)
    assert.Equal(t, 3, r2.Skipped)
}
```

### 8.2 TestPipeline_StateVersionIncrement

| # | Scenario | Expected |
|---|----------|----------|
| 1 | First deploy | All resources version=1 |
| 2 | Same config redeploy | All skipped, versions unchanged |
| 3 | Modified config redeploy | Changed resources version=2, unchanged skipped |

---

## 9. Test Infrastructure

### 9.1 Test Helpers

```go
// internal/testutil/helpers.go

// TempDir creates a temp directory with infracast.yaml
func TempProjectDir(t *testing.T, yamlContent string) string

// MockBuildMeta creates a standard BuildMeta for testing
func MockBuildMeta() *mapper.BuildMeta

// EncoreProjectFiles creates minimal Encore project Go files
func EncoreProjectFiles(t *testing.T, dir string, databases []string, caches []string)
```

### 9.2 Mock Implementations

| Mock | Package | Purpose |
|------|---------|---------|
| `mock.Provider` | `providers/mock` | ✅ Already exists (TA01) |
| `mock.Store` | `internal/state/mock` | In-memory state for provisioner tests |
| `mock.CredentialManager` | `internal/credentials/mock` | Returns static credentials |
| `mock.STSClient` | `internal/credentials/mock` | Simulates STS API responses |

### 9.3 Test Execution

```bash
# Run all unit tests
make test

# Run with coverage
make test-coverage   # target: >= 80% per package

# Run integration tests (requires Alicloud credentials)
make test-integration

# Run specific module tests
go test ./internal/state/... -v -race
go test ./internal/provisioner/... -v -race
go test ./pkg/hash/... -v -race
```

---

## 10. Test Count Summary

| Module | Happy | Boundary | Error/Attack | Total |
|--------|-------|----------|-------------|-------|
| Config Parser | 5 | 14 | 4 | **23** |
| Service Mapper | 5 | 9 | 3 | **17** |
| spec_hash | 3 | 8 | 1 | **12** |
| State Store | 5 | 5 | 3 | **13** |
| Config Generator | 4 | 5 | 2 | **11** |
| Credential Manager | 5 | 7 | 6 | **18** |
| Provisioner | 6 | 4 | 4 | **14** |
| Integration | 3 | 2 | 0 | **5** |
| **Total** | **36** | **54** | **23** | **113** |

---

## Acceptance Criteria (Phase 5)

- [ ] Every function in Tech Spec has Happy Path + Boundary + Error test cases
- [ ] Table-driven test format used for all multi-case tests
- [ ] Concurrent test for State Store optimistic locking included with code
- [ ] Integration test covers Map → Provision → Generate → Idempotent cycle
- [ ] Mock implementations listed for all external dependencies
- [ ] Test count >= 100 across all modules
- [ ] Coverage target documented (>= 80%)

---

*— End of Document —*

*Infracast Test Specification v1.1 (Frozen) | Phase 5 of dev-lifecycle | Confidential*
