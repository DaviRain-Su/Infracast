# Getting Started with Infracast

This guide will walk you through deploying your first Encore application using Infracast.

## Prerequisites

- [Go](https://golang.org/dl/) 1.22 or later
- [Encore CLI](https://encore.dev/docs/install)
- [Infracast CLI](../README.md#installation)
- Alibaba Cloud account with access credentials

## Credential and Security Baseline (AliCloud)

Set credentials in your shell (never commit them to git):

```bash
export ALICLOUD_ACCESS_KEY="your-access-key-id"
export ALICLOUD_SECRET_KEY="your-access-key-secret"
export ALICLOUD_REGION="cn-hangzhou"

# Optional: override RDS whitelist explicitly.
# Default behavior (recommended): use current VSwitch CIDR automatically.
export ALICLOUD_RDS_SECURITY_IP_LIST="10.0.0.0/24"
```

Recommended minimum RAM permissions for single-cloud flow:
- `AliyunRDSFullAccess`
- `AliyunKvstoreFullAccess`
- `AliyunOSSFullAccess`
- `AliyunVPCFullAccess`
- `AliyunSTSAssumeRoleAccess` (only when using STS mode)

For production, replace broad managed policies with a custom least-privilege policy
scoped to your target region/resources.

## 5-Step Quickstart

### Step 1: Initialize Your Project

Create a new Infracast project:

```bash
infracast init my-app --provider alicloud --region cn-hangzhou
```

This creates:
- `infracast.yaml` - Project configuration
- `.infra/` - Infrastructure state directory
- `.gitignore` - Git ignore rules
- `README.md` - Project documentation

### Step 2: Configure Resources

Edit `infracast.yaml` to define your infrastructure:

```yaml
app_name: my-app
provider: alicloud
region: cn-hangzhou

environments:
  dev:
    description: Development environment

resources:
  sql_servers:
    main:
      instance_class: pg.n2.medium.1
      storage: 20
  
  redis:
    cache:
      node_type: redis.master.small.default
```

### Step 3: Provision Infrastructure

Create the cloud resources:

```bash
infracast provision --env dev
```

This will:
- Create VPC and networking
- Provision RDS PostgreSQL instance
- Create Redis cache cluster
- Generate `infracfg.json` with connection details

Security notes:
- Database and Redis passwords are generated with cryptographically secure randomness.
- RDS whitelist defaults to VSwitch CIDR (private network), not `127.0.0.1` and not `0.0.0.0/0`.

### Step 4: Deploy Your Application

Build and deploy your Encore app:

```bash
infracast deploy --env dev
```

The deployment process:
1. Builds Docker image (`encore build docker`)
2. Pushes to container registry
3. Deploys to Kubernetes
4. Verifies health checks

### Step 5: Verify Deployment

Check your deployment status:

```bash
# View status
infracast status --env dev

# View logs
infracast logs --env dev

# Open application URL
infracast open --env dev
```

## Next Steps

### Add More Environments

```bash
# Create staging environment
infracast env create staging --provider alicloud --region cn-shanghai

# Deploy to staging
infracast deploy --env staging
```

### Configure Notifications

Add webhook notifications for deployments:

```yaml
# infracast.yaml
notifications:
  feishu:
    webhook_url: "https://open.feishu.cn/..."
```

### Learn More

- [Technical Spec](03-technical-spec.md)
- [Task Breakdown](04-task-breakdown.md)
- [Single-Cloud Operations Handbook](06-single-cloud-operations.md)

## Quick Command Flow

The shortest path from zero to deployed:

```bash
infracast init my-app --provider alicloud --region cn-hangzhou -y
cd my-app
# edit infracast.yaml → uncomment the resources you need
infracast provision --env dev
infracast deploy --env dev
infracast status --env dev
```

## Common Errors & Next Steps

| Error | Cause | Fix |
|-------|-------|-----|
| `ECFG001: failed to load config` | Missing or invalid `infracast.yaml` | Run `infracast init` or check YAML syntax |
| `ECFG002: environment not found` | Environment name doesn't exist | Run `infracast env list` to see available envs |
| `EDEPLOY001: invalid environment` | Typo in `--env` flag | Valid values: `dev`, `staging`, `production`, `local` |
| `NotEnoughBalance` | Cloud account balance too low for node provisioning | Top up account or use spot instances |
| `KUBECONFIG` not set | Missing Kubernetes config | `export KUBECONFIG=~/.kube/config` |
| Docker build fails | Docker daemon not running | Run `docker info` to verify, then `docker start` |
| Registry push fails | Invalid registry credentials | Re-authenticate: `docker login <registry-url>` |
| Deploy timeout | Network or cluster issue | Check connectivity; retry with `--verbose` for details |

## Troubleshooting a Failed Deploy with Trace ID

Every deploy/provision run generates a `trace_id` that links all steps in that pipeline. When a deploy fails, use the trace ID to see the full timeline:

**Step 1**: Find the failed deploy trace

```bash
infracast logs --level ERROR --since 1h
```

Output shows:
```
TIME              TRACE         LEVEL  ACTION  STEP        STATUS  ENV  DURATION  MESSAGE
2026-04-15 16:30  trc_17131...  ERROR  deploy  provision   fail    dev  5s        EPROV003: NotEnoughBalance...
```

**Step 2**: View all steps in that trace

```bash
infracast logs --trace trc_17131...
```

Output shows the full pipeline:
```
TIME              TRACE         LEVEL  ACTION  STEP        STATUS  ENV  DURATION  MESSAGE
2026-04-15 16:30  trc_17131...  INFO   deploy  build       ok      dev  12s       Docker image built
2026-04-15 16:30  trc_17131...  INFO   deploy  push        ok      dev  8s        Image pushed to registry
2026-04-15 16:30  trc_17131...  ERROR  deploy  provision   fail    dev  5s        EPROV003: NotEnoughBalance...

  Error in [deploy/provision]:
    Code:       EPROV003
    Request ID: 7B3A4C2D-...
    Message:    InvalidAccountStatus.NotEnoughBalance
```

**Step 3**: Act on the error

The error code (`EPROV003`) and provider request ID let you:
- Look up the exact cloud API call that failed
- Cross-reference with your provider's console/support
- See the "Common Errors" table above for suggested fixes

## Example Applications

Check out the [examples](../examples/) directory for complete sample applications:

- [hello-world](../examples/hello-world/) - Minimal example
- [todo-app](../examples/todo-app/) - Todo app with database
- [blog-api](../examples/blog-api/) - Blog API with OSS uploads
