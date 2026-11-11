# M4 Conformance Suite Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Design:** `docs/superpowers/specs/2026-11-11-m4-conformance-design.md`  
**Base commit:** `d70d6801efc51ded7fb4b4c905c735f17ea3560c`

## Global Constraints

- Every vector file is a committed JSON fixture (not generated in-test); the harness reads them and asserts the expectation.
- No production code changes to `internal/attestation`, `internal/receipt`, `internal/evaluation`, or other evaluator packages. New package `internal/conformance` is a test-only harness.
- No new dependency; no network; standard-library crypto only.
- Cross-version gofmt (both toolchains on a clean LF view) before every push.

## File Structure

### Create

- `conformance/` directory with JSON vector files (see design §2.1).
- `internal/conformance/harness.go` + `harness_test.go` — the Go conformance runner.

### Modify

- `docs/superpowers/specs/2026-11-11-m4-conformance-design.md` — status only after verified implementation.

---

### Task 1: Evidence Envelope Vectors

- [ ] **Step 1: RED** — harness test reads `conformance/evidence-envelope/` and validates each vector against the envelope schema + attestation validation.
- [ ] **Step 2: Observe RED.**
- [ ] **Step 3: Create** vector files: valid-minimal, valid-full, invalid-missing-subject, invalid-bad-revision, invalid-bad-digest, invalid-stale-timestamp, invalid-unsupported-schema.
- [ ] **Step 4: GREEN.**
- [ ] **Step 5: Commit** `feat: add evidence envelope conformance vectors`.

### Task 2: Receipt Vectors

- [ ] **Step 1: RED** — receipt vectors: valid-advisory-approved, valid-authoritative, invalid-duplicate-requirement, invalid-evidence-mismatch, invalid-missing-verifier, invalid-bad-evaluated-at.
- [ ] **Step 2: Observe RED.**
- [ ] **Step 3: Create** vector files.
- [ ] **Step 4: GREEN.**
- [ ] **Step 5: Commit** `feat: add receipt conformance vectors`.

### Task 3: Trusted Root and Signed Root Vectors

- [ ] **Step 1: RED** — trusted-root vectors: valid-initial, valid-rotation (old+new), valid-revocation, invalid-rollback, invalid-self-introduction, invalid-expired-issuer, invalid-unknown-key, invalid-duplicate-key.
- [ ] **Step 2: Observe RED.**
- [ ] **Step 3: Create** vector files (fixed Ed25519 seed for signing vectors).
- [ ] **Step 4: GREEN.**
- [ ] **Step 5: Commit** `feat: add trusted root conformance vectors`.

### Task 4: Check Payload Vectors

- [ ] **Step 1: RED** — check-payload vectors: valid-success, valid-failure, valid-neutral, invalid-unknown-conclusion, invalid-missing-receipt-ref.
- [ ] **Step 2: Observe RED.**
- [ ] **Step 3: Create** vector files.
- [ ] **Step 4: GREEN.**
- [ ] **Step 5: Commit** `feat: add check payload conformance vectors`.

### Task 5: Full Verification and Documentation Alignment

- [ ] **Step 1: Repository verification** (CONTRIBUTING.md) + both-toolchain gofmt.
- [ ] **Step 2: Scope audit** — no evaluator changes; the harness only reads committed JSON vectors.
- [ ] **Step 3: Docs-only commit** flipping the spec status to `Implemented in M4 conformance slice`.
- [ ] **Step 4: Push and require exact-head CI**; record run IDs.
- [ ] **Step 5: Final review gate** against the design, ADR-0006, foundation §12, and `docs/threat-model/v0.md`.

## Exit Criteria

- The `conformance/` directory contains committed JSON vectors covering all six schema drafts and the signing/rotation protocol.
- The Go harness validates every vector; the suite passes in CI.
- No production code, dependency, or schema change; exact-head CI green before merge readiness is claimed.
