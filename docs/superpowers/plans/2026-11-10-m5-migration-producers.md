# M5 Migration Producer Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Design:** `docs/superpowers/specs/2026-11-10-m5-migration-producers-design.md`  
**Base commit:** `66ec9e3e5a7d0b40b2cdbab7b34ee2a9b9042b3a`

## Global Constraints

- Every step is test-first: RED must fail for the intended reason; all tests are offline.
- Production changes: `cmd/assurectl/init.go` (+ test), `cmd/assurectl/main.go` (usage), `examples/migration/` fixtures + test, `docs/guides/migration-producers.md`. No changes to `internal/**` (the evaluator, attestation, receipt, and appserver packages are untouched).
- Generated documents must pass `contract.LoadLocal` and `policy.LoadLocal` validation.
- Cross-version gofmt (both toolchains on a clean LF view) before every push.

## File Structure

### Create

- `cmd/assurectl/init.go` + `init_test.go` — `assurectl init` subcommand.
- `examples/migration/` — per-type contract/policy/evidence fixture triples + test.

### Modify

- `cmd/assurectl/main.go` — subcommand registration + usage lines.
- `docs/superpowers/specs/2026-11-10-m5-migration-producers-design.md` — status only after verified implementation.

---

### Task 1: `assurectl init` Subcommand

- [ ] **Step 1: RED** — init happy path per type (contract + policy files created, valid JSON, pass `LoadLocal`); fail-closed on existing files; unknown type rejected; missing required flags rejected.
- [ ] **Step 2: Observe RED.**
- [ ] **Step 3: Implement** `init.go` with template generation.
- [ ] **Step 4: GREEN** + whole-repo tests.
- [ ] **Step 5: Commit** `feat: add assurectl init for migration producers`.

### Task 2: Example Fixtures

- [ ] **Step 1: RED** — mirror `examples/basic` test patterns for `examples/migration/`: JSON syntax, schema versions, work-unit types, evidence digest match.
- [ ] **Step 2: Observe RED.**
- [ ] **Step 3: Create** fixtures for `api_migration` and `dependency_upgrade`.
- [ ] **Step 4: GREEN.**
- [ ] **Step 5: Commit** `feat: add migration producer example fixtures`.

### Task 3: Producer Guide

- [ ] **Step 1: Write** `docs/guides/migration-producers.md` per design §3.1.
- [ ] **Step 2: Review pass** — claims match implemented behavior.
- [ ] **Step 3: Commit** `docs: add migration producer integration guide`.

### Task 4: Full Verification and Documentation Alignment

- [ ] **Step 1: Repository verification** (CONTRIBUTING.md) + both-toolchain gofmt.
- [ ] **Step 2: Scope audit** — no `internal/**` changes.
- [ ] **Step 3: Docs-only commit** flipping the spec status to `Implemented in M5 migration-producers slice`.
- [ ] **Step 4: Push and require exact-head CI**; record run IDs.
- [ ] **Step 5: Final review gate** against the design, foundation §3 (product boundary), and §16 (milestone roadmap).

## Exit Criteria

- `assurectl init` scaffolds valid, loadable contract and policy documents per work-unit type.
- Example fixtures demonstrate the migration producer pattern end-to-end.
- The producer guide is grounded in implemented behavior.
- No `internal/**` or dependency change; exact-head CI green before merge readiness.
