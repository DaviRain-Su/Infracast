# Changelog

All notable changes to Infracast will be documented in this file.

Format follows [Keep a Changelog](https://keepachangelog.com/).

## [v0.2.0] — 2026-04-15

### Added

- **`status --output json|yaml`**: Structured output for CI/CD integration via `--output json` or `--output yaml`. Default table behavior unchanged
- **Error observability in status**: Failed resources now show actionable hints (7 known patterns: EPROV001, EPROV003, EDEPLOY076, NotEnoughBalance, timeout, unauthorized, quota)
- **`--set key=value` config override**: `infracast deploy --set region=cn-shanghai --set replicas=3` overrides config file values. Priority: CLI `--set` > env-specific YAML > file-level YAML > defaults
- **`infracast rollback` command**: Image-based rollback via `infracast rollback --env dev --image <tag>`. Updates K8s deployment, waits for stabilization, runs health check, logs to audit store with trace ID

### Fixed

- **Nil `Spec.Replicas` in rollback**: Added nil guard in `isRollbackStable` (same pattern as v0.1.6 F3)

### Known Limitations

- `rollback` updates `container[0]` only — multi-container pods would need extension (single-container is the current Infracast pattern)

## [v0.1.6] — 2026-04-15

### Fixed

- **P0: `commit[:7]` panic (F1)**: Added `shortCommit()` bounds check — prevents runtime panic when commit string is shorter than 7 characters
- **P0: K8s deployment timeout bypass (F2)**: `WaitForDeployment` now uses `timeoutCtx` instead of parent `ctx` for API calls — timeout mechanism was silently ineffective
- **P0: Nil `Spec.Replicas` dereference (F3)**: Added nil guard in `health.go` and `k8s.go` — K8s API can return nil Replicas, defaulting to 1
- **P0: `generateRandomPassword` panic (F4)**: Changed `pickRandChar`/`generateRandomPassword` from panic to error return — prevents provisioning crash on crypto/rand failure
- **P1: Audit query scan errors swallowed (F7)**: Scan errors now propagated via `lastScanErr` instead of silent `continue`
- **P1: `RowsAffected` error ignored (F8)**: `store.go` now checks `RowsAffected()` error, returns `ESTATE003` on failure
- **P1: `updateResourceStatus` error swallowed (F9)**: Provisioner now logs UpsertResource failures to stderr
- **P1: Nil map panic in infragen (F10)**: `mapSQLServer`/`mapRedis`/`mapObjectStore` use `getOrDefault()` for nil-safe map access

### Known Follow-ups (deferred to v0.1.7)

- F5: `finalizeResult` uses `context.Background()` — intentional design (wontfix)
- F6: `run.go LoadConfig` stub returns hardcoded defaults — known placeholder
- F11: VSwitch orphan on Redis partial failure — requires architectural change
- D1-D6: Signal handler leaks, parseInt silent failure, hardcoded constants, etc.

## [v0.1.5] — 2026-04-15

### Fixed

- **Hardcoded AppName (P1)**: `deploy` and `provision` commands now read `app` field from `infracast.yaml` via `Config.AppName()` instead of hardcoded `"my-app"`
- **BuildMeta not passed to config generation (P1)**: `stepGenerateConfig` now uses `BuildResult.BuildMeta` (with services, databases, caches) instead of a bare `AppName`-only fallback
- **BuildImage missing from BuildMeta (P2)**: `Build()` now populates `BuildMeta.BuildImage` with the parsed image tag from encore build output
- **Empty endpoint after provision (P1)**: Added `EDEPLOY076` validation — database and cache endpoints are checked for empty string after provisioning (catches resources still initializing)

### Added

- **`app` config field**: `infracast.yaml` now supports `app: <name>` to set the application name (fallback: `"my-app"`)
- **Rollback audit logging**: Health check failures that trigger rollback are now logged to the audit store with `AuditActionRollback` and success/failure status
- **Provisioning polling constants**: `ProvisionPollTimeout`, `ProvisionPollInterval`, `VPCPollTimeout`, `VPCPollInterval` replace magic numbers
- **ConfigJSON persistence**: Provisioner writes resource output as JSON to state store after successful apply
- **Regression tests**: `TestConfig_AppName`, `TestExtractBuildMeta_BuildImagePopulated`, `TestStepGenerateConfig_UsesBuildResultMeta`, `TestPipeline_SetAuditStore`, `TestProvisionPollConstants`, `TestApply_PersistsEndpointInConfigJSON`

## [v0.1.4] — 2026-04-15

### Fixed

- **State store resource leaks (P0)**: Added `defer store.Close()` to all state store opens in `status.go`, `provision.go`, `deploy.go` (validateEnvironment), and `env.go` (loadEnvironments, saveEnvironment, deleteEnvironment)
- **Deploy pipeline redundancy (P1)**: Replaced 4 independent `Pipeline.Execute()` calls (each running all 7 steps) with a single execution via `runDeployPipeline()` — eliminates 3× redundant pipeline runs
- **Duplicate helper function (P1)**: Removed `envAnyProvision()` from provision.go, unified on `envAny()` from destroy.go
- **Destroy error codes (P1)**: Added structured error codes `EDESTROY001`–`EDESTROY005` to all destroy command error paths (was plain text)
- **`setDefaultEnvironment` crash (P2)**: Now creates `.infra/` directory before writing `default-env` file (was failing if directory didn't exist)
- **Silent error in `loadEnvironments` (P2)**: `ListResourcesByEnv` errors are now surfaced instead of silently discarded

### Added

- **Regression tests**: `TestDestroyErrorCodesStructured`, `TestDestroyCredentialErrorCode`, `TestSetDefaultEnvironmentCreatesDir`, `TestDeployPipelineUsedOnce`

## [v0.1.3] — 2026-04-15

### Changed

- **`provision` command**: Wired to `internal/provisioner` — now loads `infracast.yaml`, validates credentials, creates Alicloud provider, and calls real provisioning pipeline (was a stub printing text)
- **`deploy` step functions**: Wired to `internal/deploy.Pipeline.Execute()` — build, push, provision, k8s-deploy, verify now delegate to real pipeline (were `time.Sleep` placeholders)
- **`loadDeployConfig`**: Reads `infracast.yaml` via `config.Load()` with environment-specific overrides, falls back to defaults when file is missing
- **`validateEnvironment`**: Queries state store for user-created environments, falls back to well-known defaults (dev/staging/production/local); error message now guides user to `env create`

### Fixed

- **Environment validation mismatch**: `deploy --env <name>` now accepts any environment created via `env create`, not just a hardcoded whitelist of 4 names

## [v0.1.2] — 2026-04-15

### Fixed

- **Schema CHECK constraint**: Added `environment` to `resource_type` CHECK — `env create` was blocked at DB level when writing `_env_meta` record
- **`_env_meta` display leak**: Filtered internal `_env_meta` records from `status` and `env` resource counts and detail views
- **`env list` hardcoded provider/region**: Now parses `ConfigJSON` from `_env_meta` to show actual provider and region values
- **`env show` color scheme**: Aligned resource status colors with `status` command (ready/created=green, failed/error=red, other=yellow)
- **ECFG005 error message**: Now includes guidance text `(v0.1.x supports alicloud only)`

### Added

- **Regression tests**: 27 new tests across `deploy_test.go` (13), `provision_test.go` (5), `destroy_test.go` (9) covering command structure, flags, validation, safety defaults, and helper functions

## [v0.1.1] — 2026-04-15

### Fixed

- **`status` command**: Wired to state store — now shows environment list with resource counts and per-env detail view (was a stub printing placeholder text)
- **`env` commands**: `list`, `show`, `create`, `delete` now backed by state store instead of hardcoded data; `isValidProvider` restricted to `alicloud` only (single-cloud constraint)
- **DB path hardcode**: `logs` command reads `INFRACAST_STATE_DB` env var for state database path, falls back to `.infra/state.db` (was hardcoded TODO)

### Changed

- **`status --env` default**: Changed from `dev` to empty (shows all environments when omitted)

## [v0.1.0] — 2026-04-15

### Added

- **CLI Commands**: `init`, `env create/list/use`, `provision`, `deploy`, `destroy`, `status`, `logs`, `run`, `version`
- **Alicloud Provider**: VPC, VSwitch, RDS PostgreSQL, Redis, OSS provisioning with automatic credential generation
- **Deploy Pipeline**: Build (Encore) → Push (ACR) → Deploy (K8s) → Health Check → Notify, with trace ID correlation
- **Automatic Rollback**: Failed health checks trigger `kubectl rollout undo` with safety checks (destructive migration detection)
- **Audit Logging**: SQLite-backed audit trail with `--format json`, `--output wide`, `--trace`, `--level`, `--since` filters
- **Notifications**: Feishu and DingTalk webhook support for deploy success/failure/rollback events
- **Resource Cleanup**: `destroy` command with `--dry-run`, `--apply`, `--keep-vpc` flags; bulk cleanup via `cmd/cleanup`
- **Error Code System**: 78 structured error codes across 5 modules (ECFG, EDEPLOY, EPROV, EIGEN, ESTATE)
- **Release Build**: Cross-platform binaries (darwin/amd64, darwin/arm64, linux/amd64) with SHA-256 checksums
- **Regression Suite**: `make regression` one-command regression script
- **Example Apps**: hello-world, todo-app, blog-api, web-app, migration, health-check
- **Documentation**: Getting started, deployment manual, error code matrix, operations runbook, prerequisites checklist, demo script

### Known Limitations

- **ACK Verify deferred**: Full E2E deploy verification requires ACK cluster with sufficient account balance; `NotEnoughBalance` blocks node pool scaling
- **Multi-cloud frozen**: Only Alicloud is supported; Huawei Cloud, Tencent Cloud, Volcengine adapters are not implemented
- **`infracast status`**: Does not support `--output` flag; shows state store data only (not live cloud state)
- **No HTTPS/TLS termination**: Deploy pipeline does not configure Ingress TLS; manual Ingress setup required for production
- **No secrets rotation**: Generated database/cache passwords are static after provisioning

### Dependencies

- Go 1.22+
- Encore CLI (latest)
- Docker
- kubectl 1.28+
- Alibaba Cloud account with RDS/Redis/OSS/VPC/ACK/ACR access
