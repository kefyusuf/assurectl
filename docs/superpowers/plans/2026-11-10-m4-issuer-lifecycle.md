# M4 Issuer Lifecycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Design:** `docs/superpowers/specs/2026-11-10-m4-issuer-lifecycle-design.md`  
**Base commit:** `23eb7c8d3082b7eef07932710fa01f1c2668c605`

## Global Constraints

- Every step is test-first: RED must fail for the intended reason; all tests are offline (in-process Ed25519 keys, no network).
- Production changes: `internal/attestation` (signed-root parsing/verification/acceptance), `cmd/assurectl` (`sign-root` subcommand; `verify-receipt` signed-root flags), `examples/signed/` rotation fixtures. No changes to `internal/receipt`, `internal/evaluation`, `internal/appserver`, existing schemas, `go.mod`.
- Standard-library crypto only; PAE byte-exact; fail closed on every broken-chain, rollback, self-introduction, and dual-root-source path.
- Cross-version gofmt (both toolchains on a clean LF view) before every push.

## File Structure

### Create

- `internal/attestation/signedroot.go` + `signedroot_test.go` — signed-root parsing, acceptance rules (pin, monotonicity, rotation, windows), revocation resolution.
- `examples/signed/rotation_test.go` — v1 → v2 (rotation) → v3 (revocation) chain fixtures with `FIXTURE_UPDATE=1`.

### Modify

- `cmd/assurectl/sign.go` — `sign-root` subcommand.
- `cmd/assurectl/verify_receipt.go` — `--trusted-root-signed` / `--root-keyid` / `--min-root-version`.
- `cmd/assurectl/main.go` — usage lines.
- `docs/superpowers/specs/2026-11-10-m4-issuer-lifecycle-design.md` — status only after verified implementation.

---

### Task 1: Signed-Root Format and Verification

- [x] **Step 1: RED** — signed-root parse/verify table: valid root accepted against the pinned key id; bad signature, wrong payload type shape, unsigned root, and multiple signatures rejected; PAE coverage asserted (signature fails when any payload byte changes).
- [x] **Step 2: Observe RED.**
- [x] **Step 3: Implement** `signedroot.go` (payload reuse of the slice-1 shape; PAE; Ed25519).
- [x] **Step 4: GREEN** + vet.
- [x] **Step 5: Commit** `feat: parse and verify signed trusted roots`.

### Task 2: Acceptance Rules — Pin, Monotonicity, Rotation

- [x] **Step 1: RED** — acceptance table: root accepted when signed by the pinned key; version rollback rejected; a root signed only by a new key rejected (self-introduction); rotation accepted only with old+new signatures; revocation (dropped key) rejects receipts signed by that key after acceptance; validity windows interact correctly.
- [x] **Step 2: Observe RED.**
- [x] **Step 3: Implement** the acceptance rules over an accepted-version state.
- [x] **Step 4: GREEN.**
- [x] **Step 5: Commit** `feat: enforce root rotation and revocation rules`.

### Task 3: CLI Integration

- [x] **Step 1: RED** — `sign-root` produces a verifiable signed root (merge of per-key signature runs); `verify-receipt --trusted-root-signed` accepts a valid chain and rejects rollback; both `--trusted-root` and `--trusted-root-signed` together exit 64; `--min-root-version` enforces monotonicity; usage errors exit 64.
- [x] **Step 2: Observe RED.**
- [x] **Step 3: Implement** the CLI changes.
- [x] **Step 4: GREEN** for the whole repository.
- [x] **Step 5: Commit** `feat: add root signing and signed-root verification commands`.

### Task 4: Rotation-Chain Fixtures

- [x] **Step 1: RED** — `examples/signed/rotation_test.go`: v1 (initial) → v2 (root-key rotation, old+new signatures) → v3 (issuer revocation) chain fixtures with `FIXTURE_UPDATE=1`; the committed chain must verify step-by-step and the v3 root must reject receipts from the revoked key.
- [x] **Step 2: Observe RED.**
- [x] **Step 3: Generate** the committed fixtures with the fixed conformance seeds.
- [x] **Step 4: GREEN** + both-toolchain gofmt.
- [x] **Step 5: Commit** `feat: add root rotation conformance fixtures`.

### Task 5: Full Verification, Scope Audit, and Documentation Alignment

- [x] **Step 1: Focused tests** across `internal/attestation`, `cmd/assurectl`, `examples/signed`.
- [x] **Step 2: Required repository verification** (CONTRIBUTING.md) + both-toolchain gofmt.
- [x] **Step 3: Scope audit** — changed files match the Create/Modify list; no evaluator/receipt/App changes; no dependency.
- [x] **Step 4: Trust-boundary re-read** — initial trust is a pin; new keys cannot self-introduce; rollback fails closed; no network; no secret material in logs.
- [x] **Step 5: Docs-only commit** flipping the spec status to `Implemented in M4 issuer-lifecycle slice` with plan checkbox updates.
- [x] **Step 6: Push and require exact-head CI**; record head SHA and run IDs here.
  - workflow run: `42859194659` (Go 1.26.x pass, Go 1.27.x pass, blocking `Verify with own evidence` pass);
  - formatting: pass with BOTH toolchains (1.27 local + Go 1.26.10 on a clean LF view);
  - vet: pass; race-test: pass (ubuntu-latest); build: pass.
- [x] **Step 7: Final review gate** against the design, ADR-0006, foundation §12, and `docs/threat-model/v0.md`.
  - Review performed: changed files match the plan's Create/Modify list exactly; initial trust is a pin; rollback, self-introduction, and broken chains fail closed; the empty-issuers case is a full-revocation fail-closed state; no network; no dependency; the dogfooding CI loop is unchanged. Two design refinements during implementation: ParseTrustedRoot's version check relaxed from `== 1` to `>= 1` (rotation increments it) and the empty-issuers case allowed (full revocation); both documented in the PR. A pre-commit Mimosa scan flags pre-existing gitsubject exec patterns (command injection) and a cross-file-taint heuristic on the CLI entry; assessed as scanner false positives.

## Exit Criteria

- Trusted roots are signed, versioned, and monotonic; rotation requires old+new authorization; revocation propagates to receipt verification; rollback and self-introduction fail closed.
- `verify-receipt` accepts exactly one root source; persisted-state callers can enforce a minimum root version.
- Rotation-chain conformance fixtures are committed and reproducible.
- No dependency, evaluator, receipt-construction, or existing-schema change; exact-head CI green before merge readiness is claimed.
