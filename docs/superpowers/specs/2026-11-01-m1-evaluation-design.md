# M1 Requirement Evaluation and Findings Design

**Status:** Draft for review  
**Date:** 2026-11-01  
**Milestone:** M1 — Local advisory evaluation  
**Base commit:** `f795de63c37366105e9723ac701f8654326772d1`  
**Scope:** requirement-to-evidence matching, normalized requirement evidence states, findings, and deterministic verdict/decision composition only

## 1. Purpose

This slice implements the verification-engine stage of the core architecture (`docs/superpowers/specs/2026-09-02-assurectl-foundation-design.md` §13): it combines the typed local inputs that already exist — `contract.Loaded`, `policy.Loaded`, and `[]evidence.Loaded` — with an explicit evaluation time, and produces normalized per-requirement evidence states, observed outcomes, findings, and a final verdict/decision pair delegated to the existing deterministic kernel (`internal/decision`).

This slice closes the gap between ingestion and decision: after the evidence-ingestion slice, no production code consumes `evidence.Loaded`, `contract.Loaded`, or `policy.Loaded`, and `decision.Evaluate` has no production caller. After this slice, the only remaining M1 work is the waiver engine, the `assurectl verify` CLI surface, and the unsigned advisory receipt.

## 2. Non-scope

This slice does not implement:

- a waiver input file format, waiver records, or waiver application — the waiver engine is the next slice; every `RequirementResult.WaiverStatus` is `NOT_APPLICABLE` in this slice, so `ACCEPTED_WITH_RISK` is unreachable here by construction;
- the `assurectl verify` subcommand, CLI output, or exit-code mapping;
- receipt construction, receipt ingestion, or any of the semantic receipt invariants deferred by `docs/superpowers/plans/2026-09-02-m0-foundation-execution-notes.md`;
- policy-layer merging (ADR-0004 monotonic multi-layer resolution is M3 trusted-base work; this slice evaluates the single advisory workspace policy);
- producer identity verification, CI assertion, or trusted-base producer trust (ADR-0007): producer trust here is only the policy allowlist comparison defined in §7;
- any new schema, any network access, any new module dependency, and any change to `internal/decision`, `internal/domain`, `internal/evidence`, `internal/contract`, `internal/policy`, or `cmd/assurectl`.

## 3. New package

One new package: `internal/evaluation`.

Public surface (exact shape may tighten during implementation; semantics are normative):

```go
type Input struct {
    Contract    contract.Loaded
    Policy      policy.Loaded
    Evidence    []evidence.Loaded
    EvaluatedAt time.Time
}

type Result struct {
    Requirements []domain.RequirementResult
    Findings     []domain.Finding
    Verdict      domain.EvaluationResult
}

func Evaluate(input Input) (Result, error)
```

`Evaluate` is a pure function of `Input`. It never reads the wall clock, the filesystem, the network, or the environment; the caller supplies the evaluation instant (the future CLI adapter will pass its own clock reading, and tests pass fixed instants). Repeated calls with deeply equal inputs return deeply equal outputs.

## 4. Inputs and fail-closed validation

`Evaluate` rejects (returns error, produces no result) when:

- `Contract.Digest` or `Policy.Digest` is empty, or either `TrustStatus`/`AuthorityBasis` pair is not the advisory workspace pair produced by the loaders (`UNTRUSTED` / `ADVISORY_WORKSPACE`); this slice evaluates only loader-shaped inputs, not ad-hoc constructions;
- `Contract.Contract.SchemaVersion` is not `assurectl/verification-contract/v0` or `Policy.Policy.SchemaVersion` is not `assurectl/policy/v0`;
- a policy requirement ID or acceptance-criterion requirement reference is empty or not a canonical identifier;
- two policy requirements share an ID (the loader already rejects duplicates; re-checked here fail-closed);
- `EvaluatedAt` is the zero `time.Time`.

A zero evidence set is valid input (every requirement evaluates to `MISSING`); evidence entries are consumed exactly as ingestion produced them — `Loaded.SubjectBinding` is `EXACT` or `OTHER_REVISION`, digests are verified, and the set is duplicate-free.

## 5. Requirement universe

The required verifications are the policy requirements. The contract's acceptance criteria organize requirements; they may add constraints but never weaken policy (ADR-0004). Cross-checks:

- Every acceptance-criterion reference that does not resolve to a policy requirement ID produces a blocking `CONTRACT_UNKNOWN_REQUIREMENT` finding. The requirement universe still evaluates; the finding blocks the final decision through the kernel's blocking-finding precedence.
- A policy requirement referenced by no acceptance criterion is still evaluated and produces an `INFO` finding (`POLICY_REQUIREMENT_UNREFERENCED`). Policy requirements are authoritative on their own; absence from the contract's criteria is not silently ignored, but it is not blocking.
- Criteria with empty `requirements` lists are valid (a criterion may be manually verified); no finding.

No requirement is ever created that policy does not define, and no policy requirement is dropped.

## 6. Matching

A policy requirement matches every evidence envelope whose `Envelope.Type` equals the requirement's `EvidenceType` by exact, case-sensitive string comparison. No trimming, case folding, or aliasing: evidence producers emit a bounded type string and policy authors constrain exactly that string. An envelope matches at most the set of requirements whose `EvidenceType` it equals; one envelope may satisfy multiple requirements with the same evidence type.

## 7. Per-envelope evaluation facts

Each matching envelope contributes one evaluation fact derived from data ingestion already established, plus the explicit evaluation time:

| Condition (first match wins) | Fact |
|---|---|
| `SubjectBinding == OTHER_REVISION` | `STALE_REVISION` (INV-007: evidence for another revision is stale; a waiver cannot fix it) |
| `FinishedAt.After(EvaluatedAt)` | `STALE_FUTURE` (observation completes after the evaluation instant — outside the permitted validity window; also covers fabricated future timestamps under clock skew) |
| requirement has `MaxAgeSeconds` and `EvaluatedAt.Sub(FinishedAt) > MaxAgeSeconds` | `STALE_AGE` |
| producer type not in requirement's `AllowedProducerTypes` | `UNTRUSTED_PRODUCER` |
| otherwise | `CANDIDATE` |

`FinishedAt` is the envelope's parsed invocation completion instant (`Loaded.FinishedAt`); age arithmetic uses exact `time.Duration` comparison against the `big.Int` bound converted once per requirement. An empty `AllowedProducerTypes` permits no producer type: every matching envelope is `UNTRUSTED_PRODUCER` (fail closed; empty is never read as "any").

The state `INVALID` is never assigned to an envelope in this slice: ingestion already failed closed on malformed or integrity-failing input, and §8 reserves `INVALID` for a requirement-level inconsistency.

## 8. Requirement normalization

Per requirement, over its matching envelopes' facts:

| Situation | `RequirementResult` |
|---|---|
| no matching envelope | `EvidenceState: MISSING`, no outcome, empty `EvidenceIDs` |
| at least one `CANDIDATE`, candidate outcomes consistent | `EvidenceState: VALID`, aggregated outcome, `EvidenceIDs` = all candidate envelope IDs |
| at least one `CANDIDATE`, candidate outcomes conflicting (both `PASSED` and `FAILED` present) | `EvidenceState: INVALID` (internally inconsistent evidence set, design §9), no outcome, `EvidenceIDs` = all candidate envelope IDs |
| no candidate, at least one `STALE_*` | `EvidenceState: STALE`, no outcome, `EvidenceIDs` = all stale envelope IDs |
| no candidate, no stale, at least one `UNTRUSTED_PRODUCER` | `EvidenceState: UNTRUSTED`, no outcome, `EvidenceIDs` = all untrusted envelope IDs |

Outcome aggregation over consistent candidates: any `ERROR` → `ERROR`; otherwise any `FAILED` → `FAILED`; otherwise `PASSED`. `STALE` takes precedence over `UNTRUSTED` when both facts exist, because revision binding is the more specific diagnosis; both sets still receive findings. These two precedence choices (conflict → `INVALID`; `STALE` > `UNTRUSTED`) are deterministic, fail-closed, and stay inside the closed vocabulary — no protocol or schema change, so no ADR is required.

`Waivable` is copied from the policy requirement; `WaiverStatus` is `NOT_APPLICABLE` for every result (§2). `EvidenceIDs` are sorted and deduplicated. A requirement result is emitted for every policy requirement, even when `MISSING` — the receipt schema requires at least one requirement result and `MISSING` must be representable without fabricated evidence.

## 9. Findings

Findings are deterministic facts, not prose; codes are stable and machine-consumed. Emitted findings:

| Code | Category | Severity | Blocking | When |
|---|---|---|---|---|
| `EVIDENCE_MISSING` | `EVIDENCE` | `ERROR` | yes | requirement has no matching envelope |
| `EVIDENCE_STALE_REVISION` | `EVIDENCE` | `ERROR` | yes | an envelope binds `OTHER_REVISION` |
| `EVIDENCE_STALE_FUTURE` | `EVIDENCE` | `ERROR` | yes | an envelope finishes after `EvaluatedAt` |
| `EVIDENCE_STALE_AGE` | `EVIDENCE` | `ERROR` | yes | an envelope exceeds `max_age_seconds` |
| `EVIDENCE_UNTRUSTED_PRODUCER` | `EVIDENCE` | `ERROR` | yes | an envelope's producer type is not allowed |
| `EVIDENCE_CONFLICT` | `EVIDENCE` | `ERROR` | yes | candidate outcomes conflict for one requirement |
| `CONTRACT_UNKNOWN_REQUIREMENT` | `CONTRACT` | `ERROR` | yes | criterion references an unknown requirement ID |
| `POLICY_REQUIREMENT_UNREFERENCED` | `POLICY` | `INFO` | no | policy requirement referenced by no criterion |

Every finding carries the requirement ID; `EVIDENCE_*` findings additionally carry the contributing envelope IDs. `Message` is bounded, deterministic text. Findings are ordered by (code, requirement ID, joined evidence IDs) so output is byte-stable across runs. `EVIDENCE_MISSING` is emitted once per missing requirement, not per envelope (there is no envelope).

The kernel remains the sole authority on verdict and decision: blocking findings above make requirements indeterminate through their evidence states, and additionally feed `decision.Evaluate`, whose blocking-finding precedence produces `BLOCKED` (ADR-0002, ADR-0005). This slice does not second-guess, filter, or reorder kernel output.

## 10. Composition and determinism

`Evaluate` composes the normalized results and findings through `internal/decision.Evaluate` exactly once. The expected decision surface of this slice (with waivers absent) is therefore:

- every requirement `VALID`/`PASSED` and no blocking finding → `PASSED` / `APPROVED`;
- any established `FAILED` with everything else established → `FAILED` / `REJECTED`;
- any `MISSING`/`STALE`/`UNTRUSTED`/`INVALID`/`ERROR` or blocking finding → `INDETERMINATE` / `BLOCKED`.

`ACCEPTED_WITH_RISK` is unreachable in this slice (§2); the kernel's acceptance-with-risk branch is exercised by the next slice's waiver engine, not by loosening anything here.

Determinism invariants (INV-010): no `time.Now()`, no map-iteration-dependent output, no randomness; requirement results are ordered by policy requirement order (policy `requirements` array order, which is deterministic and validated unique), findings by §9's ordering; repeated evaluation of deeply equal input is deeply equal output.

## 11. Testing strategy

Test-first throughout (observe RED before each GREEN):

1. fail-closed caller validation table (§4);
2. requirement-universe cross-checks (§5) including unknown references and unreferenced requirements;
3. matching table: exact-type matching, multi-requirement sharing, case sensitivity (§6);
4. per-envelope fact table: stale revision, future observation, age boundary (exactly at the limit is fresh, one nanosecond beyond is stale), empty allowlist (§7);
5. normalization table: missing, valid pass/fail/error, conflict, stale-over-untrusted precedence (§8);
6. findings catalog: codes, categories, severities, blocking flags, ordering stability (§9);
7. end-to-end composition against the kernel, including the expected decision surface (§10);
8. determinism: repeated calls deeply equal, output stable under evidence-array shuffling beyond documented order (findings still sorted; requirement order fixed by policy);
9. adversarial fixtures: future-dated evidence, conflicting same-type evidence, cross-revision mix, self-asserted trust fields never consulted (ingestion already rejects them; evaluation never reads any trust field from envelopes).

## 12. Boundary to the next slice

The next (final) M1 slice adds: the waiver engine (typed waiver application with exact failed-requirement targeting, scope, expiry, and policy `waivable` permission — enabling `ACCEPTED_WITH_RISK` through the kernel), the `assurectl verify` subcommand composing `gitsubject.Resolve` → loaders → `evidence.LoadLocal` → `evaluation.Evaluate`, CLI output and exit codes, and the unsigned advisory receipt builder implementing the semantic invariants deferred by the M0 execution notes (duplicate requirement IDs, `evidence_ids` ↔ top-level `evidence` membership and digest matching, waiver target matching, semantic decision derivation, policy-provenance eligibility, RFC3339 semantic parsing). This slice's `Result` is designed to map directly onto the receipt's requirement results, findings, and verdict/decision fields.
