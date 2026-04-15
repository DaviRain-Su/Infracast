# Release Notes: v0.1.0

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
| `status` command is a stub | No `--output` flag | Use `kubectl get svc/pods` directly |
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
