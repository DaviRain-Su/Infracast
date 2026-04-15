# Release Notes — Infracast vX.Y.Z

> **Date:** YYYY-MM-DD
> **Commit:** `<short-sha>`
> **Platforms:** darwin/amd64, darwin/arm64, linux/amd64

---

## What's New

- [ ] Feature or improvement (1-line summary)
- [ ] Feature or improvement (1-line summary)

## Bug Fixes

- [ ] Fix description (reference error code if applicable, e.g. EDEPLOY050)

## Breaking Changes

- None / list any breaking changes here

## Compatibility

| Component | Minimum Version |
|-----------|----------------|
| Go | 1.22+ |
| Encore CLI | latest |
| Docker | 20.10+ |
| kubectl | 1.28+ |
| Alicloud Provider | Tested with cn-hangzhou |

## Known Environment Gates

These are infrastructure prerequisites, not code defects:

- **ACK Cluster**: A running Alibaba Container Service (ACK) cluster is required for deploy operations. If the cluster is not provisioned or the account has insufficient balance, deploy will fail with `EPROV003` or `NotEnoughBalance`.
- **KUBECONFIG**: Must be set and pointing to a valid ACK cluster. Missing config will produce `EDEPLOY011`.
- **ACR Credentials**: Container registry authentication must be configured for image push. Failure produces `EDEPLOY072`.
- **Billing**: Pay-as-you-go resources (RDS, Redis, ACK nodes) require sufficient account balance.

## How to Install

### Download Binary

```bash
# Example for darwin/arm64 (Apple Silicon)
curl -LO https://github.com/DaviRain-Su/Infracast/releases/download/vX.Y.Z/infracast_vX.Y.Z_darwin_arm64.tar.gz
curl -LO https://github.com/DaviRain-Su/Infracast/releases/download/vX.Y.Z/checksums.txt

# Verify checksum
shasum -a 256 -c checksums.txt --ignore-missing

# Extract and install
tar xzf infracast_vX.Y.Z_darwin_arm64.tar.gz
sudo mv infracast_vX.Y.Z_darwin_arm64/infracast /usr/local/bin/
```

### Verify

```bash
infracast version
# Expected:
# infracast version vX.Y.Z
#   commit: <short-sha>
#   build time: YYYY-MM-DD_HH:MM:SS
```

### Build from Source

```bash
git clone https://github.com/DaviRain-Su/Infracast.git
cd Infracast
make build
./bin/infracast version
```

## Rollback

If this release causes issues:

1. Download the previous release from [Releases](https://github.com/DaviRain-Su/Infracast/releases)
2. Replace the binary: `sudo mv infracast /usr/local/bin/`
3. Verify: `infracast version` shows the previous version
4. State database (`.infra/state.db`) is forward-compatible; no rollback needed for data

## Full Changelog

`git log <prev-tag>..vX.Y.Z --oneline`
