# M2 Waiver Records Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Design:** `docs/superpowers/specs/2026-11-02-m2-waiver-records-design.md`  
**Base commit:** `91cc1fa135f7477fad90eabde8997bcc47af6ee2`

## Global Constraints

- Every step is test-first: RED must fail for the intended reason before implementation.
- Production changes: new `internal/waiver`, `internal/evaluation` (waiver application only), `internal/receipt` (waiver emission only), `internal/verify` (flag plumbing), `cmd/assurectl` (`--waivers` flag), new `docs/adr/0008-*.md` and `schemas/waiver-record.v0.schema.json`. `internal/decision` and `internal/domain` stay untouched.
- No `time.Now()` outside the CLI adapter; no network; no new dependency.
- Waiver application never mutates evidence state, outcome, findings (beyond its own codes), or the verdict.
- `ACCEPTED_WITH_RISK` must be reachable end-to-end in this slice through a valid waiver on a waivable failure.
- Cross-version gofmt check (both toolchains on a clean LF view) before every push.

## File Structure

### Create

- `docs/adr/0008-advisory-explicit-waiver-inputs.md` — the ADR summarized in design §2.
- `schemas/waiver-record.v0.schema.json` — v0 draft matching the receipt's `waiver_record` definition.
- `internal/waiver/waiver.go` — typed record, `LoadLocal(path)`, canonical digest, fail-closed validation.
- `internal/waiver/waiver_test.go` — loader table and security regressions.
- `internal/evaluation/waiver_test.go` — engine application and rejection tables.
- `cmd/assurectl/verify_test.go` — `--waivers` cases appended.

### Modify

- `internal/evaluation/evaluate.go` — `Input.Waivers`, application engine, new finding codes.
- `internal/receipt/build.go` — copy waiver records into the receipt (builder checks already enforce the invariants).
- `internal/verify/verify.go` — `Options.Waivers`, load, pass through.
- `cmd/assurectl/verify.go` — `--waivers <path>` flag.
- `docs/superpowers/specs/2026-11-02-m2-waiver-records-design.md` — status only after verified implementation.

## Review Focus

1. **Waivers never touch facts** — evidence states, outcomes, non-waiver findings, and the verdict are untouched by application.
2. **No silent self-authorization** — waivers load only from the explicit `--waivers` path; no auto-discovery.
3. **Rejected waivers stay visible** — rejected records appear in the receipt with digests and non-blocking findings; the targeted failure keeps driving `REJECTED`.
4. **Kernel remains the only decision authority** — application sets `WaiverStatus`; the kernel branch already tested in M0 produces `ACCEPTED_WITH_RISK`.
5. **Schema draft matches the receipt's waiver_record** — field-for-field, so receipts stay schema-conformant.

---

### Task 1: ADR-0008 and Schema Draft

**Files:** create `docs/adr/0008-advisory-explicit-waiver-inputs.md`, `schemas/waiver-record.v0.schema.json`; extend `schemas/schema_syntax_test.go` coverage.

- [ ] **Step 1: RED** — extend the schema-syntax test to require the new draft to parse and to reuse the receipt's `waiver_record` field set.
- [ ] **Step 2: Observe RED**, write the ADR and the schema draft.
- [ ] **Step 3: GREEN**, then commit `docs: bound waiver inputs with ADR-0008`.

### Task 2: Waiver Loader

**Files:** create `internal/waiver/waiver.go`, `internal/waiver/waiver_test.go`.

- [ ] **Step 1: RED loader tests** — valid record round-trip with canonical digest; duplicate `id` rejected; `FINDING` target kind rejected; non-canonical identifiers rejected; unparseable or pre-dated expiry rejected; oversized file rejected (1 MiB bound); missing file yields an empty set, not an error; digest is formatting-independent.
- [ ] **Step 2: Observe RED.**
- [ ] **Step 3: Implement** the typed record and `LoadLocal` with the same strict-decoding discipline as the other loaders.
- [ ] **Step 4: GREEN** + vet.
- [ ] **Step 5: Commit** `feat: load typed waiver records`.

### Task 3: Waiver Application Engine

**Files:** modify `internal/evaluation/evaluate.go`; create `internal/evaluation/waiver_test.go`.

- [ ] **Step 1: RED engine tests** — per design §4: apply on waivable VALID/FAILED with matching scope and timing (`WaiverStatus` becomes `VALID`, `WAIVER_APPLIED` info finding); first record wins, later duplicates superseded; one rejection code per reason (`TARGET`, `SCOPE`, `EXPIRED`, `TIMING`); non-waivable and non-failed targets rejected without blocking; PASSED requirements unaffected; facts never mutated; shuffled input keeps deterministic output.
- [ ] **Step 2: Observe RED.**
- [ ] **Step 3: Implement** the application pass inside `Evaluate` before kernel composition.
- [ ] **Step 4: GREEN.**
- [ ] **Step 5: Commit** `feat: apply waiver records in evaluation`.

### Task 4: Receipt Emission and ACCEPTED_WITH_RISK End-to-End

**Files:** modify `internal/receipt/build.go`; extend `internal/receipt` tests.

- [ ] **Step 1: RED tests** — a built receipt carries applied and rejected waiver records with their digests; an all-waivered-failures evaluation yields an `ACCEPTED_WITH_RISK` receipt whose re-derivation check passes; a rejected-waiver receipt keeps decision `REJECTED`.
- [ ] **Step 2: Observe RED.**
- [ ] **Step 3: Implement** the emission mapping.
- [ ] **Step 4: GREEN.**
- [ ] **Step 5: Commit** `feat: emit waiver records in advisory receipts`.

### Task 5: Orchestrator and CLI Flag

**Files:** modify `internal/verify/verify.go`, `internal/verify/verify_test.go`, `cmd/assurectl/verify.go`, `cmd/assurectl/verify_test.go`.

- [ ] **Step 1: RED tests** — `verify.Run` loads and applies a waiver file (accepted-with-risk receipt end-to-end in a temp repository); a malformed waiver file fails the run with no receipt; CLI `--waivers` happy path, missing file (exit 2), malformed file (exit 2), and the flag documented in usage.
- [ ] **Step 2: Observe RED.**
- [ ] **Step 3: Implement** the plumbing.
- [ ] **Step 4: GREEN** for the whole repository.
- [ ] **Step 5: Commit** `feat: accept explicit waiver input in verify`.

### Task 6: Full Verification, Scope Audit, and Documentation Alignment

**Files:** modify only the design spec status if verified.

- [ ] **Step 1: Focused tests** across `internal/waiver`, `internal/evaluation`, `internal/receipt`, `internal/verify`, `cmd/assurectl`.
- [ ] **Step 2: Required repository verification** (CONTRIBUTING.md), including both-toolchain gofmt.
- [ ] **Step 3: Scope audit** — only the Create/Modify list changed; no dependency; no `time.Now` outside the CLI adapter.
- [ ] **Step 4: Trust-boundary re-read** — no auto-discovery; rejected waivers visible; kernel untouched; advisory authority only.
- [ ] **Step 5: Docs-only commit** flipping the spec status to `Implemented in M2 waiver-records slice` with plan checkbox updates.
- [ ] **Step 6: Push and require exact-head CI**; record head SHA and run ID here.
- [ ] **Step 7: Final review gate** against the design, ADR-0002, ADR-0004, ADR-0008, and `docs/threat-model/v0.md`.

## Exit Criteria

- The waiver input is explicit, advisory, digest-bearing, and fail-closed at the loader.
- Application is deterministic, scope/timing-bound, non-mutating to facts, and rejection-visible.
- `ACCEPTED_WITH_RISK` is reachable end-to-end and schema-condition checks hold on the emitted receipt.
- No decision, schema, or trust-model change beyond ADR-0008's bounded surface; exact-head CI green before merge readiness.
