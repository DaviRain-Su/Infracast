# Single-Cloud Operations Runbook (Alicloud)

Actionable procedures for operating Infracast on Alicloud.
For architecture and config details, see [Single-Cloud Operations Handbook](06-single-cloud-operations.md).
For error codes, see [Error Code Matrix](error-code-matrix.md).

---

## 1. Alerting Setup

### 1.1 Alicloud Budget Alerts

Set up in **Alicloud Console → Billing Management → Budget**:

| Alert | Threshold | Action |
|-------|-----------|--------|
| Daily spend | ¥50 | Email + SMS notification |
| Monthly spend | ¥500 | Email + SMS notification |
| Balance low | ¥100 remaining | Email + SMS + pause non-critical deploys |

### 1.2 Resource Count Alerts

Monitor via Alicloud CloudMonitor or manual check:

```bash
# Quick resource count check
infracast status --env dev
```

Alert conditions:
- RDS instances with prefix `infracast-*` > 3 → likely leaked test resources
- Redis instances with prefix `infracast-*` > 3 → same
- VPC count > 5 → quota pressure; run cleanup

### 1.3 Deploy Failure Alerts

Infracast logs all operations to the audit database. Monitor for failures:

```bash
# Check for errors in last hour
infracast logs --level ERROR --since 1h

# Check for errors in last 24 hours (JSON for automation)
infracast logs --level ERROR --since 24h --format json
```

If using Feishu/DingTalk notifications, deploy failures are sent automatically when configured in `infracast.yaml`:

```yaml
notifications:
  feishu:
    webhook_url: "https://open.feishu.cn/..."
```

---

## 2. Rollback Procedures

### 2.1 Automatic Rollback (Deploy Pipeline)

Infracast automatically triggers rollback when health checks fail during deploy:

1. Deploy detects pod unhealthy (EDEPLOY050)
2. Checks if safe to rollback (previous revision exists, no destructive migrations)
3. Executes `kubectl rollout undo`
4. Verifies rollback stabilizes

Check rollback status:

```bash
# View the deploy trace to see if rollback triggered
infracast logs --trace <trace_id>

# Manual check via kubectl
kubectl rollout status deployment/<app-name> -n <namespace>
kubectl rollout history deployment/<app-name> -n <namespace>
```

### 2.2 Manual Rollback

When automatic rollback fails or wasn't triggered:

```bash
# Step 1: Check current status
kubectl get pods -n <namespace>
kubectl describe deployment <app-name> -n <namespace>

# Step 2: Roll back to previous revision
kubectl rollout undo deployment/<app-name> -n <namespace>

# Step 3: Verify
kubectl rollout status deployment/<app-name> -n <namespace>

# Step 4: Check health endpoint
curl -s http://<service-ip>/livez | jq .
curl -s http://<service-ip>/readyz | jq .
```

### 2.3 Rollback Blocked: Destructive Migrations (EDEPLOY068)

If the deploy includes irreversible database migrations, rollback is blocked. In this case:

1. **Do NOT** run `kubectl rollout undo` — DB schema won't match old code
2. Fix the application code and redeploy forward
3. If DB is corrupted, restore from backup (see §4.3)

---

## 3. Cleanup Procedures

### 3.1 Standard Cleanup (Per Environment)

```bash
# Always dry-run first
infracast destroy --env dev --dry-run --keep-vpc 1

# Review output, then apply
infracast destroy --env dev --apply --keep-vpc 1
```

### 3.2 Bulk Cleanup (All Test Resources)

When multiple test environments have leaked resources:

```bash
# Dry-run with prefix filter
go run ./cmd/cleanup --region cn-hangzhou --prefix infracast

# Apply when satisfied
go run ./cmd/cleanup --region cn-hangzhou --prefix infracast --apply
```

### 3.3 Handling Dependency Errors During Cleanup

Common errors: `DependencyViolation.VSwitch`, `DependencyViolation.Kvstore`, `DependencyViolation.NetworkInterface`

These are **expected** — cloud resources release asynchronously. Procedure:

1. Confirm RDS/Redis deletion was accepted (check Alicloud console)
2. Wait 3–10 minutes for async release
3. Rerun the destroy/cleanup command
4. If still failing after 15 minutes, check Alicloud console for stuck resources and delete manually

### 3.4 Weekly Hygiene

Run every Monday (or after any test sprint):

```bash
# 1. Check for leaked resources
go run ./cmd/cleanup --region cn-hangzhou --prefix infracast

# 2. Review and apply if needed
go run ./cmd/cleanup --region cn-hangzhou --prefix infracast --apply

# 3. Verify quota headroom
# Check VPC count in Alicloud console (limit: 10 per region default)

# 4. Review audit trail for anomalies
infracast logs --since 7d --level WARN
infracast logs --since 7d --level ERROR
```

---

## 4. Incident Response: Common Failures

### 4.1 NotEnoughBalance / InvalidAccountStatus.NotEnoughBalance

**Symptom**: Provision or deploy fails with EPROV003 or ACK node pool scaling fails.

**Diagnosis**:
```bash
infracast logs --level ERROR --since 1h
# Look for: EPROV003, NotEnoughBalance
```

**Fix**:
1. Check Alicloud billing: **Console → Billing Management → Account Overview**
2. For pay-as-you-go: check both cash balance AND credit limit
3. ACK node pools may need ¥100+ balance even for small instances
4. Top up account, then retry:
   ```bash
   infracast provision --env dev
   # or
   infracast deploy --env dev
   ```

### 4.2 ServiceLinkedRole.NotExist

**Symptom**: First-time provisioning fails for RDS or OSS.

**Diagnosis**:
```bash
infracast logs --level ERROR --since 1h
# Look for: ServiceLinkedRole
```

**Fix**:
1. Open **Alicloud RDS Console** → click "Create Instance" (don't need to purchase)
2. This triggers automatic ServiceLinkedRole creation
3. For OSS: open **OSS Console** → click "Activate" if prompted
4. Retry provision after 1 minute

### 4.3 Deploy Timeout (EDEPLOY050)

**Symptom**: Health check fails after deploy, pods not ready.

**Diagnosis**:
```bash
# Get the trace
infracast logs --level ERROR --since 1h

# Check pod status
kubectl get pods -n <namespace>
kubectl describe pod <pod-name> -n <namespace>

# Check application logs
kubectl logs <pod-name> -n <namespace>
```

**Common causes and fixes**:

| Pod Status | Likely Cause | Fix |
|------------|-------------|-----|
| Pending | No nodes available (NotEnoughBalance) | Top up account, check node pool |
| CrashLoopBackOff | App crash on startup | Check logs, fix application code |
| ImagePullBackOff | ACR auth or image not found | `docker login <acr-url>`, verify image pushed |
| Running but not Ready | Health endpoint failing | Check `/livez` and `/readyz` implementation |

### 4.4 ACR Push Failure (EDEPLOY040–047)

**Symptom**: Image push to Alicloud Container Registry fails.

**Diagnosis**:
```bash
infracast logs --level ERROR --since 1h
# Look for: EDEPLOY040-047
```

**Fix**:
```bash
# Re-authenticate
docker login --username=<access-key-id> <acr-registry-url>

# Verify image exists locally
docker images | grep <app-name>

# Manual push test
docker push <acr-registry-url>/<namespace>/<image>:<tag>
```

### 4.5 KUBECONFIG / Cluster Connectivity (EDEPLOY011)

**Symptom**: K8s operations fail with client init error.

**Fix**:
```bash
# Verify KUBECONFIG
echo $KUBECONFIG
kubectl cluster-info

# If using ACK, re-fetch kubeconfig
aliyun cs GET /k8s/<cluster-id>/user_config | jq -r '.config' > ~/.kube/config
export KUBECONFIG=~/.kube/config

# Test connectivity
kubectl get nodes
```

### 4.6 RDS/Redis Stuck in Transition (IncorrectDBInstanceState)

**Symptom**: Destroy or modify operations fail because instance is in transition.

**Fix**:
1. Check instance status in Alicloud console
2. Wait for current operation to complete (usually 5–15 minutes)
3. Retry the operation
4. If stuck for >30 minutes, file Alicloud support ticket with RequestID

---

## 5. Escalation Path

When self-service fixes don't resolve the issue:

1. **Collect evidence**:
   ```bash
   infracast logs --trace <trace_id> --format json > /tmp/trace-dump.json
   ```

2. **Check Alicloud-side**:
   - Copy the `Request ID` from error output
   - Search in **Alicloud Console → ActionTrail** for the request

3. **File support ticket** with:
   - Infracast trace dump (JSON)
   - Alicloud Request ID
   - Region and resource IDs
   - Steps to reproduce

---

## 6. Quick Reference

| Scenario | Command |
|----------|---------|
| Recent errors | `infracast logs --level ERROR --since 1h` |
| Trace a deploy | `infracast logs --trace trc_xxx` |
| Full trace (JSON) | `infracast logs --trace trc_xxx --format json` |
| Wide output | `infracast logs --output wide` |
| Pod status | `kubectl get pods -n <namespace>` |
| Pod logs | `kubectl logs <pod> -n <namespace>` |
| Manual rollback | `kubectl rollout undo deployment/<app> -n <ns>` |
| Destroy (dry-run) | `infracast destroy --env dev --dry-run --keep-vpc 1` |
| Destroy (apply) | `infracast destroy --env dev --apply --keep-vpc 1` |
| Bulk cleanup | `go run ./cmd/cleanup --region cn-hangzhou --prefix infracast --apply` |
| Check health | `curl -s http://<ip>/livez \| jq .` |
