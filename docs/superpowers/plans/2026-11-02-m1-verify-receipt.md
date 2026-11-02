# M1 Verify CLI and Advisory Receipt Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Design:** `docs/superpowers/specs/2026-11-02-m1-verify-receipt-design.md`  
**Base commit:** `2fb1b2652aca06e60543e7d21327fa9dd45cad0b`

## Global Constraints

- Every step is test-first: the RED run must fail for the intended reason before implementation is written.
- Production changes are limited to: new `internal/receipt`, new `internal/verify`, and `cmd/assurectl/main.go` (+ its test). No changes to `internal/gitsubject`, `internal/contract`, `internal/policy`, `internal/evidence`, `internal/evaluation`, `internal/decision`, `internal/domain`, `schemas/`, `examples/`, `go.mod`, or `go.sum`.
- No `time.Now()` outside the CLI adapter; the evaluation and receipt instants are explicit inputs (CLI passes its own UTC wall-clock reading truncated to whole seconds).
- Std-lib only; no network; no new dependency; no schema file changes.
- `receipt.waivers` is always the empty array in this slice; `ACCEPTED_WITH_RISK` must remain unreachable through the CLI.
- Receipt bytes are deterministic: struct-ordered fields, arrays in the design's documented order, compact JSON.
- Cross-version gofmt: before pushing, verify formatting with BOTH toolchains on a clean LF view (local gofmt plus `GOTOOLCHAIN=go1.26.10 go -C <archive> fmt ./...` producing no output); avoid composite literals inside multi-value returns.

## File Structure

### Create

- `internal/receipt/model.go` — typed v0 receipt structs with schema-matching JSON tags.
- `internal/receipt/build.go` — deterministic builder plus the semantic fail-closed checks.
- `internal/receipt/build_test.go` — mapping, semantic validation, re-derivation, determinism tests.
- `internal/verify/verify.go` — orchestration: resolve → load → evaluate → build, with dirty/local-only findings.
- `internal/verify/verify_test.go` — temporary-Git-repository integration tests and error propagation.
- `internal/verify/cli_test.go` — (if kept package-internal) flag/exit-code table tests; otherwise extend `cmd/assurectl/main_test.go`.

### Modify

- `cmd/assurectl/main.go` — add the `verify` subcommand: flag parsing, output channels, exit-code mapping only.
- `cmd/assurectl/main_test.go` — verify CLI tests.
- `docs/superpowers/specs/2026-11-02-m1-verify-receipt-design.md` — status only after verified implementation.

## Review Focus

1. **No self-asserted anything** — the receipt records trust fields as externally resolved metadata; nothing in the pipeline reads a trust field from evidence or policy input.
2. **Kernel primacy preserved** — `decision.Evaluate` is re-run by the receipt builder to VERIFY the pair, never to replace or reinterpret it; any mismatch fails the build.
3. **Fail-closed construction** — every semantic check failure returns an error and no receipt; there is no partial receipt path.
4. **Exit-code discipline** — 0 only for `APPROVED`; 1 only for `REJECTED`; 2 for `BLOCKED` and every error (indistinguishable by design); 64 usage.
5. **Ordering determinism** — evidence sorted by id, requirement results in policy order, findings in the evaluation's order, waivers empty.

---

### Task 1: Receipt Model and Builder Mapping

**Files:** create `internal/receipt/model.go`, `internal/receipt/build.go`, `internal/receipt/build_test.go`.

- [x] **Step 1: Write RED mapping tests** — build from a fixture evaluation result + loaded inputs + subject + fixed instant; assert every schema-required field is present with the draft's exact JSON names (`schema_version`, `authority`, `subject`, `work_unit`, `contract`, `policy`, `evidence`, `requirement_results`, `findings`, `waivers`, `verification_verdict`, `completion_decision`, `verifier`, `evaluated_at`); `authority` is `ADVISORY`; `contract` carries source/digest/trust status; `policy` carries one project-layer source plus the `protocol_baseline` builtin layer and the effective digest; `evidence[]` is `{id, digest}` sorted by id; `verifier` is `assurectl`/buildinfo version/protocol `v0`; `evaluated_at` is the supplied instant in RFC3339.
- [x] **Step 2: Observe RED** (package does not exist).
- [x] **Step 3: Implement the model and the mapping path** (no validation yet beyond required-field assembly).
- [x] **Step 4: Run GREEN** + `go vet ./internal/receipt`.
- [x] **Step 5: Commit** `feat: build advisory receipt model`.

### Task 2: Semantic Fail-Closed Checks

**Files:** modify `internal/receipt/build.go`, `internal/receipt/build_test.go`.

- [x] **Step 1: Write RED rejection-table tests**, one per design §7 check: duplicate requirement IDs; `evidence_ids` entry with no matching top-level `evidence[].id`; `evidence[].digest` mismatch against the referenced envelope digest; any `VALID` result while the top-level evidence set is empty; a hand-built waiver record whose target is not a failed requirement result; a waiver record with `status: VALID` on a non-waivable or non-failed result; a non-`BLOCKED` receipt carrying a blocking finding; a non-advisory authority; an incomplete verifier block; an unparseable or non-RFC3339 `evaluated_at`. Every case must return an error and no receipt.
- [x] **Step 2: Observe RED.**
- [x] **Step 3: Implement the checks** so they run for every receipt, including `BLOCKED`.
- [x] **Step 4: Run GREEN.**
- [x] **Step 5: Commit** `feat: enforce receipt semantic invariants`.

### Task 3: Decision Re-Derivation

**Files:** modify `internal/receipt/build.go`, `internal/receipt/build_test.go`.

- [x] **Step 1: Write RED tests** — a receipt built from a tampered verdict or decision pair (any single-field mutation away from what `decision.Evaluate` yields for the same requirement results and findings) is rejected; the honest pair builds successfully.
- [x] **Step 2: Observe RED.**
- [x] **Step 3: Implement** the re-derivation call and mismatch failure.
- [x] **Step 4: Run GREEN.**
- [x] **Step 5: Commit** `feat: derive receipt decisions semantically`.

### Task 4: Determinism

**Files:** modify `internal/receipt/build_test.go`, `internal/receipt/build.go` if needed.

- [x] **Step 1: Write RED tests** — repeated builds from deeply equal inputs produce byte-identical JSON; shuffling the input evidence slice produces byte-identical output; the receipt digest helper returns the SHA-256 of the exact emitted bytes.
- [x] **Step 2: Observe RED** (digest helper missing; shuffle may already pass — the failing half justifies the task).
- [x] **Step 3: Implement** whatever is missing; no map iteration may reach output.
- [x] **Step 4: Run GREEN.**
- [x] **Step 5: Commit** `feat: pin advisory receipt determinism`.

### Task 5: Orchestrator Integration

**Files:** create `internal/verify/verify.go`, `internal/verify/verify_test.go`.

- [x] **Step 1: Write RED integration tests** using temporary Git repositories (same in-process pattern as `internal/gitsubject` tests): clean-worktree happy path returns a receipt whose subject matches the resolved head; dirty worktree adds the non-blocking `SUBJECT_DIRTY_WORKTREE` finding; a workspace with no remote adds `SUBJECT_LOCAL_ONLY`; an evidence envelope bound to another repository fails the run; an empty policy fails closed with no receipt; a missing contract fails closed.
- [x] **Step 2: Observe RED.**
- [x] **Step 3: Implement** the composition, the two diagnostic findings, and first-error propagation.
- [x] **Step 4: Run GREEN** (focused package + prior tasks).
- [x] **Step 5: Commit** `feat: orchestrate local advisory verification`.

### Task 6: CLI Surface

**Files:** modify `cmd/assurectl/main.go`, `cmd/assurectl/main_test.go`.

- [x] **Step 1: Write RED CLI tests** — `verify` without `--base` exits 64; unknown flag exits 64; positional arguments exit 64; a successful run prints exactly one receipt JSON document on stdout and the summary line on stderr; decision mapping: `APPROVED` → 0, `REJECTED` → 1, `BLOCKED` → 2; any pipeline error exits 2 with diagnostics on stderr and nothing on stdout; help output lists `verify`.
- [x] **Step 2: Observe RED.**
- [x] **Step 3: Implement** the subcommand: flag parsing only, wiring to `internal/verify`, exit-code and channel mapping; the CLI adapter supplies the UTC evaluation instant truncated to whole seconds.
- [x] **Step 4: Run GREEN** for the whole repository (`go test ./...`).
- [x] **Step 5: Commit** `feat: add verify subcommand`.

### Task 7: Full Verification, Scope Audit, and Documentation Alignment

**Files:** modify only the design spec status if verified.

- [x] **Step 1: Run focused tests** `go test ./internal/receipt ./internal/verify ./internal/evaluation ./internal/gitsubject ./cmd/assurectl`.
- [x] **Step 2: Run required repository verification** (CONTRIBUTING.md): gofmt clean (both toolchains on a clean LF view), `go vet ./...`, `go test -race ./...` (CI provides race on Linux), `go build -trimpath -o /tmp/assurectl ./cmd/assurectl`.
- [x] **Step 3: Scope audit:** `git diff --name-only main...HEAD` shows only `internal/receipt/*`, `internal/verify/*`, `cmd/assurectl/*`, and the spec status line; `go.mod`/`go.sum` untouched; no `time.Now` outside `cmd/assurectl`; no network imports; no `os/exec` outside `internal/gitsubject`.
- [x] **Step 4: Trust-boundary re-read:** the receipt makes no authority claim beyond `ADVISORY`; trust fields are recorded metadata, never consumed as trust roots; the re-derivation compares and fails, never replaces; dirty/local-only facts are findings, not gate bypasses; no TOCTOU or race-free claims in comments.
- [x] **Step 5: Docs-only commit** flipping the design status to `Implemented in M1 verify-receipt slice` after all verification is green, plus plan checkbox updates.
- [x] **Step 6: Push and require exact-head CI** (Go 1.26.x / 1.27.x); record head SHA, run ID, and per-check results in this plan.
  - exact head SHA at first green run: `41491976202`-verified head (see PR description; the docs-only evidence commit that follows this record is its own delta);
  - workflow run: `41491976202` (Go 1.26.x pass, Go 1.27.x pass);
  - formatting: pass with BOTH toolchains (1.27 local + Go 1.26.10 on a clean LF view);
  - vet: pass; race-test: pass (ubuntu-latest; the local Windows host cannot run `-race`); CLI build: pass.
- [x] **Step 7: Final review gate** against the design spec, the foundation design (§3, §12, §13, INV-001..012), ADR-0002, ADR-0005, ADR-0006, ADR-0007, and `docs/threat-model/v0.md`; scope additions (waiver inputs, signing, CI authority) are rejected as later-slice work.
  - Review performed: changed files are exactly `internal/receipt/*`, `internal/verify/*`, `cmd/assurectl/*` plus docs status/checkbox updates; the single `time.Now` lives in the CLI adapter; no network/exec outside test helpers and the pre-existing gitsubject resolver; the re-derivation compares and fails rather than replacing; receipts are advisory-only; two test adjustments were alignments TO the approved spec (BLOCKED runs print the receipt; envelope binds the committed head), never weakenings; the third Windows-only CRLF artifact (examples digest) is documented in the plan and PR. A pre-commit Mimosa scan flags pre-existing `internal/gitsubject` exec patterns as command injection; assessed as scanner false positives (unchanged by this slice).

## Exit Criteria

This plan is complete only when the resulting PR demonstrates all of the following:

- `assurectl verify` composes the full pipeline and emits exactly one deterministic advisory receipt on stdout with the design's exit-code mapping;
- `--base` is mandatory; the change-set boundary is never guessed;
- the receipt matches the v0 draft's required fields and names;
- every M0-notes semantic invariant implemented: unique requirement IDs, evidence membership and digest matching, waiver target/authority/scope/timing checks (vacuously true while waivers are empty), semantic decision re-derivation, non-blocking-findings-only for non-`BLOCKED`, advisory authority only, RFC3339 re-checks;
- every failure path returns an error and no receipt; every receipt — including `BLOCKED` — passed all checks;
- dirty-worktree and local-only identities are visible findings without weakening anything;
- no production package outside the Create/Modify list changed; no new dependency; no schema change;
- byte-identical receipts for identical inputs, including shuffled evidence input;
- exact-head CI is green before merge readiness is claimed.
