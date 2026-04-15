# Pre-release Checklist: v0.1.0 GA

Verify each item before tagging the GA release. All items must be checked with evidence.

---

## 1. CI & Build

Evidence: `b120857` on main, 2026-04-15

- [x] `gofmt -l .` returns empty — verified, no output
- [x] `go vet ./...` passes — verified, no warnings
- [x] `go test -race ./...` passes — verified, 16 packages pass, 0 failures
- [x] `go build ./cmd/infracast/` succeeds — verified, binary produced
- [x] `make release-build` produces 3 platform archives:
  - [x] `infracast_<ver>_darwin_amd64.tar.gz`
  - [x] `infracast_<ver>_darwin_arm64.tar.gz`
  - [x] `infracast_<ver>_linux_amd64.tar.gz`
- [x] `make release-checksums` generates valid `checksums.txt`
- [x] GitHub Actions CI workflow passes on main (equivalent checks verified locally)

## 2. Dependencies & Version Lock

Evidence: `go mod verify` → "all modules verified"

- [x] `go.mod` specifies Go 1.22+ toolchain
- [x] `go.sum` is committed and in sync (`go mod verify` passes)
- [x] No floating/unversioned dependencies in `go.mod`
- [x] All direct dependencies reviewed (no new deps since rc1)

## 3. Version & Metadata

- [x] `infracast version` outputs correct version, commit, build time
- [x] `cmd/infracast/main.go` ldflags variables are set (`Version`, `Commit`, `BuildTime`)
- [x] Git tag `v0.1.0-rc1` exists and points to correct commit
- [ ] Git tag `v0.1.0` created and pushed (to be done in #133)

## 4. CHANGELOG

- [x] `CHANGELOG.md` exists in project root
- [x] Covers all changes since initial release
- [x] Grouped by category: Added, Changed, Fixed, Known Limitations
- [x] References relevant task numbers

## 5. Documentation Completeness

Evidence: all files verified present at `b120857`

- [x] `README.md` — updated for single-cloud scope, links to all docs
- [x] `docs/getting-started.md` — 5-step quickstart, error table, trace ID walkthrough
- [x] `docs/deployment-manual.md` — full command flow, failure decision tree, download/verify
- [x] `docs/error-code-matrix.md` — 78 codes, Source column, cross-references
- [x] `docs/runbook.md` — alerting, rollback, cleanup, 6 incident scenarios
- [x] `docs/prerequisites-checklist.md` — account/credential/tool/quota/cost checks
- [x] `docs/demo-script.md` — 8-step demo, dry-run variant, talking points
- [x] `docs/release-notes-template.md` — template exists
- [x] `docs/zh-CN/README.md` — Chinese docs navigation index
- [x] `docs/zh-CN/getting-started.md` — Chinese quickstart
- [x] `docs/zh-CN/deployment-manual.md` — Chinese deployment manual
- [x] `docs/zh-CN/runbook.md` — Chinese operations runbook
- [x] `docs/zh-CN/prerequisites-checklist.md` — Chinese prerequisites
- [x] `docs/zh-CN/demo-script.md` — Chinese demo script
- [x] `docs/zh-CN/error-code-matrix.md` — Chinese error code matrix
- [x] `docs/zh-CN/release-notes-template.md` — Chinese release notes template
- [x] `docs/zh-CN/06-single-cloud-operations.md` — Chinese operations handbook
- [x] `examples/README.md` — lists all examples

## 6. Documentation Cross-links

Evidence: `grep` verified all links present

- [x] `getting-started.md` links to `error-code-matrix.md`
- [x] `deployment-manual.md` links to `error-code-matrix.md`
- [x] `error-code-matrix.md` links to `06-single-cloud-operations.md`
- [x] `runbook.md` links to `error-code-matrix.md` and `06-single-cloud-operations.md`
- [x] `demo-script.md` links to `prerequisites-checklist.md` and `error-code-matrix.md`
- [x] `README.md` links to `getting-started.md`, `runbook.md`, `prerequisites-checklist.md`
- [x] `docs/zh-CN/getting-started.md` relative links corrected (`../../` for parent traversal)

## 7. Single-Cloud Scope Compliance

Evidence: README rewritten, multi-cloud references removed

- [x] No references to Huawei Cloud, Tencent Cloud, or Volcengine in user-facing docs
- [x] README reflects single-cloud (Alicloud) focus
- [x] No multi-cloud provider code paths exposed in CLI
- [x] `infracast.yaml` examples use `provider: alicloud` only

## 8. Known Issues & Limitations

These are documented in `RELEASE-NOTES.md`:

- [x] ACK Verify deferred (account/billing gate) — documented
- [x] Multi-cloud frozen (single-cloud only for v0.1.0) — documented
- [x] `infracast status` is a stub (no `--output` flag) — documented
- [x] Full E2E deploy requires ACK cluster + sufficient balance — documented

## 9. Release Artifacts

- [x] Tag: `v0.1.0-rc1` (existing)
- [x] 3 platform archives built and checksummed
- [x] `RELEASE-NOTES.md` written (features, limitations, requirements)
- [ ] Tag: `v0.1.0` created and pushed (to be done in #133)
- [ ] GitHub Release created for v0.1.0 (to be done in #133)

## 10. rc1 → GA Diff Review

- [x] 4 commits after `v0.1.0-rc1`:
  - `40943dc` docs(zh-CN): add complete Chinese documentation set
  - `12acb95` docs: fix invalid CLI commands and broken links in EN/zh-CN docs
  - `5ae20bc` docs(zh-CN): fix relative links in getting-started.md
  - `b120857` docs(zh-CN): fix relative link paths for parent directory traversal
- [x] Impact: **documentation only**, zero code changes
- [x] Risk: **none** — no behavioral changes to CLI or deployment pipeline

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

# Regression suite
make regression

# Version check
./bin/infracast version

# Doc link check
grep -r "error-code-matrix" docs/ --include="*.md"
grep -r "prerequisites-checklist" docs/ --include="*.md"
grep -r "06-single-cloud" docs/ --include="*.md"
```
