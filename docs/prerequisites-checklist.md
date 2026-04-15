# Cost & Environment Prerequisites Checklist (Alicloud)

Pre-flight checklist before running `infracast provision` or `infracast deploy`.
Verify each item before your first deployment — and recheck after account changes.

---

## 1. Account Prerequisites

| # | Check | How to Verify | Fix if Failed |
|---|-------|---------------|---------------|
| 1 | Real-name verification completed | Alicloud Console → Account → Verification Status | Complete verification flow |
| 2 | Account balance ≥ ¥100 | Console → Billing → Account Overview | Top up via Console → Billing → Recharge |
| 3 | Pay-as-you-go credit limit sufficient | Console → Billing → Credit Limit | Request increase or pre-pay |
| 4 | RDS service activated | Console → RDS → check for "Activate" prompt | Click "Activate" or start Create Instance wizard |
| 5 | Redis (Kvstore) service activated | Console → Redis → check for "Activate" prompt | Click "Activate" |
| 6 | OSS service activated | Console → OSS → check for "Activate" prompt | Click "Activate" |
| 7 | ACK cluster running in target region | Console → Container Service → Clusters | Create ACK cluster (Managed, ≥1 node) |
| 8 | ACR (Container Registry) accessible | `docker login <acr-url>` succeeds | Enable ACR in Console, create namespace |

---

## 2. Credential Prerequisites

| # | Check | How to Verify | Fix if Failed |
|---|-------|---------------|---------------|
| 1 | `ALICLOUD_ACCESS_KEY` set | `echo $ALICLOUD_ACCESS_KEY` | `export ALICLOUD_ACCESS_KEY="your-key"` |
| 2 | `ALICLOUD_SECRET_KEY` set | `echo $ALICLOUD_SECRET_KEY` | `export ALICLOUD_SECRET_KEY="your-secret"` |
| 3 | `ALICLOUD_REGION` set | `echo $ALICLOUD_REGION` | `export ALICLOUD_REGION="cn-hangzhou"` |
| 4 | `KUBECONFIG` set and valid | `kubectl cluster-info` | `export KUBECONFIG=~/.kube/config` |
| 5 | Docker daemon running | `docker info` | Start Docker Desktop or `systemctl start docker` |

### RAM Permissions (Minimum)

The AccessKey must have these managed policies attached:

| Policy | Required For |
|--------|-------------|
| `AliyunRDSFullAccess` | RDS PostgreSQL provisioning |
| `AliyunKvstoreFullAccess` | Redis provisioning |
| `AliyunOSSFullAccess` | OSS bucket creation |
| `AliyunVPCFullAccess` | VPC/VSwitch networking |
| `AliyunCSFullAccess` | ACK cluster operations |
| `AliyunCRFullAccess` | ACR image push |

For production: replace managed policies with a custom least-privilege policy scoped to your target region.

---

## 3. Local Tool Prerequisites

| Tool | Version | Check | Install |
|------|---------|-------|---------|
| Go | ≥ 1.22 | `go version` | https://golang.org/dl/ |
| Encore CLI | latest | `encore version` | https://encore.dev/docs/install |
| Docker | running | `docker info` | https://docs.docker.com/get-docker/ |
| Infracast CLI | latest | `infracast version` | `go install github.com/DaviRain-Su/infracast/cmd/infracast@latest` |
| kubectl | ≥ 1.28 | `kubectl version --client` | https://kubernetes.io/docs/tasks/tools/ |

---

## 4. Quota & Resource Limits

Default Alicloud quotas that matter for Infracast:

| Resource | Default Quota | Required | Check |
|----------|---------------|----------|-------|
| VPC per region | 10 | ≥ 1 available | Console → VPC → VPC List |
| VSwitch per VPC | 24 | ≥ 1 available | Console → VPC → VSwitch |
| RDS instances per region | 30 | ≥ 1 available | Console → RDS → Instances |
| Redis instances per region | 20 | ≥ 1 available | Console → Redis → Instances |
| OSS buckets per account | 100 | ≥ 1 available | Console → OSS → Bucket List |

If quota is exhausted:
1. Clean up unused test resources first: `go run ./cmd/cleanup --region cn-hangzhou --prefix infracast --apply`
2. Request quota increase: Console → Quota Center

---

## 5. Cost Estimates

Approximate costs for a single `dev` environment (cn-hangzhou, pay-as-you-go):

| Resource | Spec | Approximate Cost |
|----------|------|-----------------|
| RDS PostgreSQL | pg.n2.medium.1, 20GB | ~¥8/day |
| Redis | redis.master.small.default | ~¥5/day |
| OSS | Standard, minimal usage | < ¥1/day |
| ACK Managed Cluster | Control plane free, worker nodes vary | ¥0 (control) + worker cost |
| ACK Worker Node | ecs.u1-c1m2.xlarge (4vCPU/8GiB) | ~¥5–10/day |
| **Total (dev)** | | **~¥20–25/day** |

Cost reduction tips:
- Run `infracast destroy --env dev --apply --keep-vpc 1` after testing
- Use `--keep-vpc 1` to avoid recreating networking (saves time and avoids quota pressure)
- Use smaller instance specs for testing (e.g., `pg.n2.small.1`)
- Set up daily budget alerts at ¥50/day

---

## 6. Pre-flight Script

Run this before your first deploy to verify all prerequisites:

```bash
#!/bin/bash
# Infracast pre-flight check

echo "=== Account & Credentials ==="
echo -n "ALICLOUD_ACCESS_KEY: "
[ -n "$ALICLOUD_ACCESS_KEY" ] && echo "SET ✓" || echo "MISSING ✗"
echo -n "ALICLOUD_SECRET_KEY: "
[ -n "$ALICLOUD_SECRET_KEY" ] && echo "SET ✓" || echo "MISSING ✗"
echo -n "ALICLOUD_REGION:     "
[ -n "$ALICLOUD_REGION" ] && echo "$ALICLOUD_REGION ✓" || echo "MISSING ✗"

echo ""
echo "=== Local Tools ==="
echo -n "Go:        "; go version 2>/dev/null || echo "NOT FOUND ✗"
echo -n "Encore:    "; encore version 2>/dev/null || echo "NOT FOUND ✗"
echo -n "Docker:    "; docker info >/dev/null 2>&1 && echo "RUNNING ✓" || echo "NOT RUNNING ✗"
echo -n "kubectl:   "; kubectl version --client --short 2>/dev/null || echo "NOT FOUND ✗"
echo -n "Infracast: "; infracast version 2>/dev/null || echo "NOT FOUND ✗"

echo ""
echo "=== Kubernetes ==="
echo -n "KUBECONFIG: "
[ -n "$KUBECONFIG" ] && echo "$KUBECONFIG ✓" || echo "MISSING ✗"
echo -n "Cluster:   "; kubectl cluster-info 2>/dev/null | head -1 || echo "NOT REACHABLE ✗"

echo ""
echo "=== Pre-flight complete ==="
```

---

## 7. Failure Triage Path

When a prerequisite check fails, follow this decision tree:

```
Pre-flight check failed
│
├─ Credential missing → Set environment variables (§2)
│
├─ Tool not found → Install from URLs in §3
│
├─ Docker not running → Start Docker daemon
│
├─ kubectl cannot reach cluster
│   ├─ KUBECONFIG not set → export KUBECONFIG=~/.kube/config
│   └─ Cluster unreachable → Check ACK cluster status in Console
│
├─ Alicloud API error
│   ├─ InvalidAccessKeyId → Verify key in RAM Console
│   ├─ Forbidden / NoPermission → Attach required policies (§2)
│   └─ ServiceLinkedRole.NotExist → Open RDS/OSS Console to trigger creation
│
└─ Quota exceeded → Clean up resources (§4) or request increase
```
