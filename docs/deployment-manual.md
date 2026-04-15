# Single-Cloud Deployment Manual (Alicloud)

Complete command-flow reference for deploying an Encore application on Alicloud using Infracast.
Target audience: new developer, zero-to-deployed in 30 minutes.

---

## 1. Environment Prerequisites

### 1.1 Local Tools

| Tool | Version | Check | Install |
|------|---------|-------|---------|
| Go | ≥ 1.22 | `go version` | https://golang.org/dl/ |
| Encore CLI | latest | `encore version` | https://encore.dev/docs/install |
| Docker | running | `docker info` | https://docs.docker.com/get-docker/ |
| Infracast CLI | latest | `infracast version` | `go install github.com/DaviRain-Su/infracast/cmd/infracast@latest` |
| kubectl | ≥ 1.28 | `kubectl version --client` | https://kubernetes.io/docs/tasks/tools/ |

### 1.2 Alicloud Account

- Real-name verification completed
- RDS and OSS services enabled (open each console once to trigger ServiceLinkedRole creation)
- Sufficient balance for pay-as-you-go resources (RDS pg.n2.medium.1 + Redis ≈ ¥20/day)
- ACK (Container Service) cluster created and running in target region

### 1.3 Credentials

```bash
# Required — never commit to git
export ALICLOUD_ACCESS_KEY="your-access-key-id"
export ALICLOUD_SECRET_KEY="your-access-key-secret"
export ALICLOUD_REGION="cn-hangzhou"

# Kubernetes — point to your ACK cluster
export KUBECONFIG="$HOME/.kube/config"

# Optional — override RDS whitelist (default: VSwitch CIDR)
# export ALICLOUD_RDS_SECURITY_IP_LIST="10.0.0.0/24"
```

### 1.4 RAM Permissions (Minimum)

- `AliyunRDSFullAccess`
- `AliyunKvstoreFullAccess`
- `AliyunOSSFullAccess`
- `AliyunVPCFullAccess`
- `AliyunCSFullAccess` (for ACK operations)
- `AliyunCRFullAccess` (for container registry push)

For production, replace managed policies with a custom least-privilege policy scoped to your target region/resources.

---

## 2. Command Flow: init → env → provision → deploy → logs → destroy

### Step 1: Initialize Project

```bash
infracast init my-app --provider alicloud --region cn-hangzhou -y
cd my-app
```

**Expected output (success):**
```
✓ Created infracast.yaml
✓ Created .infra/ directory
✓ Created .gitignore
✓ Project my-app initialized for alicloud (cn-hangzhou)
```

**Expected output (failure — directory exists):**
```
ECFG001: failed to load config: directory my-app already exists
```

### Step 2: Create Environment

```bash
infracast env create dev --provider alicloud --region cn-hangzhou
infracast env list
```

**Expected output (success):**
```
Environment dev created.

ENVIRONMENT  PROVIDER  REGION       CURRENT
dev          alicloud  cn-hangzhou  →
```

### Step 3: Provision Infrastructure

```bash
infracast provision --env dev
```

Creates VPC, VSwitch, RDS PostgreSQL, Redis, and generates `infracfg.json`.

**Expected output (success):**
```
✓ VPC created (vpc-bp1xxx)
✓ VSwitch created (vsw-bp1xxx)
✓ RDS PostgreSQL created (pgm-bp1xxx)
✓ Redis created (r-bp1xxx)
✓ Generated infracfg.json

Provision complete. Resources ready in env dev.
```

**Expected output (failure — insufficient balance):**
```
EPROV003: InvalidAccountStatus.NotEnoughBalance
  Hint: Top up your Alicloud account or use smaller instance specs.
```

### Step 4: Deploy Application

```bash
infracast deploy --env dev
```

Builds Docker image, pushes to ACR, deploys to ACK, verifies health checks.

**Expected output (success):**
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

**Expected output (failure — health check timeout):**
```
  Step           Status  Duration
  ─────────────  ──────  ────────
  build          OK      12s
  push           OK      8s
  deploy         OK      15s
  health-check   FAIL    30s

  Hint: Application did not pass health check within timeout.
        Check application logs: infracast logs --trace trc_xxx
  Error: EDEPLOY050: deployment timeout after 30s

  ✗ Deploy failed (3/4 steps passed, 65s total)
```

### Step 5: Verify & Observe

```bash
# View deployment status
infracast status --env dev

# View recent audit logs
infracast logs --limit 10

# Trace a specific deploy run
infracast logs --trace trc_1713184200000000000

# View only errors from last hour
infracast logs --level ERROR --since 1h
```

### Step 6: Destroy (Cleanup)

```bash
# Dry-run first — always
infracast destroy --env dev --dry-run --keep-vpc 1

# Apply when satisfied
infracast destroy --env dev --apply --keep-vpc 1
```

**Expected output (dry-run):**
```
[DRY-RUN] Would delete:
  - RDS: pgm-bp1xxx
  - Redis: r-bp1xxx
  - VSwitch: vsw-bp1xxx (skipped, --keep-vpc)
  - VPC: vpc-bp1xxx (skipped, --keep-vpc)

No resources were deleted. Run with --apply to execute.
```

---

## 3. Failure Decision Tree

When a deploy or provision fails, follow this path:

```
Deploy/Provision failed
│
├─ Get trace ID from output banner
│   └─ infracast logs --trace trc_xxx
│
├─ Identify the failing step (build / push / provision / deploy / health-check)
│
├─ Check error code:
│   │
│   ├─ ECFG001 — Missing provider / ECFG019 — Environment not found
│   │   └─ Fix: Check infracast.yaml syntax, run `infracast env list`
│   │
│   ├─ EDEPLOY001 — Invalid environment
│   │   └─ Fix: Verify --env flag matches `infracast env list`
│   │
│   ├─ EDEPLOY050 — Deployment/health check timeout
│   │   └─ Fix: Check app logs (`kubectl logs`), verify /health endpoint works locally
│   │
│   ├─ EPROV003 / NotEnoughBalance — Billing
│   │   └─ Fix: Top up account, or use smaller instance specs
│   │
│   ├─ Docker build fails
│   │   └─ Fix: `docker info` → verify daemon running, check Dockerfile
│   │
│   ├─ Registry push fails / unauthorized
│   │   └─ Fix: `docker login <registry-url>`, check ACR credentials
│   │
│   ├─ KUBECONFIG not set
│   │   └─ Fix: `export KUBECONFIG=~/.kube/config`, verify ACK cluster
│   │
│   ├─ Timeout (generic)
│   │   └─ Fix: Check network, retry with `--verbose`
│   │
│   └─ Unknown error
│       └─ Fix: Copy request_id from logs, check Alicloud console
│
└─ After fix, re-run the same command
```

### Using Trace ID for Deep Diagnosis

```bash
# 1. Find recent errors
infracast logs --level ERROR --since 1h

# 2. Get the trace ID from the output, then view full pipeline
infracast logs --trace trc_17131...

# 3. Output shows all steps with status:
# TIME              TRACE         LEVEL  ACTION  STEP        STATUS  ENV  DURATION  MESSAGE
# 2026-04-15 16:30  trc_17131...  INFO   deploy  build       ok      dev  12s       Docker image built
# 2026-04-15 16:30  trc_17131...  INFO   deploy  push        ok      dev  8s        Image pushed
# 2026-04-15 16:30  trc_17131...  ERROR  deploy  provision   fail    dev  5s        EPROV003: NotEnoughBalance
#
#   Error in [deploy/provision]:
#     Code:       EPROV003
#     Request ID: 7B3A4C2D-...
#     Message:    InvalidAccountStatus.NotEnoughBalance

# 4. Use error code + request ID to look up in Alicloud console
```

---

## 4. Regression Command Set (Single-Cloud)

A copy-paste sequence for validating the full pipeline. Run after code changes to confirm nothing is broken.

### 4.1 Success Path

```bash
# Init
infracast init regression-test --provider alicloud --region cn-hangzhou -y
cd regression-test

# Copy health-check example
cp -r /path/to/infracast/examples/health-check/* .

# Create environment
infracast env create dev --provider alicloud --region cn-hangzhou

# Provision (creates cloud resources — costs money)
infracast provision --env dev

# Deploy
infracast deploy --env dev

# Verify health
curl -s "$(infracast status --env dev --output url)/livez" | jq .
# Expected: {"status":"ok","uptime":"..."}

curl -s "$(infracast status --env dev --output url)/readyz" | jq .
# Expected: {"status":"ready","checks":{"self":"ok"},...}

# View audit trail
infracast logs --limit 5
# Expected: recent deploy steps with status=ok

# Cleanup
infracast destroy --env dev --apply --keep-vpc 1
```

### 4.2 Failure Path (Simulated)

```bash
# Deploy with failure simulation
SIMULATE_FAILURE=true infracast deploy --env dev

# Verify readiness probe reports unhealthy
curl -s "$(infracast status --env dev --output url)/readyz" | jq .
# Expected: {"status":"unhealthy","checks":{"self":"fail"},...}

# Check error audit trail
infracast logs --level ERROR --since 10m
# Expected: health-check step with status=fail
```

### 4.3 Smoke Test (No Cloud Resources)

For local validation without spending money:

```bash
# Build only — verifies Go compilation and Docker image
infracast deploy --env dev --dry-run

# Verify CLI tools
infracast version
infracast env list
infracast logs --limit 1
```

---

## 5. Quick Reference

| Task | Command |
|------|---------|
| Initialize | `infracast init my-app --provider alicloud --region cn-hangzhou -y` |
| Create env | `infracast env create dev --provider alicloud --region cn-hangzhou` |
| Switch env | `infracast env use staging` |
| List envs | `infracast env list` |
| Provision | `infracast provision --env dev` |
| Deploy | `infracast deploy --env dev` |
| Status | `infracast status --env dev` |
| Logs (recent) | `infracast logs --limit 20` |
| Logs (errors) | `infracast logs --level ERROR --since 1h` |
| Logs (trace) | `infracast logs --trace trc_xxx` |
| Destroy (dry) | `infracast destroy --env dev --dry-run --keep-vpc 1` |
| Destroy (apply) | `infracast destroy --env dev --apply --keep-vpc 1` |
