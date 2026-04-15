# Pre-release Checklist: v0.1.0-rc1

Verify each item before tagging the release. All items must be checked.

---

## 1. CI & Build

- [ ] `gofmt -l .` returns empty (no formatting issues)
- [ ] `go vet ./...` passes with no warnings
- [ ] `go test -race ./...` passes (all tests green)
- [ ] `go build ./cmd/infracast/` succeeds
- [ ] `make release-build` produces 3 platform archives:
  - [ ] `infracast_<ver>_darwin_amd64.tar.gz`
  - [ ] `infracast_<ver>_darwin_arm64.tar.gz`
  - [ ] `infracast_<ver>_linux_amd64.tar.gz`
- [ ] `make release-checksums` generates valid `checksums.txt`
- [ ] GitHub Actions CI workflow passes on main

## 2. Version & Metadata

- [ ] `infracast version` outputs correct version, commit, build time
- [ ] `cmd/infracast/main.go` ldflags variables are set (`Version`, `Commit`, `BuildTime`)
- [ ] Git tag matches release version (e.g., `v0.1.0-rc1`)

## 3. Documentation Completeness

- [ ] `README.md` — updated for single-cloud scope, links to all docs
- [ ] `docs/getting-started.md` — 5-step quickstart, error table, trace ID walkthrough
- [ ] `docs/deployment-manual.md` — full command flow, failure decision tree, download/verify
- [ ] `docs/error-code-matrix.md` — 78 codes, Source column, cross-references
- [ ] `docs/runbook.md` — alerting, rollback, cleanup, 6 incident scenarios
- [ ] `docs/prerequisites-checklist.md` — account/credential/tool/quota/cost checks
- [ ] `docs/demo-script.md` — 8-step demo, dry-run variant, talking points
- [ ] `docs/release-notes-template.md` — template exists
- [ ] `examples/README.md` — lists all 6 examples

## 4. Documentation Cross-links

- [ ] `getting-started.md` links to `error-code-matrix.md`
- [ ] `deployment-manual.md` links to `error-code-matrix.md`
- [ ] `error-code-matrix.md` links to `06-single-cloud-operations.md`
- [ ] `runbook.md` links to `error-code-matrix.md` and `06-single-cloud-operations.md`
- [ ] `demo-script.md` links to `prerequisites-checklist.md` and `error-code-matrix.md`
- [ ] `README.md` links to `getting-started.md`, `runbook.md`, `prerequisites-checklist.md`

## 5. Single-Cloud Scope Compliance

- [ ] No references to Huawei Cloud, Tencent Cloud, or Volcengine in user-facing docs
- [ ] README reflects single-cloud (Alicloud) focus
- [ ] No multi-cloud provider code paths exposed in CLI
- [ ] `infracast.yaml` examples use `provider: alicloud` only

## 6. Known Issues & Limitations

- [ ] ACK Verify deferred (account/billing gate) — documented
- [ ] Multi-cloud frozen (single-cloud only for v0.1.0) — documented
- [ ] `infracast status` is a stub (no `--output` flag) — documented
- [ ] Full E2E deploy requires ACK cluster + sufficient balance — documented

## 7. Release Artifacts

- [ ] Tag: `v0.1.0-rc1`
- [ ] 3 platform archives built and checksummed
- [ ] `RELEASE-NOTES.md` written (features, limitations, requirements)
- [ ] GitHub Release created (if applicable)

---

## Verification Commands

```bash
# CI checks
gofmt -l .
go vet ./...
go test -race ./...
go build ./cmd/infracast/

# Release build
make release

# Version check
./bin/infracast version

# Doc link check (manual: open each .md, verify links resolve)
```
