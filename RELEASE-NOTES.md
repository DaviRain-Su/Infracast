# Release Notes: v0.1.6

**Date**: 2026-04-15
**Status**: Patch Release (single-cloud, Alicloud only)

---

## What's Fixed in v0.1.6

### P0 — Critical Fixes

- **F1: `commit[:7]` panic** — New `shortCommit()` function prevents runtime panic when commit string is shorter than 7 characters (affects `Build`, `GetLocalImageName`, `StreamBuild`)
- **F2: K8s deployment timeout bypass** — `WaitForDeployment` now passes `timeoutCtx` (not parent `ctx`) to K8s API calls. Previously the timeout was silently ineffective
- **F3: Nil `Spec.Replicas` dereference** — Added nil guard before dereferencing `deployment.Spec.Replicas` in `health.go` and `k8s.go`. K8s API can return nil, now defaults to 1
- **F4: `generateRandomPassword` panic** — `pickRandChar` and `generateRandomPassword` now return `(T, error)` instead of panicking on crypto/rand failure or empty charset

### P1 — High Priority Fixes

- **F7: Audit query scan errors** — `Query()` in `audit.go` now propagates scan errors via `lastScanErr` instead of silently continuing with incomplete results
- **F8: `RowsAffected` error ignored** — `store.go` UpsertResource now checks `RowsAffected()` error, returning `ESTATE003` on failure instead of false `ErrConcurrentUpdate`
- **F9: Resource status update error swallowed** — Provisioner `updateResourceStatus` now logs `UpsertResource` failures to stderr
- **F10: Nil map panic in infragen** — `mapSQLServer`, `mapRedis`, `mapObjectStore` use new `getOrDefault()` for nil-safe map access with defaults

### Regression Tests Added

- `TestShortCommit_BoundsCheck` (5 cases including empty string)
- `TestGetLocalImageName_ShortCommit`
- `TestGenerateRandomPassword_ReturnsErrorNotPanic`
- `TestPickRandChar_EmptyCharset_ReturnsError`
- `TestGetOrDefault_NilMap`, `TestGetOrDefault_MissingKey`
- `TestMapSQLServer_NilOutputMap`

### Known Follow-ups (deferred)

- F5: `finalizeResult` uses `context.Background()` — intentional for post-cancel logging (wontfix)
- F6: `run.go LoadConfig` stub — known placeholder, not a regression
- F11: VSwitch orphan on Redis partial failure — requires architectural change
- D1-D6: Signal handler leaks, parseInt silent failure, hardcoded provider constants

---

## Previous Release: v0.1.5

**Date**: 2026-04-15
**Status**: Patch Release (single-cloud, Alicloud only)

## What's New in v0.1.5

### Stabilization (#151)

- **Provisioning polling constants**: `ProvisionPollTimeout` (10m), `ProvisionPollInterval` (30s), `VPCPollTimeout` (2m), `VPCPollInterval` (5s) replace magic numbers in alicloud provider
- **Empty endpoint validation**: `EDEPLOY076` error raised when database or cache endpoint is empty after provisioning — catches resources still initializing
- **ConfigJSON persistence**: Provisioner writes resource output as JSON to state store after successful apply

### Deploy Chain Improvements (#152)

- **`app` config field**: `infracast.yaml` now supports `app: <name>` to set the application name; `Config.AppName()` provides fallback to `"my-app"`
- **BuildMeta passthrough**: `stepGenerateConfig` uses full `BuildResult.BuildMeta` (AppName, Services, Databases, Caches, BuildImage) instead of bare AppName fallback
- **BuildImage populated**: `Build()` assigns parsed image tag to `BuildMeta.BuildImage`
- **Rollback audit logging**: Health check failures that trigger rollback are logged to audit store with `AuditActionRollback` and success/failure status; rollback failures get additional `AuditLevelError` entry

### Regression Tests Added

- `TestConfig_AppName` (3 sub-tests) — App field reading, empty fallback, YAML loading
- `TestExtractBuildMeta_BuildImagePopulated` — BuildImage assignment chain
- `TestStepGenerateConfig_UsesBuildResultMeta` — BuildResult.BuildMeta preferred over fallback
- `TestPipeline_SetAuditStore` — audit store injection
- `TestProvisionPollConstants` — polling constant values
- `TestApply_PersistsEndpointInConfigJSON` — ConfigJSON state persistence

---

## Previous Release: v0.1.4

**Date**: 2026-04-15
**Status**: Patch Release (single-cloud, Alicloud only)

## What's Fixed in v0.1.4

- **State store resource leaks (P0)**: All `state.NewStore()` calls now have `defer store.Close()` — prevents SQLite connection leaks in status, provision, deploy, and env commands
- **Deploy pipeline 4× redundancy (P1)**: Each deploy step (build/push/k8s-deploy/verify) was independently executing the full 7-step pipeline. Now runs once via `runDeployPipeline()`. Net reduction: ~160 lines, 3× fewer pipeline executions per deploy
- **Duplicate `envAnyProvision()` (P1)**: Removed from provision.go; unified on `envAny()` already in destroy.go
- **Destroy missing error codes (P1)**: Added `EDESTROY001`–`EDESTROY005` to all error paths (missing env, broad prefix, missing credentials, provider creation, partial failure)
- **`env use` crash without `.infra/` (P2)**: `setDefaultEnvironment()` now calls `os.MkdirAll(".infra", 0755)` before writing
- **Silent `loadEnvironments` error (P2)**: `ListResourcesByEnv` errors now propagate instead of being discarded with `_`

### Regression Tests Added

- `TestDestroyErrorCodesStructured` — validates `EDESTROY001` on empty env
- `TestDestroyCredentialErrorCode` — validates `EDESTROY003` on missing credentials
- `TestSetDefaultEnvironmentCreatesDir` — validates `.infra/` auto-creation + file content
- `TestDeployPipelineUsedOnce` — validates step result type compatibility

---

## Previous Release: v0.1.3

**Date**: 2026-04-15
**Status**: Patch Release (single-cloud, Alicloud only)

## Breaking Behavior Change in v0.1.3

**`infracast provision` and `infracast deploy` now execute real cloud operations.** In v0.1.0–v0.1.2, these commands were stubs that printed text or slept briefly. Starting with v0.1.3, they delegate to the internal provisioner and deploy pipeline, which will create real Alicloud resources when credentials are configured.

If you run `infracast provision` or `infracast deploy` with valid `ALICLOUD_ACCESS_KEY` and `ALICLOUD_SECRET_KEY`, real resources will be created and billed. Use `--dry-run` to preview.

## What's Changed in v0.1.3

- **`infracast provision`** now reads `infracast.yaml`, validates Alicloud credentials, maps config to resource specs, and calls the provisioner pipeline. Missing config returns `ECFG001`; missing credentials returns `EPROV001`.
- **`infracast deploy`** step functions (build/push/provision/k8s-deploy/verify) delegate to `Pipeline.Execute()` instead of placeholder sleeps.
- **`loadDeployConfig`** reads `infracast.yaml` with environment-specific overrides, falls back to defaults when file is missing.
- **`validateEnvironment`** queries the state store for user-created environments. Custom environments created via `env create` are now accepted by `deploy --env`. Error message guides user to `env create`.

---

## Previous Release: v0.1.2

**Date**: 2026-04-15
**Status**: Patch Release (single-cloud, Alicloud only)

### What's Fixed in v0.1.2

- **`env create` unblocked**: Schema CHECK constraint now accepts `environment` resource type
- **Clean resource counts**: Internal `_env_meta` records filtered from `status` and `env` output
- **`env list` shows real metadata**: Provider and region parsed from state store
- **Consistent status colors**: `env show` aligned with `status` command
- **Better error guidance**: `ECFG005` includes `(v0.1.x supports alicloud only)`
- **27 regression tests** added for deploy, provision, and destroy command paths

---

## Previous Release: v0.1.1

**Date**: 2026-04-15
**Status**: Patch Release (single-cloud, Alicloud only)

### What's Fixed in v0.1.1

- **`infracast status`** now queries the state store — shows environment summary with resource counts, or per-env detail with `--env`
- **`infracast env`** commands (`list`, `show`, `create`, `delete`) wired to state store, replacing hardcoded placeholder data
- **DB path configurable** via `INFRACAST_STATE_DB` environment variable (was hardcoded)
- **Provider constraint** enforced: `env create` only accepts `alicloud` (single-cloud)

---

## Previous Release: v0.1.0

**Date**: 2026-04-15
**Status**: General Availability (single-cloud, Alicloud only)

---

## What's New

Infracast v0.1.0 is the first stable release, delivering a complete single-cloud deployment pipeline for Encore applications on Alibaba Cloud.

### Single-Command Deploy

```bash
infracast deploy --env dev
```

One command handles: Docker build → ACR push → K8s deploy → health check verification. Failed health checks trigger automatic rollback.

### Full Infrastructure Lifecycle

| Command | Description |
|---------|-------------|
| `infracast init` | Initialize project with `infracast.yaml` |
| `infracast provision` | Create VPC, RDS, Redis, OSS resources |
| `infracast deploy` | Build, push, deploy, and verify |
| `infracast destroy` | Tear down resources (with `--dry-run` safety) |

### Observability

- **Trace IDs**: Every deploy/provision run gets a unique trace ID for pipeline correlation
- **Audit Logs**: `infracast logs --trace <id>` shows full pipeline with step-by-step status
- **JSON Output**: `infracast logs --format json` for scripting and CI/CD integration
- **78 Error Codes**: Structured codes (ECFG/EDEPLOY/EPROV/EIGEN/ESTATE) with documented fixes

### Notifications

Deploy results sent automatically to Feishu or DingTalk webhooks.

---

## Known Limitations

| Limitation | Impact | Workaround |
|-----------|--------|------------|
| ACK Verify deferred | Full E2E deploy needs ACK + sufficient balance | Use `--dry-run` for validation; top up account for real deploys |
| Multi-cloud frozen | Only Alicloud supported | No workaround; by design for v0.1.x |
| `status` shows state store only | No live cloud query or `--output` flag | Use `kubectl get svc/pods` for live state |
| No TLS termination | Deploy doesn't configure Ingress TLS | Manual Ingress/cert-manager setup |
| No secrets rotation | DB/cache passwords static after provision | Manual rotation via Alicloud console |

---

## Environment Requirements

### Minimum

- Go 1.22+
- Encore CLI (latest)
- Docker (running)
- kubectl 1.28+
- Alibaba Cloud account (real-name verified, balance ≥ ¥100)

### Alicloud Services

- RDS PostgreSQL
- Redis (Kvstore)
- OSS
- VPC
- ACK (Container Service for Kubernetes)
- ACR (Container Registry)

### RAM Permissions

- `AliyunRDSFullAccess`
- `AliyunKvstoreFullAccess`
- `AliyunOSSFullAccess`
- `AliyunVPCFullAccess`
- `AliyunCSFullAccess`
- `AliyunCRFullAccess`

See [Prerequisites Checklist](docs/prerequisites-checklist.md) for full verification steps.

---

## Cost Estimate

A single `dev` environment costs approximately **¥20–25/day** (pay-as-you-go):

| Resource | Approximate Cost |
|----------|-----------------|
| RDS pg.n2.medium.1 | ~¥8/day |
| Redis small | ~¥5/day |
| ACK worker node | ~¥5–10/day |
| OSS | < ¥1/day |

Run `infracast destroy --env dev --apply --keep-vpc 1` after testing to stop charges.

---

## Installation

### Download Binary

```bash
VERSION="v0.1.0"
# macOS Apple Silicon
curl -LO "https://github.com/DaviRain-Su/Infracast/releases/download/${VERSION}/infracast_${VERSION}_darwin_arm64.tar.gz"
tar xzf "infracast_${VERSION}_darwin_arm64.tar.gz"
sudo mv "infracast_${VERSION}_darwin_arm64/infracast" /usr/local/bin/
```

### Verify

```bash
shasum -a 256 -c checksums.txt --ignore-missing
infracast version
```

### Build from Source

```bash
git clone https://github.com/DaviRain-Su/Infracast.git
cd Infracast
make build
./bin/infracast version
```

---

## Documentation

| Document | Description |
|----------|-------------|
| [Getting Started](docs/getting-started.md) | 5-step quickstart |
| [Deployment Manual](docs/deployment-manual.md) | Full command flow |
| [Error Code Matrix](docs/error-code-matrix.md) | 78 error codes with fixes |
| [Operations Runbook](docs/runbook.md) | Incident response procedures |
| [Prerequisites](docs/prerequisites-checklist.md) | Environment verification |
| [Demo Script](docs/demo-script.md) | 10-15 min demo walkthrough |

---

## Upgrade Path

This is the initial release. Future v0.1.x patches will be backwards-compatible. Breaking changes (if any) will be documented in CHANGELOG.md and announced in release notes.
