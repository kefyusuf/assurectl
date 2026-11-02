# M2 Waiver Records and Risk Acceptance Design

**Status:** Draft for review  
**Date:** 2026-11-02  
**Milestone:** M2 — Trusted CI evidence (slice 1 of the milestone)  
**Base commit:** `91cc1fa135f7477fad90eabde8997bcc47af6ee2`  
**Scope:** waiver record input, the waiver application engine, receipt waiver emission, and the `--waivers` CLI flag only

## 1. Purpose

M1 left `receipt.waivers` permanently empty and `ACCEPTED_WITH_RISK` unreachable: waiver inputs were deferred because a local waiver file is a security-sensitive protocol surface requiring its own ADR (`2026-11-02-m1-verify-receipt-design.md` §2). This slice introduces that surface deliberately: a typed waiver-record input, an application engine inside `internal/evaluation`, receipt emission of applied and rejected records, and the ADR that bounds what waiver inputs may never do. The M0 execution notes' waiver invariants, already enforced by the receipt builder, stop being vacuous.

## 2. ADR-0008 (authored in this slice): waiver inputs are advisory, explicit, and never authoritative

- Waiver records are risk acceptance, never verification: applying one cannot change evidence state, observed outcome, findings, or the verification verdict (INV-008; ADR-0002).
- A waiver applies only when the policy marks the failed requirement `waivable`; no input can waive a non-waivable requirement (foundation design §10; ADR-0004's monotonic rule "waivable → non-waivable only" holds in every layer).
- The v0 waiver input is an explicit CLI opt-in (`--waivers <path>`), never auto-discovered: a repository cannot silently ship its own risk acceptance.
- Waiver records enter advisory receipts as recorded facts with their digests and approver authority. When M3 introduces authoritative receipts, waiver provenance becomes subject to the same source-eligibility rules as policy provenance; this slice keeps authority `ADVISORY` only, so no authoritative claim arises.
- No custom cryptography, no self-issued trust: the approver authority field records who accepted the risk, and the advisory receipt makes no authentication claim about it.

## 3. Input format

New schema draft `schemas/waiver-record.v0.schema.json` and new file `internal/waiver`:

- `assurectl verify --waivers <path>` loads a JSON array of waiver records from a workspace-relative path (1 MiB workspace-bounded read, `localinput`, strict decoding — same discipline as the contract and policy loaders).
- Each record matches the receipt's `waiver_record` definition exactly: `id`, `target {kind, id}`, `scope {repository_uri, work_unit_id, head_revision?}`, `reason`, `accepted_risk`, `approver {identity, authority}`, `issued_at`, `expires_at`, `status`, `digest`.
- `target.kind` is `REQUIREMENT` (the `FINDING` kind stays reserved; supplying it fails the load fail-closed).
- `status` is a resolved validity supplied by the caller context; the loader accepts `VALID` or `INVALID` and the engine re-derives applicability — a caller-supplied `VALID` is a claim, not a decision (INV-005 discipline for status fields).
- The loader computes the canonical record digest (JSON-marshal of the validated value, `inputmeta.SHA256`) and rejects duplicates by `id`, non-canonical identifiers, unparseable RFC3339 timestamps, and `expires_at` before `issued_at`.
- No waiver file (or no flag) means an empty waiver set — the M1 behavior.

## 4. Engine semantics (`internal/evaluation`)

`evaluation.Input` gains `Waivers []waiver.Record`. Application is deterministic and per requirement:

1. Only an established failure is a target: requirement result `VALID` + `FAILED` + policy `waivable`.
2. A record applies when: `target.kind == REQUIREMENT`, `target.id` equals the requirement ID, `scope.repository_uri` equals the subject's canonical repository URI, `scope.work_unit_id` equals the contract work-unit ID, an optional `scope.head_revision` equals the subject's head revision, `issued_at <= evaluated_at < expires_at`, and the record status is `VALID`.
3. An applicable record sets `RequirementResult.WaiverStatus = VALID` and emits one `WAIVER_APPLIED` finding (category `WAIVER`, severity `INFO`, non-blocking, evidence IDs empty, message carrying the waiver ID and requirement ID). The first applicable record in input order wins; later duplicates of the same target emit `WAIVER_SUPERSEDED`.
4. A `VALID` record that fails any applicability condition emits `WAIVER_REJECTED_<REASON>` (one deterministic code per reason: `TARGET`, `SCOPE`, `EXPIRED`, `TIMING`) with severity `WARNING`, non-blocking, and the record's digest is still emitted in the receipt. A rejected waiver never blocks completion: the failure it targeted stays visible and drives `REJECTED` through the kernel.
5. Waiver application never mutates evidence states, outcomes, other findings, or the verdict; only `WaiverStatus` and the decision (via the existing kernel branch) reflect it.
6. `ACCEPTED_WITH_RISK` becomes reachable: every failure validly waived with no indeterminate requirements and no blocking findings.

Rejected-waiver findings are non-blocking by design choice documented here: the design's "malformed claim fails closed" is satisfied by refusing application (the failure stays a failure), not by converting the run to `BLOCKED`, which would misreport a known technical outcome as indeterminate.

## 5. Receipt integration

`receipt.Build` copies `evaluation.Result`'s waiver records into `receipt.waivers` (applied and rejected alike, input order), each carrying its loader-computed digest. The existing builder checks become live: exact failed-requirement targeting, `VALID` only on waivable `VALID/FAILED` results, scope matching against the receipt subject and work unit, RFC3339 re-checks. A receipt whose decision is `ACCEPTED_WITH_RISK` must contain at least one `VALID` waiver record (schema conditional, now satisfied by construction and re-verified by the re-derivation check).

## 6. Determinism and fail-closed rules

- No `time.Now()` anywhere; the engine compares against the explicit `EvaluatedAt`.
- Waiver findings are emitted in input order per requirement, then the existing (code, requirement, evidence) sort orders the final findings list.
- Loader errors and engine ambiguity return errors or rejections per §3–§4; no partial sets, no guessed applicability.

## 7. Testing strategy

Test-first throughout: loader table (valid record, duplicate IDs, reserved target kind, bad timestamps, expiry before issuance, digest determinism); engine table (apply, supersede, each rejection reason, non-waivable target, non-failed target, no-op on PASSED requirements); end-to-end `ACCEPTED_WITH_RISK` through evaluation + receipt + kernel; receipt emission of applied and rejected records; CLI `--waivers` happy path, missing file, and malformed file; determinism (shuffled waiver input changes nothing byte-wise except documented finding order).

## 8. Boundary to the next M2 slices

Slice 2 extends evidence with CI execution context (producer workflow identity, CI-asserted trust facts per foundation §11 Tier 1 and the threat model's CI/OIDC producer-identity protections); slice 3 adds the reference GitHub Action and protected-workflow guidance. Authoritative waiver provenance and trusted-base policy resolution remain M3.
