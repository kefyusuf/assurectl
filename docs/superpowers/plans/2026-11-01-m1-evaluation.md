# M1 Requirement Evaluation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Design:** `docs/superpowers/specs/2026-11-01-m1-evaluation-design.md`  
**Base commit:** `f795de63c37366105e9723ac701f8654326772d1`

## Global Constraints

- Every step is test-first: the RED run must fail for the intended reason before implementation is written.
- `internal/evaluation` is the only production package created or modified. No changes to `internal/decision`, `internal/domain`, `internal/evidence`, `internal/contract`, `internal/policy`, `internal/gitsubject`, `internal/localinput`, `internal/inputmeta`, `cmd/`, `schemas/`, `examples/`, `go.mod`, or `go.sum`.
- No `time.Now()` anywhere in the new package; the evaluation instant is an input.
- Std-lib only; no network, no filesystem access, no environment reads.
- All domain values come from the closed vocabulary; no new states, categories, or severities.
- Findings and results are deterministic and ordered exactly as the design specifies.
- Do not weaken or skip kernel behavior: `internal/decision.Evaluate` is called once and its output is returned unmodified.

## File Structure

### Create

- `internal/evaluation/evaluate.go` — input validation, requirement universe, matching, fact derivation, normalization, findings, kernel composition.
- `internal/evaluation/evaluate_test.go` — fail-closed validation, universe cross-checks, matching, fact and normalization tables.
- `internal/evaluation/findings_test.go` — findings catalog, ordering, stability.
- `internal/evaluation/composition_test.go` — end-to-end decision surface and determinism.
- `internal/evaluation/security_regression_test.go` — adversarial fixtures: future-dated evidence, conflicting outcomes, cross-revision mixes, empty allowlists, self-asserted trust fields never consulted.

### Modify

- `docs/superpowers/specs/2026-11-01-m1-evaluation-design.md` — status only after verified implementation; no semantic rewrite during implementation.

## Review Focus

1. **No wall clock** — any `time.Now` in `internal/evaluation` is a defect; freshness must be computed only against `Input.EvaluatedAt`.
2. **Closed vocabulary discipline** — `MISSING`/`STALE`/`UNTRUSTED`/`VALID`/`INVALID` are assigned exactly per the design's normalization table; `STALE` must not be recorded for repository mismatches (impossible post-ingestion) or used as a euphemism for untrusted producers.
3. **Kernel primacy** — the composition must not pre-filter requirements, findings, or outcomes before `decision.Evaluate`, and must not post-process the returned verdict/decision.
4. **`ACCEPTED_WITH_RISK` unreachability** — no test or code path in this slice may reach `ACCEPTED_WITH_RISK`; the waiver branch belongs to the next slice.
5. **Ordering determinism** — results follow policy order; findings follow the (code, requirement, evidence) ordering; shuffling the input evidence array must not change findings order or content.

---

### Task 1: Package Skeleton, Input Validation, and the Pure-Function Contract

**Files:**
- Create: `internal/evaluation/evaluate.go`, `internal/evaluation/evaluate_test.go`

- [x] **Step 1: Write RED tests for the public surface and fail-closed validation**

Table tests over `Evaluate(Input)`: zero `EvaluatedAt` rejected; empty/missing digests rejected; non-advisory trust/authority pairs rejected; wrong contract/policy schema versions rejected; empty or non-canonical requirement IDs rejected; duplicate requirement IDs rejected (constructed directly, bypassing the loader); valid minimal input (one policy requirement, empty evidence set, fixed instant) accepted and returns one `MISSING` result.

- [x] **Step 2: Run `go test ./internal/evaluation` and observe RED** (package does not exist).

- [x] **Step 3: Implement the minimal types and validation** (`Input`, `Result`, `Evaluate` validation path only; return an empty-but-valid `Result` for the accepted case).

- [x] **Step 4: Run GREEN**, then `go vet ./internal/evaluation`.

- [x] **Step 5: Commit** `feat: add evaluation input validation`.

### Task 2: Requirement Universe and Matching

**Files:** modify `internal/evaluation/evaluate.go`, `internal/evaluation/evaluate_test.go`

- [x] **Step 1: Write RED tests** for §5 cross-checks: criterion referencing unknown requirement → `CONTRACT_UNKNOWN_REQUIREMENT` (blocking) while remaining requirements still evaluate; unreferenced policy requirement → `POLICY_REQUIREMENT_UNREFERENCED` (info) and still evaluated; criteria with empty requirements lists produce nothing; and §6 matching: exact case-sensitive type equality, one envelope satisfying two same-type requirements, no cross-type leakage.

- [x] **Step 2: Observe RED.**

- [x] **Step 3: Implement universe assembly and matching.**

- [x] **Step 4: Run GREEN** (focused tests + prior tasks).

- [x] **Step 5: Commit** `feat: assemble evaluation requirement universe`.

### Task 3: Per-Envelope Facts — Revision, Producer, Freshness

**Files:** modify `internal/evaluation/evaluate.go`, `internal/evaluation/evaluate_test.go`

- [x] **Step 1: Write RED table tests** for §7: `OTHER_REVISION` → stale fact; `FinishedAt` one nanosecond after `EvaluatedAt` → stale future; age exactly at `MaxAgeSeconds` → candidate, one nanosecond beyond → stale age; requirement without `MaxAgeSeconds` never stale by age; empty `AllowedProducerTypes` → untrusted for every producer; disallowed producer type → untrusted; precedence `STALE_REVISION` > `STALE_FUTURE` > `STALE_AGE` > `UNTRUSTED_PRODUCER` > candidate when several apply to one envelope.

- [x] **Step 2: Observe RED.**

- [x] **Step 3: Implement fact derivation** against `evidence.Loaded` fields only (`SubjectBinding`, `FinishedAt`, `Envelope.Producer.Type`) and policy fields; no envelope trust fields consulted.

- [x] **Step 4: Run GREEN.**

- [x] **Step 5: Commit** `feat: derive evaluation evidence facts`.

### Task 4: Requirement Normalization and Outcome Aggregation

**Files:** modify `internal/evaluation/evaluate.go`, `internal/evaluation/evaluate_test.go`

- [x] **Step 1: Write RED table tests** for §8: no matches → `MISSING` with empty `EvidenceIDs`; consistent candidates → `VALID` with aggregation (all `PASSED` → `PASSED`; any `ERROR` → `ERROR`; else any `FAILED` → `FAILED`); conflicting `PASSED`/`FAILED` candidates → `INVALID` with no outcome; stale-only → `STALE`; untrusted-only → `UNTRUSTED`; stale-over-untrusted precedence; `Waivable` copied from policy; `WaiverStatus` always `NOT_APPLICABLE`; `EvidenceIDs` sorted and deduplicated; results ordered by policy requirement order.

- [x] **Step 2: Observe RED.**

- [x] **Step 3: Implement normalization and aggregation.**

- [x] **Step 4: Run GREEN.**

- [x] **Step 5: Commit** `feat: normalize evaluation requirement results`.

### Task 5: Findings Catalog and Ordering

**Files:** create `internal/evaluation/findings_test.go`; modify `internal/evaluation/evaluate.go`

- [x] **Step 1: Write RED tests** for §9: each finding's code/category/severity/blocking/requirement/evidence-IDs binding; `EVIDENCE_MISSING` emitted once per requirement; `Message` bounded and deterministic; ordering by (code, requirement ID, joined evidence IDs) stable across input shuffles; no duplicate findings for one envelope; `POLICY_REQUIREMENT_UNREFERENCED` non-blocking and not duplicated per criterion.

- [x] **Step 2: Observe RED.**

- [x] **Step 3: Implement the findings stage.**

- [x] **Step 4: Run GREEN.**

- [x] **Step 5: Commit** `feat: emit evaluation findings`.

### Task 6: Kernel Composition, Security Regressions, Determinism

**Files:** create `internal/evaluation/composition_test.go`, `internal/evaluation/security_regression_test.go`; modify `internal/evaluation/evaluate.go`

- [x] **Step 1: Write RED composition tests** for §10's decision surface: all-passed → `PASSED`/`APPROVED`; established failure → `FAILED`/`REJECTED`; missing/stale/untrusted/conflict/error and blocking findings → `INDETERMINATE`/`BLOCKED`; assert `ACCEPTED_WITH_RISK` never occurs for any fixture in the package.

- [x] **Step 2: Observe RED** (composition not yet wired).

- [x] **Step 3: Wire `decision.Evaluate` exactly once**; return its verdict/decision unmodified.

- [x] **Step 4: Write RED security regressions:** future-dated evidence blocks; conflicting same-type evidence blocks with `EVIDENCE_CONFLICT`; cross-revision mix does not rescue a requirement to `VALID`; empty allowlist blocks; envelope trust fields (none exist on the type — assert evaluation compiles against the typed model and never constructs trust) and candidate-head sources cannot influence results beyond the typed inputs; repeated calls with deeply equal input are deeply equal; shuffled evidence input preserves findings order.

- [x] **Step 5: Observe RED, implement missing pieces, run GREEN.**

- [x] **Step 6: Run `go test -race ./internal/evaluation`** (CI provides race evidence on Linux; local Windows hosts without cgo record the limitation).

- [x] **Step 7: Commit** `feat: compose evaluation with decision kernel`.

### Task 7: Full Verification, Scope Audit, and Documentation Alignment

**Files:**
- Modify only if verified: `docs/superpowers/specs/2026-11-01-m1-evaluation-design.md`.

- [x] **Step 1: Run focused tests** `go test ./internal/evaluation ./internal/decision ./internal/evidence ./internal/contract ./internal/policy`.

- [x] **Step 2: Run required repository verification** (CONTRIBUTING.md): gofmt clean, `go vet ./...`, `go test -race ./...`, `go build -trimpath -o /tmp/assurectl ./cmd/assurectl`.

- [x] **Step 3: Scope audit:** `git diff --name-only main...HEAD` shows only `internal/evaluation/*` and the design doc; `go.mod`/`go.sum`, `cmd/`, `internal/decision/`, `internal/domain/`, `internal/evidence/`, `schemas/`, `examples/` untouched.

- [x] **Step 4: Trust-boundary re-read:** no `time.Now`, no filesystem/network/environment reads in `internal/evaluation`; no envelope field beyond the typed model consulted; no kernel post-processing; no `ACCEPTED_WITH_RISK` path; no TOCTOU or trust claims in comments.

- [x] **Step 5: Docs-only commit** flipping the design status to `Implemented in M1 evaluation slice` after all verification is green, plus plan checkbox updates.

- [ ] **Step 6: Push and require exact-head CI** (Go 1.26.x / 1.27.x matrix); record head SHA, run ID, and per-check results.

- [ ] **Step 7: Final review gate** against the design spec, the foundation design (§9, §10, §13, INV-005..012), ADR-0002, ADR-0004, ADR-0005, ADR-0007, and `docs/threat-model/v0.md`; scope additions (waivers, `verify`, receipts) are rejected as new-slice work.

## Exit Criteria

This plan is complete only when the resulting PR demonstrates all of the following:

- `Evaluate` is a pure function with fail-closed input validation and no clock, filesystem, network, or environment access;
- the requirement universe is exactly the policy requirements, with contract cross-checks surfaced as findings;
- matching is exact, case-sensitive evidence-type equality;
- stale-revision, stale-future, stale-age, and untrusted-producer facts follow the design's precedence, with the age boundary exact at the limit;
- normalization produces the closed-vocabulary states with the conflict → `INVALID` and stale-over-untrusted rules;
- findings match the catalog with stable deterministic ordering;
- the kernel is called once and returned unmodified; the reachable decision surface is `APPROVED`/`REJECTED`/`BLOCKED` only;
- repeated and shuffled-input evaluations are deterministic;
- no production package outside `internal/evaluation` changed; no new dependency;
- exact-head CI is green before merge readiness is claimed.
