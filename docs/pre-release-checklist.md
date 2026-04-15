# Pre-release Checklist: v0.1.0-rc1

Verify each item before tagging the release. All items must be checked with evidence.

---

## 1. CI & Build

Evidence: `c21b453` on main, 2026-04-15

- [x] `gofmt -l .` returns empty — verified, no output
- [x] `go vet ./...` passes — verified, no warnings
- [x] `go test -race ./...` passes — verified, 16 packages pass, 0 failures
- [x] `go build ./cmd/infracast/` succeeds — verified, binary produced
- [ ] `make release-build` produces 3 platform archives:
  - [ ] `infracast_<ver>_darwin_amd64.tar.gz`
  - [ ] `infracast_<ver>_darwin_arm64.tar.gz`
  - [ ] `infracast_<ver>_linux_amd64.tar.gz`
- [ ] `make release-checksums` generates valid `checksums.txt`
- [ ] GitHub Actions CI workflow passes on main

## 2. Dependencies & Version Lock

Evidence: `go mod verify` → "all modules verified"

- [x] `go.mod` specifies Go 1.22+ toolchain
- [x] `go.sum` is committed and in sync (`go mod verify` passes)
- [x] No floating/unversioned dependencies in `go.mod`
- [ ] All direct dependencies reviewed for known vulnerabilities (`govulncheck ./...` if available)

## 3. Version & Metadata

- [ ] `infracast version` outputs correct version, commit, build time
- [ ] `cmd/infracast/main.go` ldflags variables are set (`Version`, `Commit`, `BuildTime`)
- [ ] Git tag matches release version (`v0.1.0-rc1`)

## 4. CHANGELOG

- [ ] `CHANGELOG.md` exists in project root
- [ ] Covers all changes since last release (or initial release)
- [ ] Grouped by category: Added, Changed, Fixed, Known Limitations
- [ ] References relevant task/PR numbers

## 5. Documentation Completeness

Evidence: all files verified present at `c21b453`

- [x] `README.md` — updated for single-cloud scope, links to all docs
- [x] `docs/getting-started.md` — 5-step quickstart, error table, trace ID walkthrough
- [x] `docs/deployment-manual.md` — full command flow, failure decision tree, download/verify
- [x] `docs/error-code-matrix.md` — 78 codes, Source column, cross-references
- [x] `docs/runbook.md` — alerting, rollback, cleanup, 6 incident scenarios
- [x] `docs/prerequisites-checklist.md` — account/credential/tool/quota/cost checks
- [x] `docs/demo-script.md` — 8-step demo, dry-run variant, talking points
- [x] `docs/release-notes-template.md` — template exists
- [x] `examples/README.md` — lists all examples

## 6. Documentation Cross-links

Evidence: `grep` verified all links present

- [x] `getting-started.md` links to `error-code-matrix.md` — line 167
- [x] `deployment-manual.md` links to `error-code-matrix.md` — line 6
- [x] `error-code-matrix.md` links to `06-single-cloud-operations.md` — line 171
- [x] `runbook.md` links to `error-code-matrix.md` and `06-single-cloud-operations.md` — lines 4–5
- [x] `demo-script.md` links to `prerequisites-checklist.md` and `error-code-matrix.md` — lines 5, 134
- [x] `README.md` links to `getting-started.md`, `runbook.md`, `prerequisites-checklist.md` — docs table

## 7. Single-Cloud Scope Compliance

Evidence: README rewritten at `c21b453`, multi-cloud references removed

- [x] No references to Huawei Cloud, Tencent Cloud, or Volcengine in user-facing docs
- [x] README reflects single-cloud (Alicloud) focus
- [x] No multi-cloud provider code paths exposed in CLI
- [x] `infracast.yaml` examples use `provider: alicloud` only

## 8. Known Issues & Limitations

These must be documented in `RELEASE-NOTES.md` (W4-3):

- [ ] ACK Verify deferred (account/billing gate) — documented
- [ ] Multi-cloud frozen (single-cloud only for v0.1.0) — documented
- [ ] `infracast status` is a stub (no `--output` flag) — documented
- [ ] Full E2E deploy requires ACK cluster + sufficient balance — documented

## 9. Release Artifacts

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

# Dependency check
go mod verify
go mod tidy && git diff --exit-code go.mod go.sum

# Release build
make release

# Version check
./bin/infracast version

# Doc link check
grep -r "error-code-matrix" docs/ --include="*.md"
grep -r "prerequisites-checklist" docs/ --include="*.md"
grep -r "06-single-cloud" docs/ --include="*.md"
```
