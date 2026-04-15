# Single-Cloud Operations Handbook (AliCloud)

This handbook is the operational baseline for Alicloud single-cloud delivery.
It covers deployment, cleanup, troubleshooting, and cost controls.

## 1. Deployment Manual

### 1.1 Preconditions

- Account:
  - Real-name verification completed.
  - RDS and OSS services enabled.
  - Sufficient balance for pay-as-you-go resources.
- Credentials:
  - `ALICLOUD_ACCESS_KEY`
  - `ALICLOUD_SECRET_KEY`
  - Optional: `ALICLOUD_REGION` (default `cn-hangzhou`)
- RAM permissions (minimum baseline):
  - `AliyunRDSFullAccess`
  - `AliyunKvstoreFullAccess`
  - `AliyunOSSFullAccess`
  - `AliyunVPCFullAccess`

### 1.2 Recommended shell setup

```bash
set -a
source .env
set +a
```

### 1.3 Provision-only validation (core infra)

Current state: dedicated `provision-verify` helper is planned, not yet implemented as a stable command.

Use this validation path for now:

```bash
infracast provision --env dev --region cn-hangzhou --dry-run
```

Expected pass signal:
- No validation errors from config/provider checks.

### 1.4 Full deployment chain (when Encore/ACK is ready)

Current state: full E2E command is planned and depends on complete pipeline prerequisites.

Planned command:

```bash
E2E_FULL=1 go test ./e2e/ -run TestFullE2EDeployment -v
```

Notes:
- This creates real cloud resources and incurs cost.
- If Encore CLI or ACK prerequisites are missing, run provision-only first.

## 2. Cleanup Manual

### 2.1 Primary path: formal destroy command

Dry-run first:

```bash
infracast destroy --env dev --region cn-hangzhou --dry-run --keep-vpc 1
infracast destroy --env dev --region cn-hangzhou --prefix infracast-dev --dry-run --keep-vpc 1
```

Apply:

```bash
infracast destroy --env dev --region cn-hangzhou --apply --keep-vpc 1
```

Safety:
- Without `--apply`, command stays dry-run.
- Broad prefixes (for example `infracast`) require `--force` for real deletion.

### 2.2 Bulk fallback cleanup (legacy helper)

Use only when you need prefix-wide cleanup:

```bash
go run ./cmd/cleanup --region cn-hangzhou --prefix infracast
go run ./cmd/cleanup --region cn-hangzhou --prefix infracast --apply
```

### 2.3 Expected async behavior

After RDS/Redis deletion, VSwitch/VPC often fail temporarily with dependency errors.
This is normal cloud-side async release. Wait 3-10 minutes and rerun cleanup/destroy.

## 3. Troubleshooting Runbook

### 3.1 Account and service readiness

- `ServiceLinkedRole.NotExist`
  - Trigger creation by opening RDS console and going through Create Instance flow (no purchase needed).
- `CommodityServiceCalling.Exception`
  - Usually account-side: incomplete verification, unpaid status, or service not enabled.
- `UserDisable` (OSS)
  - Enable OSS service and verify RAM permission for OSS.

### 3.2 Network and quota

- `QuotaExceeded.Vpc`
  - Cleanup old test VPC/VSwitch resources, or request quota increase.
- `IncorrectVpcStatus`
  - Caused by creating dependent resources before VPC is available.
  - Ensure VPC wait/poll logic is active before VSwitch creation.
- `InvalidvSwitchId ... zone not supported`
  - Redis/RDS zone mismatch; use zone fallback or supported-zone probing.

### 3.3 Deletion dependency errors

- `DependencyViolation.Kvstore`
- `DependencyViolation.NetworkInterface`
- `DependencyViolation.VSwitch`

Action:
1. Confirm RDS/Redis deletion requests were accepted.
2. Wait for async release window.
3. Rerun destroy/cleanup.

### 3.4 RDS transient deletion states

- `IncorrectDBInstanceState`
  - Instance is in transition (creating/modifying/deleting).
  - Retry later; treat as transient, not fatal configuration issue.

## 4. Cost and Alert Recommendations

### 4.1 Cost controls

- Keep `--keep-vpc 1` to reduce repeated network creation.
- Run cleanup immediately after tests.
- Prefer provision-only validation during fix sprints; run full E2E only when needed.
- Tag all test resources with a common prefix (`infracast-*`) for fast cleanup.

### 4.2 Suggested operational alerts

- Budget alert:
  - Monthly and daily thresholds for test account spend.
- Resource count alert:
  - Number of RDS/Redis/VPC resources with prefix `infracast-*`.
- Long-lived resource alert:
  - Any test resource older than 24h.
- Cleanup failure alert:
  - Repeated dependency errors beyond retry window.

### 4.3 Weekly hygiene checklist

1. Dry-run destroy for each active environment.
2. Remove stale test resources.
3. Verify quota headroom (VPC, RDS, Redis).
4. Review RAM access keys and rotate if needed.
