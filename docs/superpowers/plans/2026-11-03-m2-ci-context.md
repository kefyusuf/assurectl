# M2 CI-Asserted Context Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Design:** `docs/superpowers/specs/2026-11-03-m2-ci-context-design.md`  
**Base commit:** `276593160f52acf6140e7e8dab7359a182d180e7`

## Global Constraints

- Every step is test-first: RED must fail for the intended reason before implementation.
- Production changes: new `internal/cicontext`, `internal/verify` (cross-checks + authority), `internal/receipt` (authority acceptance), `cmd/assurectl` (`--ci`). No changes to `internal/decision`, `internal/domain`, `internal/evaluation`, `internal/evidence`, `internal/waiver`, `schemas/`, `go.mod`.
- Environment enters only through `cicontext.Read(environ []string)` and the CLI adapter; the core stays environment-free.
- No network, no JWT/OIDC handling, no new dependency, no schema change.
- Without `--ci` (or `Options.CI`), behavior is byte-identical to the previous release.
- Cross-version gofmt (both toolchains on a clean LF view) before every push.

## File Structure

### Create

- `internal/cicontext/cicontext.go` — typed context, `Read(environ)`, canonical repository.
- `internal/cicontext/cicontext_test.go` — read/canonicalization tables.
- `internal/verify/ci_test.go` — cross-check integration tests with temporary repositories.
- `internal/receipt/ci_test.go` — authority-tier acceptance tests.

### Modify

- `internal/verify/verify.go` — `Options.CI`, cross-checks, authority selection.
- `internal/receipt/build.go` — `BuildInput.CI`, tier validation.
- `cmd/assurectl/verify.go` — `--ci` flag and environment wiring.
- `cmd/assurectl/verify_test.go` — CLI cases appended.
- `docs/superpowers/specs/2026-11-03-m2-ci-context-design.md` — status only after verified implementation.

## Review Focus

1. **Assertion, not validation** — no code may claim to authenticate the runner; the receipt records an asserted tier and the subject cross-checks are the only safeguard.
2. **Closed environment surface** — only the listed `GITHUB_*` variables are read; no os.Getenv in production code outside the CLI adapter.
3. **Fail-closed cross-checks** — repository or SHA mismatch produces an error and no receipt, never a downgraded advisory receipt.
4. **Byte-identical advisory path** — without CI context, outputs are unchanged.

---

### Task 1: Context Adapter

**Files:** create `internal/cicontext/cicontext.go`, `internal/cicontext/cicontext_test.go`.

- [x] **Step 1: RED** — table tests for `Read(environ)`: happy path maps all eight variables; each missing required variable fails; malformed `GITHUB_SHA` (short/uppercase/non-hex) fails; malformed `GITHUB_REPOSITORY` fails; `CanonicalRepository()` returns `github.com/<owner>/<repo>`; optional variables stay empty when absent; unknown `GITHUB_*` variables are ignored.
- [x] **Step 2: Observe RED** (package missing).
- [x] **Step 3: Implement** the adapter with exact-field matching over the supplied environ slice.
- [x] **Step 4: GREEN** + vet.
- [x] **Step 5: Commit** `feat: read GitHub Actions verification context`.

### Task 2: Subject Cross-Checks and Authority Selection

**Files:** modify `internal/verify/verify.go`; create `internal/verify/ci_test.go`.

- [x] **Step 1: RED** — integration tests: matching CI context yields a `CI_ASSERTED` receipt whose subject equals the resolved one; repository mismatch fails; SHA mismatch fails; unset `CI` keeps the advisory path byte-identical.
- [x] **Step 2: Observe RED.**
- [x] **Step 3: Implement** the cross-checks after subject resolution and before input loading; authority passes through to the receipt input.
- [x] **Step 4: GREEN.**
- [x] **Step 5: Commit** `feat: cross-check CI context against resolved subjects`.

### Task 3: Receipt Authority

**Files:** modify `internal/receipt/build.go`; create `internal/receipt/ci_test.go`.

- [x] **Step 1: RED** — `CI_ASSERTED` with `BuildInput.CI` set builds; `CI_ASSERTED` without context is rejected; `ADVISORY` with a context set is rejected; other tiers stay rejected.
- [x] **Step 2: Observe RED.**
- [x] **Step 3: Implement** the tier validation.
- [x] **Step 4: GREEN.**
- [x] **Step 5: Commit** `feat: emit CI-asserted receipt authority`.

### Task 4: CLI Flag

**Files:** modify `cmd/assurectl/verify.go`, `cmd/assurectl/verify_test.go`, `cmd/assurectl/main.go` (usage line).

- [x] **Step 1: RED** — `--ci` with a fabricated GitHub environment runs the CI-asserted path (exit 0, authority `CI_ASSERTED` in the receipt); `--ci` without the variables exits 2 with diagnostics and no receipt; `--ci=false` or omitted keeps the advisory path; `--ci value` (positional form) is a usage error.
- [x] **Step 2: Observe RED.**
- [x] **Step 3: Implement** the flag and environment wiring.
- [x] **Step 4: GREEN** for the whole repository.
- [x] **Step 5: Commit** `feat: add --ci flag for CI-asserted verification`.

### Task 5: Full Verification, Scope Audit, and Documentation Alignment

**Files:** modify only the design spec status if verified.

- [x] **Step 1: Focused tests** across `internal/cicontext`, `internal/verify`, `internal/receipt`, `cmd/assurectl`.
- [x] **Step 2: Required repository verification** (CONTRIBUTING.md), including both-toolchain gofmt.
- [x] **Step 3: Scope audit** — only the Create/Modify list changed; no dependency; no network imports; environment reads confined to the CLI adapter and the adapter package.
- [x] **Step 4: Trust-boundary re-read** — no OIDC/auth claims in code or comments; receipts say CI_ASSERTED and nothing stronger; cross-check failures produce no receipt.
- [x] **Step 5: Docs-only commit** flipping the spec status to `Implemented in M2 ci-context slice` with plan checkbox updates.
- [x] **Step 6: Push and require exact-head CI**; record head SHA and run ID here.
  - workflow run: `41683620494` (Go 1.26.x pass, Go 1.27.x pass) on the pre-evidence head; the docs-only evidence commit that follows is its own delta with its own green run;
  - formatting: pass with BOTH toolchains (1.27 local + Go 1.26.10 on a clean LF view);
  - vet: pass; race-test: pass (ubuntu-latest; the local Windows host cannot run `-race`); CLI build: pass.
- [x] **Step 7: Final review gate** against the design, foundation §11 (Tier 1), ADR-0006, ADR-0007, and `docs/threat-model/v0.md`.
  - Review performed: changed files match the plan's Create/Modify list exactly; the environment surface is confined to `cicontext.Read` and the CLI adapter (no `os.Getenv` in production code); cross-check failures produce no receipt; no OIDC/network/cryptography surface; the advisory path is byte-identical without `--ci`. A pre-commit Mimosa scan flags pre-existing `internal/gitsubject` exec patterns as command injection; assessed as scanner false positives (unchanged by this slice).

## Exit Criteria

- The CI context is a closed, typed, fail-closed view of exactly the documented GitHub Actions variables.
- Subject cross-checks fail closed; CI-asserted receipts exist only when assertions agree with the resolved subject.
- The advisory path is byte-identical when the flag is absent.
- No OIDC/network/cryptography surface entered the slice; exact-head CI green before merge readiness.
