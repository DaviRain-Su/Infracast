# Changelog

All notable changes to Infracast will be documented in this file.

Format follows [Keep a Changelog](https://keepachangelog.com/).

## [v0.1.0-rc1] — 2026-04-15

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
- **`infracast status` is a stub**: Does not support `--output` flag; use `kubectl` for detailed status
- **No HTTPS/TLS termination**: Deploy pipeline does not configure Ingress TLS; manual Ingress setup required for production
- **No secrets rotation**: Generated database/cache passwords are static after provisioning

### Dependencies

- Go 1.22+
- Encore CLI (latest)
- Docker
- kubectl 1.28+
- Alibaba Cloud account with RDS/Redis/OSS/VPC/ACK/ACR access
