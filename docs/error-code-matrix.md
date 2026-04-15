# Error Code Matrix

Complete reference for all Infracast error codes. Use this to diagnose failures and find actionable fixes.

See also: [Deployment Manual — Failure Decision Tree](deployment-manual.md#3-failure-decision-tree)

---

## ECFG — Configuration Errors

Source: `internal/config/`, `cmd/infracast/internal/commands/init.go`, `cmd/infracast/internal/commands/run.go`

| Code | Source | Message | Trigger Condition | Actionable Fix | Retryable |
|------|--------|---------|-------------------|----------------|-----------|
| ECFG001 | `internal/config/errors.go` | provider is required | `infracast.yaml` missing `provider` field, or `init` without app name | Add `provider: alicloud` to config, or pass app name to `infracast init` | N |
| ECFG002 | `internal/config/errors.go` | region is required | `infracast.yaml` missing `region` field, or invalid app name in `init` | Add `region: cn-hangzhou` to config | N |
| ECFG003 | `internal/config/errors.go` | environment name is required | `--env` flag omitted where required, or config file not found | Add `--env dev` flag, or run `infracast init` first | N |
| ECFG004 | `internal/config/errors.go` | unsupported provider | Provider value not in allowed list (`alicloud`) | Use `provider: alicloud` | N |
| ECFG005 | `internal/config/errors.go` | invalid region format | Region string doesn't match expected format | Use valid Alicloud region (e.g., `cn-hangzhou`, `cn-shanghai`) | N |
| ECFG006 | `internal/config/errors.go` | invalid environment name | Env name contains invalid characters | Use alphanumeric + hyphens only (e.g., `dev`, `staging`, `production`) | N |
| ECFG007 | `internal/config/errors.go` | storage_gb out of range | `storage_gb` override < 20 or > 32768 | Set `storage_gb` between 20 and 32768 | N |
| ECFG008 | `internal/config/errors.go` | replicas out of range | `replicas` override < 1 or > 100 | Set `replicas` between 1 and 100 | N |
| ECFG009 | `internal/config/errors.go` | invalid CPU format | CPU resource format unrecognized | Use format like `"500m"` or `"2"` | N |
| ECFG010 | `internal/config/errors.go` | invalid memory format | Memory resource format unrecognized | Use format like `"512Mi"` or `"2Gi"` | N |
| ECFG011 | `internal/config/errors.go` | invalid database engine | DB engine not in supported list | Use `postgresql` (Alicloud RDS) | N |
| ECFG012 | `internal/config/errors.go` | invalid database version | DB version not supported | Check Alicloud RDS supported versions | N |
| ECFG013 | `internal/config/errors.go` | invalid instance class | RDS instance class not recognized | Use valid Alicloud class (e.g., `pg.n2.medium.1`) | N |
| ECFG014 | `internal/config/errors.go` | memory_mb out of range | `memory_mb` < 256 or > 65536 | Set between 256 and 65536 | N |
| ECFG015 | `internal/config/errors.go` | invalid cache engine | Cache engine not in supported list | Use `redis` | N |
| ECFG016 | `internal/config/errors.go` | invalid cache version | Cache version not supported | Check Alicloud Redis supported versions | N |
| ECFG017 | `internal/config/errors.go` | invalid ACL value | OSS ACL value not recognized | Use `private`, `public-read`, or `public-read-write` | N |
| ECFG018 | `internal/config/errors.go` | invalid eviction policy | Redis eviction policy not recognized | Use `noeviction`, `allkeys-lru`, etc. | N |
| ECFG019 | `internal/config/errors.go` | environment not found | `--env` value doesn't match any defined environment | Run `infracast env list` to see available envs | N |
| ECFG020 | `internal/config/errors.go` | failed to load configuration | `infracast.yaml` missing, corrupt, or unreadable | Run `infracast init` or fix YAML syntax | N |
| ECFG021 | `internal/config/errors.go` | environment name too long | Env name exceeds 50 characters | Use a shorter environment name | N |

---

## EDEPLOY — Deploy Pipeline Errors

Source: `internal/deploy/`, `cmd/infracast/internal/commands/deploy.go`, `cmd/infracast/internal/commands/run.go`

### Build (EDEPLOY001–002)

| Code | Source | Message | Trigger Condition | Actionable Fix | Retryable |
|------|--------|---------|-------------------|----------------|-----------|
| EDEPLOY001 | `internal/deploy/build.go` | build failed / timeout | Docker build fails or exceeds timeout | Check Dockerfile, ensure Docker daemon running (`docker info`), check build output | Y |
| EDEPLOY002 | `internal/deploy/build.go` | invalid build metadata | AppName, BuildCommit, or Services missing | Ensure project has valid Encore app config | N |

### K8s Deployment (EDEPLOY010–015)

| Code | Source | Message | Trigger Condition | Actionable Fix | Retryable |
|------|--------|---------|-------------------|----------------|-----------|
| EDEPLOY010 | `internal/deploy/k8s.go` | manifest empty / namespace error | Generated manifest is empty or namespace creation fails | Check infracfg.json generation, verify cluster connectivity | Y |
| EDEPLOY011 | `internal/deploy/k8s.go` | K8s client init failed | KUBECONFIG missing or invalid, ACK cluster unreachable | `export KUBECONFIG=~/.kube/config`, verify ACK cluster is running | N |
| EDEPLOY012 | `internal/deploy/k8s.go` | infracfg.json read failed / manifest generation failed | Config file missing after provision step | Re-run `infracast provision --env <env>` | N |
| EDEPLOY013 | `internal/deploy/k8s.go` | ConfigMap apply failed | K8s API rejected ConfigMap | Check cluster permissions, verify ConfigMap content | Y |
| EDEPLOY014 | `internal/deploy/k8s.go` | Deployment apply failed | K8s API rejected Deployment manifest | Check cluster permissions, verify resource limits | Y |
| EDEPLOY015 | `internal/deploy/k8s.go` | Service apply failed | K8s API rejected Service manifest | Check cluster permissions, verify port config | Y |

### Image Push (EDEPLOY040–047)

| Code | Source | Message | Trigger Condition | Actionable Fix | Retryable |
|------|--------|---------|-------------------|----------------|-----------|
| EDEPLOY040 | `internal/deploy/docker.go` | image push failed after retries / image empty | Push to ACR failed 3 times, or local image reference empty | Check ACR credentials (`docker login`), verify image built successfully | Y |
| EDEPLOY041 | `internal/deploy/docker.go` | context cancelled during push | Push cancelled (timeout or user interrupt) | Retry the deploy | Y |
| EDEPLOY042 | `internal/deploy/docker.go` | invalid image name | Image name format invalid | Check app config for valid image naming | N |
| EDEPLOY043 | `internal/deploy/docker.go` | failed to parse source image | Local image reference unparseable | Verify Docker build produced valid image | N |
| EDEPLOY044 | `internal/deploy/docker.go` | failed to parse destination image | ACR image reference unparseable | Check ACR registry URL config | N |
| EDEPLOY045 | `internal/deploy/docker.go` | failed to load source image | Image not found in local daemon or registry | Ensure build step completed, run `docker images` | N |
| EDEPLOY046 | `internal/deploy/docker.go` | failed to push to ACR | ACR rejected the push | Re-authenticate: `docker login <acr-url>` | Y |
| EDEPLOY047 | `internal/deploy/docker.go` | failed to get ACR auth | ACR authentication token retrieval failed | Check `ALICLOUD_ACCESS_KEY`/`ALICLOUD_SECRET_KEY` | N |

### Health & Verification (EDEPLOY050–057)

| Code | Source | Message | Trigger Condition | Actionable Fix | Retryable |
|------|--------|---------|-------------------|----------------|-----------|
| EDEPLOY050 | `internal/deploy/health.go` | deployment timeout | Pods didn't become ready within timeout; rollback may trigger | Check app logs (`kubectl logs`), verify `/health` endpoint works | Y |
| EDEPLOY051 | `internal/deploy/health.go` | failed to get deployment status | K8s API query failed during health polling | Check cluster connectivity, KUBECONFIG | Y |
| EDEPLOY052 | `internal/deploy/health.go` | failed to get deployment | Deployment resource not found during health check | Verify deployment was applied (EDEPLOY014) | N |
| EDEPLOY053 | `internal/deploy/health.go` | K8s client not initialized (health) | Client nil during health check | Ensure KUBECONFIG is set | N |
| EDEPLOY054 | `internal/deploy/health.go` | failed to get service | Service resource not found | Verify service was applied (EDEPLOY015) | N |
| EDEPLOY055 | `internal/deploy/health.go` | failed to create health check request | HTTP request construction failed | Check health endpoint URL format | N |
| EDEPLOY056 | `internal/deploy/health.go` | health check HTTP failed | Network error connecting to health endpoint | Check pod status, network policies | Y |
| EDEPLOY057 | `internal/deploy/health.go` | health check non-OK status | Health endpoint returned non-200 | Check application logs, verify app starts correctly | Y |

### Rollback (EDEPLOY060–068)

| Code | Source | Message | Trigger Condition | Actionable Fix | Retryable |
|------|--------|---------|-------------------|----------------|-----------|
| EDEPLOY060 | `internal/deploy/rollback.go` | no previous revision / K8s client not initialized | First deployment (nothing to rollback to), or client nil | For first deploy: fix the deployment issue directly | N |
| EDEPLOY061 | `internal/deploy/rollback.go` | rollback execution failed / failed to get deployment | K8s API error during rollback | Check cluster permissions, connectivity | Y |
| EDEPLOY062 | `internal/deploy/rollback.go` | rollback did not stabilize / K8s client not initialized | Rolled-back pods didn't become healthy | Manual intervention: `kubectl rollout status` | Y |
| EDEPLOY063 | `internal/deploy/rollback.go` | failed to get deployment status | Status check failed during rollback monitoring | Check cluster connectivity | Y |
| EDEPLOY064 | `internal/deploy/rollback.go` | rollback progress deadline exceeded | Rollback itself timed out | Manual rollback: `kubectl rollout undo` | N |
| EDEPLOY065 | `internal/deploy/rollback.go` | K8s client not initialized (rollback) | Client nil during safe-to-rollback check | Ensure KUBECONFIG is set | N |
| EDEPLOY066 | `internal/deploy/rollback.go` | deployment not found (rollback) | Target deployment doesn't exist | Verify deployment name and namespace | N |
| EDEPLOY067 | `internal/deploy/rollback.go` | no previous revision (safe check) | Only one revision exists | Cannot rollback first deployment | N |
| EDEPLOY068 | `internal/deploy/rollback.go` | destructive migrations block rollback | Deployment includes irreversible DB migrations | Manual intervention required; do not rollback | N |

### Pipeline (EDEPLOY070–082)

| Code | Source | Message | Trigger Condition | Actionable Fix | Retryable |
|------|--------|---------|-------------------|----------------|-----------|
| EDEPLOY070 | `internal/deploy/pipeline.go` | build failed (pipeline) | Encore build step failed | Check build output, verify Encore CLI installed | Y |
| EDEPLOY071 | `internal/deploy/pipeline.go` | build result required for provision | Provision step called without prior build | Ensure build step runs first | N |
| EDEPLOY072 | `internal/deploy/pipeline.go` | ACR credentials required / init failed | Missing Alicloud credentials for container registry | Set `ALICLOUD_ACCESS_KEY` and `ALICLOUD_SECRET_KEY` | N |
| EDEPLOY073 | `internal/deploy/pipeline.go` | AliCloud credentials required | Provision step needs cloud credentials | Set `ALICLOUD_ACCESS_KEY` and `ALICLOUD_SECRET_KEY` | N |
| EDEPLOY074 | `internal/deploy/pipeline.go` | failed to create provider | Alicloud provider initialization failed | Verify credentials and region | N |
| EDEPLOY075 | `internal/deploy/pipeline.go` | failed to provision resource | Database, cache, or OSS provisioning failed | Check Alicloud console, verify quota and balance | Y |
| EDEPLOY080 | `cmd/.../run.go` | encore CLI not found / notify failed | Encore not in PATH, or notification webhook failed | Install Encore CLI: https://encore.dev/docs/install | N |
| EDEPLOY081 | `cmd/.../run.go` | failed to start encore run | Encore dev server failed to start | Check Encore project config, verify `encore.app` exists | N |
| EDEPLOY082 | `cmd/.../run.go` | prerequisite not found | Required tool missing from PATH | Install the missing tool (shown in error message) | N |

---

## EPROV — Provisioner Errors

Source: `internal/provisioner/`

| Code | Source | Message | Trigger Condition | Actionable Fix | Retryable |
|------|--------|---------|-------------------|----------------|-----------|
| EPROV001 | `internal/provisioner/errors.go` | failed to fetch credentials | Alicloud credentials missing or invalid | Set `ALICLOUD_ACCESS_KEY`/`ALICLOUD_SECRET_KEY`, verify RAM permissions | **N** |
| EPROV002 | `internal/provisioner/errors.go` | cloud provider SDK retryable error | Transient cloud API error (rate limit, network) | Retry the command | **Y** |
| EPROV003 | `internal/provisioner/errors.go` | cloud resource quota exceeded | Insufficient balance or VPC/RDS/Redis quota | Top up account, request quota increase, or use smaller specs | N |
| EPROV004 | `internal/provisioner/errors.go` | resource dependency conflict | Resources have conflicting or unresolvable dependencies | Check resource ordering, verify VPC/VSwitch exist | Y |
| EPROV005 | `internal/provisioner/errors.go` | invalid resource specification | Resource spec fails validation | Check `infracast.yaml` resource definitions | N |
| EPROV006 | `internal/provisioner/errors.go` | resource dependency not met | Required upstream resource not provisioned | Provision dependencies first (VPC before VSwitch before RDS) | N |
| EPROV007 | `internal/provisioner/errors.go` | resource destruction failed | Cloud API rejected delete request | Wait for async release, retry after 3–10 minutes | **Y** |
| EPROV008 | `internal/provisioner/errors.go` | failed to compute spec hash | Internal hash computation error | Report as bug | N |
| EPROV009 | `internal/provisioner/errors.go` | resource not found | Expected cloud resource doesn't exist | Re-run provision, or check if resource was manually deleted | N |
| EPROV010 | `internal/provisioner/errors.go` | concurrent update detected | Another operation modifying same resource | Wait and retry | **Y** |

---

## EIGEN — Infragen (Config Generation) Errors

Source: `pkg/infragen/`, `internal/infragen/`

| Code | Source | Message | Trigger Condition | Actionable Fix | Retryable |
|------|--------|---------|-------------------|----------------|-----------|
| EIGEN001 | `pkg/infragen/generator.go` | invalid / unsupported resource type | Provision result contains unknown resource type | Check `infracast.yaml` for supported resource types | N |
| EIGEN002 | `pkg/infragen/generator.go` | missing required field | Generated config missing required connection details | Verify provision completed successfully for all resources | N |
| EIGEN003 | `pkg/infragen/generator.go` | write failed | Failed to write `infracfg.json` to disk | Check disk permissions, ensure `.infra/` directory exists | N |

---

## ESTATE — State Database Errors

Source: `cmd/infracast/internal/commands/logs.go`

| Code | Source | Message | Trigger Condition | Actionable Fix | Retryable |
|------|--------|---------|-------------------|----------------|-----------|
| ESTATE001 | `cmd/.../commands/logs.go` | failed to open state database | `.infra/state.db` missing or corrupted | Run `infracast init` to create state directory, or check file permissions | N |
| ESTATE002 | `cmd/.../commands/logs.go` | failed to initialize audit table | SQLite schema migration failed | Check disk space, file permissions on `.infra/state.db` | N |

---

## Quick Lookup

Total error codes: **78**

| Module | Range | Count |
|--------|-------|-------|
| ECFG | 001–021 | 21 |
| EDEPLOY | 001–082 | 42 |
| EPROV | 001–010 | 10 |
| EIGEN | 001–003 | 3 |
| ESTATE | 001–002 | 2 |

**Not an error code?** If you see a cloud provider error (e.g., `NotEnoughBalance`, `ServiceLinkedRole.NotExist`), check the [Single-Cloud Operations Handbook](06-single-cloud-operations.md#3-troubleshooting-runbook) for Alicloud-specific troubleshooting.
