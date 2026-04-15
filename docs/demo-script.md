# Infracast Demo Script (10–15 min)

Single-cloud (Alicloud) end-to-end demo: init → provision → deploy → verify → cleanup.

**Prerequisites**: Complete the [Prerequisites Checklist](prerequisites-checklist.md) before running this demo.

---

## Overview

| Step | Duration | Requires Cloud | Command |
|------|----------|----------------|---------|
| 0. Pre-flight | 1 min | No | Credential & tool check |
| 1. Init | 30 sec | No | `infracast init` |
| 2. Build (dry-run) | 1 min | No | `infracast deploy --env dev --dry-run` |
| 3. Provision | 3–5 min | **Yes** | `infracast provision --env dev` |
| 4. Deploy | 2–3 min | **Yes** | `infracast deploy --env dev` |
| 5. Verify | 1 min | **Yes** | Health checks + logs |
| 6. Audit trail | 30 sec | No | `infracast logs` |
| 7. Cleanup | 1–2 min | **Yes** | `infracast destroy` |

Steps marked **Yes** create real Alicloud resources and incur cost (~¥2–5 for a full demo run).
Steps 0–2 and 6 can be run without cloud access for a dry-run demo.

---

## Step 0: Pre-flight Check (1 min)

```bash
# Verify credentials
echo "Access Key: ${ALICLOUD_ACCESS_KEY:0:8}..."
echo "Region: $ALICLOUD_REGION"

# Verify tools
infracast version
encore version
docker info | head -3
kubectl cluster-info | head -1
```

**Expected output**:
```
Access Key: LTAI5t5o...
Region: cn-hangzhou
infracast version v0.1.0
  commit: abc1234
  build time: 2026-04-15_12:00:00
encore version v1.56.6
 Context:    default
Kubernetes control plane is running at https://xxx.xxx.xxx.xxx:6443
```

If any check fails, see [Prerequisites Checklist](prerequisites-checklist.md).

---

## Step 1: Initialize Project (30 sec)

```bash
infracast init demo-app --provider alicloud --region cn-hangzhou -y
cd demo-app
```

**Expected output**:
```
✓ Created infracast.yaml
✓ Created .infra/ directory
✓ Created .gitignore
✓ Project demo-app initialized for alicloud (cn-hangzhou)
```

Copy the health-check example app into the project:

```bash
cp -r /path/to/infracast/examples/health-check/* .
```

Review the generated config:

```bash
cat infracast.yaml
```

---

## Step 2: Build Dry-Run (1 min)

Test the build pipeline without cloud resources:

```bash
infracast deploy --env dev --dry-run
```

**Expected output**:
```
[DRY-RUN] Build: would run `encore build docker`
[DRY-RUN] Push: would push to ACR
[DRY-RUN] Deploy: would apply K8s manifests
No resources were created. Run without --dry-run to execute.
```

This validates that the CLI, config, and Encore project are correctly set up.

> **Dry-run demo stops here.** Steps 3–7 require real Alicloud resources.

---

## Step 3: Provision Infrastructure (3–5 min)

Create cloud resources (VPC, RDS, Redis):

```bash
infracast provision --env dev
```

**Expected output**:
```
✓ VPC created (vpc-bp1xxx)
✓ VSwitch created (vsw-bp1xxx)
✓ RDS PostgreSQL created (pgm-bp1xxx)
✓ Redis created (r-bp1xxx)
✓ Generated infracfg.json

Provision complete. Resources ready in env dev.
```

While waiting, explain what's happening:
- VPC + VSwitch: private network for all resources
- RDS PostgreSQL: managed database with auto-generated secure password
- Redis: managed cache with VPC-only access
- `infracfg.json`: connection details for the application

**If it fails**: Check the error code in [Error Code Matrix](error-code-matrix.md).
Common: `EPROV003` (NotEnoughBalance) → top up account.

---

## Step 4: Deploy Application (2–3 min)

Build, push, and deploy the application:

```bash
infracast deploy --env dev
```

**Expected output**:
```
Deploy to dev
  Trace: trc_1713184200000000000

  Step           Status  Duration
  ─────────────  ──────  ────────
  build          OK      12s
  push           OK      8s
  deploy         OK      15s
  health-check   OK      3s

  ✓ Deploy succeeded (4/4 steps passed, 38s total)
```

The pipeline:
1. **Build**: `encore build docker` creates the container image
2. **Push**: Image pushed to Alicloud Container Registry (ACR)
3. **Deploy**: Kubernetes manifests applied to ACK cluster
4. **Health-check**: `/livez` endpoint verified healthy

**If health-check fails** (EDEPLOY050):
```bash
# Check what went wrong
infracast logs --trace trc_xxx
kubectl get pods -n demo-app-dev
kubectl logs <pod-name> -n demo-app-dev
```

---

## Step 5: Verify Deployment (1 min)

Check application health:

```bash
# Liveness probe
curl -s "$(infracast status --env dev --output url)/livez" | jq .
```

**Expected output**:
```json
{
  "status": "ok",
  "uptime": "45s"
}
```

```bash
# Readiness probe
curl -s "$(infracast status --env dev --output url)/readyz" | jq .
```

**Expected output**:
```json
{
  "status": "ready",
  "checks": {
    "self": "ok"
  }
}
```

```bash
# Diagnostics (shows connected resources)
curl -s "$(infracast status --env dev --output url)/diag" | jq .
```

---

## Step 6: Review Audit Trail (30 sec)

Show the deployment history:

```bash
# Recent logs
infracast logs --limit 10
```

**Expected output**:
```
Audit Logs (10 entries):

TIME                TRACE           LEVEL  ACTION     STEP          STATUS  ENV  DURATION  MESSAGE
----                -----           -----  ------     ----          ------  ---  --------  -------
2026-04-15 16:35    trc_17131...    INFO   deploy     health-check  ok      dev  3s        Health check passed
2026-04-15 16:35    trc_17131...    INFO   deploy     deploy        ok      dev  15s       K8s manifests applied
2026-04-15 16:34    trc_17131...    INFO   deploy     push          ok      dev  8s        Image pushed to ACR
2026-04-15 16:34    trc_17131...    INFO   deploy     build         ok      dev  12s       Docker image built
2026-04-15 16:30    trc_17130...    INFO   provision  -             ok      dev  180s      Provision complete

  ● 5 info
```

Show JSON output (for automation):

```bash
infracast logs --trace trc_xxx --format json | jq '.[].step'
```

---

## Step 7: Cleanup (1–2 min)

Always dry-run first:

```bash
infracast destroy --env dev --dry-run --keep-vpc 1
```

**Expected output**:
```
[DRY-RUN] Would delete:
  - RDS: pgm-bp1xxx
  - Redis: r-bp1xxx
  - VSwitch: vsw-bp1xxx (skipped, --keep-vpc)
  - VPC: vpc-bp1xxx (skipped, --keep-vpc)

No resources were deleted. Run with --apply to execute.
```

Apply when satisfied:

```bash
infracast destroy --env dev --apply --keep-vpc 1
```

Clean up the demo project:

```bash
cd ..
rm -rf demo-app
```

---

## Demo Talking Points

### What Infracast Does
- Deploys Encore applications to Alicloud with a single command
- Manages the full lifecycle: init → provision → deploy → verify → destroy
- Generates secure credentials, configures networking, handles health checks

### Key Features to Highlight
- **Trace ID**: Every deploy gets a trace ID for full pipeline visibility
- **Automatic rollback**: Failed health checks trigger rollback to previous version
- **Audit trail**: All operations logged with structured error codes
- **JSON output**: `--format json` for scripting and CI/CD integration
- **Dry-run**: Safe preview before any cloud operation

### Common Questions

| Question | Answer |
|----------|--------|
| How much does it cost? | ~¥20-25/day for a dev environment; destroy after testing |
| Which cloud providers? | Alicloud (single-cloud focus for reliability) |
| What if deploy fails? | Automatic rollback + trace ID for diagnosis |
| Can I use my own K8s cluster? | Yes, set KUBECONFIG to any ACK cluster |
| How do I add more environments? | `infracast env create staging --provider alicloud --region cn-shanghai` |

---

## Abbreviated Demo (5 min, dry-run only)

For demos without cloud access:

```bash
# 1. Init
infracast init demo-app --provider alicloud --region cn-hangzhou -y
cd demo-app

# 2. Show config
cat infracast.yaml

# 3. Show environments
infracast env list

# 4. Show audit logs (from previous runs)
infracast logs --limit 5

# 5. Show error code reference
head -30 /path/to/infracast/docs/error-code-matrix.md

# 6. Show version
infracast version
```
