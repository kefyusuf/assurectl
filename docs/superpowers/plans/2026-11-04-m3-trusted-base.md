# M3 Trusted-Base Resolution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Design:** `docs/superpowers/specs/2026-11-04-m3-trusted-base-design.md`  
**Base commit:** `15ade46128fac73dee41b894b08597e9ae3e436d`

## Global Constraints

- Every step is test-first: RED must fail for the intended reason before implementation.
- Production changes: `internal/inputmeta` (two vocabulary values), `internal/gitsubject` (one exported seam), new `internal/policy/loadbase.go` + `internal/contract/loadbase.go` code paths, `internal/verify` (`Options.TrustedBase`), `internal/receipt` (authority + provenance validation), `cmd/assurectl` (`--trusted-base`), `docs/adr/0009-*.md`. `internal/evaluation`, `internal/evidence`, `internal/waiver`, `internal/decision`, `internal/domain`, `schemas/`, `go.mod` untouched.
- Advisory mode is byte-identical to today (the dogfooding loop must not change).
- The Git execution pattern does not change: the blob reader reuses the resolver's trusted executable, sanitized environment, and argument-list execution; the path allowlist is `.assurectl/policy.json` and `.assurectl/contract.json` only.
- Cross-version gofmt (both toolchains on a clean LF view) before every push.

## File Structure

### Create

- `docs/adr/0009-trusted-base-input-resolution.md` — ADR summarizing the design's boundary.
- `internal/policy/loadbase_test.go`, `internal/contract/loadbase_test.go` — base loader tables.
- `internal/verify/trustedbase_test.go` — integration tests with temporary repositories.
- `internal/receipt/authoritative_test.go` — authority/provenance validation tests.

### Modify

- `internal/inputmeta/meta.go` — `TrustStatusTrusted`, `AuthorityBasisTrustedBase`.
- `internal/gitsubject/blob.go` — exported `ReadBlob` over the existing vetted execution path.
- `internal/policy/loader.go`, `internal/contract/loader.go` — `LoadBase` entry points sharing the existing normalization/validation.
- `internal/verify/verify.go`, `internal/receipt/build.go`, `internal/receipt/validate.go`, `cmd/assurectl/verify.go`, `cmd/assurectl/main.go` — mode plumbing and validation.
- `docs/superpowers/specs/2026-11-04-m3-trusted-base-design.md` — status only after verified implementation.

## Review Focus

1. **No fallback** — trusted-base mode reads only from the base; a missing base document is an error, never a workspace fallback.
2. **Allowlisted paths** — `ReadBlob` must reject every path outside the two `.assurectl` documents before Git runs.
3. **TRUSTED is earned by source** — only the base loaders produce `TRUSTED`/`TRUSTED_BASE`; the receipt builder rejects `TRUSTED` inputs whose source is not a `base:<oid>:<path>` form (no hand-forged trust).
4. **Advisory path untouched** — no flag, byte-identical behavior including the CI dogfooding loop.

---

### Task 1: ADR-0009 and Vocabulary Extension

**Files:** create `docs/adr/0009-trusted-base-input-resolution.md`; modify `internal/inputmeta/meta.go` + its tests.

- [ ] **Step 1: RED** — extend `inputmeta` tests to require the two new values to exist and validate.
- [ ] **Step 2: Observe RED**, write the ADR (boundary per design §2) and the vocabulary values.
- [ ] **Step 3: GREEN**, then commit `docs: bound trusted-base resolution with ADR-0009`.

### Task 2: Base Blob Reader

**Files:** create `internal/gitsubject/blob.go`, `internal/gitsubject/blob_test.go`.

- [ ] **Step 1: RED** — table tests with temporary repositories: happy path reads both allowlisted documents at a resolved revision; missing blob errors; every non-allowlisted path (including traversal forms) errors before execution; unresolved/short revision errors; nested worktree works.
- [ ] **Step 2: Observe RED.**
- [ ] **Step 3: Implement** `ReadBlob` reusing the resolver's execution machinery.
- [ ] **Step 4: GREEN** + vet.
- [ ] **Step 5: Commit** `feat: read allowlisted base blobs through the vetted Git path`.

### Task 3: Base Loaders

**Files:** create `internal/policy/loadbase_test.go`, `internal/contract/loadbase_test.go`; modify both loaders.

- [ ] **Step 1: RED** — mirror the workspace loader tables: valid documents load with `TRUSTED`/`TRUSTED_BASE` and `base:<revision>:<path>` sources; malformed base documents fail closed; digests are canonical; the same base bytes load identically across repeated calls.
- [ ] **Step 2: Observe RED.**
- [ ] **Step 3: Implement** the shared-code entry points.
- [ ] **Step 4: GREEN.**
- [ ] **Step 5: Commit** `feat: load policy and contract from the base revision`.

### Task 4: Orchestration, Receipt Authority, and CLI

**Files:** modify `internal/verify/verify.go`, `internal/receipt/build.go`, `internal/receipt/validate.go`, `cmd/assurectl/verify.go`, `cmd/assurectl/main.go`; create `internal/verify/trustedbase_test.go`, `internal/receipt/authoritative_test.go`.

- [ ] **Step 1: RED** — integration tests: candidate workspace policy differing from base policy yields an authoritative receipt recording the BASE policy digest; missing base policy errors with no receipt; advisory mode byte-identical without the flag; `--trusted-base` composes with `--ci`; receipt tests: `AUTHORITATIVE` requires trusted inputs and the `SUBJECT_BASE` marker, hand-forged `TRUSTED` sources are rejected.
- [ ] **Step 2: Observe RED.**
- [ ] **Step 3: Implement** the mode plumbing, builder validation, and CLI flag (usage line updated).
- [ ] **Step 4: GREEN** for the whole repository.
- [ ] **Step 5: Commit** `feat: add trusted-base verification mode`.

### Task 5: Full Verification, Scope Audit, and Documentation Alignment

- [ ] **Step 1: Focused tests** across the touched packages.
- [ ] **Step 2: Required repository verification** (CONTRIBUTING.md) + both-toolchain gofmt.
- [ ] **Step 3: Scope audit** — changed files match the Create/Modify list; advisory CI loop unchanged (the dogfooding verify job still runs advisory and must stay green).
- [ ] **Step 4: Trust-boundary re-read** — no fallback to workspace in trusted mode; allowlist enforced before execution; `TRUSTED` only from base loaders; no claims beyond the recorded boundary.
- [ ] **Step 5: Docs-only commit** flipping the spec status to `Implemented in M3 trusted-base slice` with plan checkbox updates.
- [ ] **Step 6: Push and require exact-head CI**; record head SHA and run IDs here.
- [ ] **Step 7: Final review gate** against the design, ADR-0004, ADR-0007, ADR-0009, and `docs/threat-model/v0.md`.

## Exit Criteria

- Trusted-base mode resolves policy and contract from the recorded base revision, fail-closed, with no workspace fallback.
- Authoritative receipts exist only with `TRUSTED` inputs whose provenance markers resolve to the subject's base (`SUBJECT_BASE`), and hand-forged trust is rejected.
- Advisory mode, including the CI dogfooding loop, is byte-identical.
- No execution-pattern, dependency, schema, or evaluation-kernel change; exact-head CI green before merge readiness.
