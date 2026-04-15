# E2E Tests for Infracast

This directory contains end-to-end tests for the Infracast deployment pipeline.

## Test Levels

### 1. Smoke Test (`E2E_TEST=1`)
Basic smoke tests that validate the pipeline structure without requiring cloud credentials:

```bash
E2E_TEST=1 go test ./e2e/... -v
```

Tests include:
- Config generation
- K8s manifest generation
- Pipeline execution validation (expects failures without clients)
- Error code verification

### 2. Full E2E Deployment (`E2E_FULL=1`)
Complete deployment test requiring real AliCloud credentials and K8s cluster access:

```bash
export ALICLOUD_ACCESS_KEY="your-access-key"
export ALICLOUD_SECRET_KEY="your-secret-key"
export ALICLOUD_REGION="cn-hangzhou"
export ACR_NAMESPACE="infracast"
export KUBECONFIG="/path/to/kubeconfig"
export ACK_CLUSTER_ID="your-cluster-id"
export ENCORE_APP_ROOT="$(pwd)/e2e/testapp"

E2E_FULL=1 go test ./e2e/... -v -run TestFullE2EDeployment -count=1
```

**Requirements:**
- AliCloud account with ACR, ACK, RDS, and Redis permissions
- Existing ACK cluster
- Kubeconfig with cluster admin access
- Encore CLI installed (for build step)

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `E2E_TEST` | No | Set to `1` to run smoke tests |
| `E2E_FULL` | No | Set to `1` to run full E2E tests |
| `ALICLOUD_ACCESS_KEY` | For E2E_FULL | AliCloud Access Key ID |
| `ALICLOUD_SECRET_KEY` | For E2E_FULL | AliCloud Access Key Secret |
| `ALICLOUD_REGION` | No | Region (default: cn-hangzhou) |
| `ACR_NAMESPACE` | No | ACR namespace (default: infracast) |
| `KUBECONFIG` | For E2E_FULL | Path to kubeconfig file |
| `ACK_CLUSTER_ID` | For E2E_FULL | ACK cluster ID |

## Pipeline Steps Tested

The full E2E test validates all 7 deployment steps:

1. **Build** - Runs `encore build docker`
2. **Push** - Pushes image to ACR with authentication
3. **Provision** - Creates cloud resources (RDS, Redis)
4. **Generate Config** - Creates `infracfg.json`
5. **Deploy** - Applies K8s manifests
6. **Verify** - Health checks
7. **Notify** - Deployment notifications

## CI/CD Integration

For CI/CD pipelines, use the smoke test mode:

```yaml
- name: E2E Smoke Test
  run: E2E_TEST=1 go test ./e2e/... -v
```

Full E2E tests should be run manually or in dedicated integration environments.

## Regression Command (Single-Cloud Mini Sprint)

```bash
set -a; source .env; set +a
export ENCORE_APP_ROOT="$(pwd)/e2e/testapp"
E2E_FULL=1 go test ./e2e/... -v -run TestFullE2EDeployment -count=1
```
