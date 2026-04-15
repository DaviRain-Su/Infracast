# Changelog

All notable changes to Infracast will be documented in this file.

Format follows [Keep a Changelog](https://keepachangelog.com/).

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
