# M1 Verify CLI and Advisory Receipt Design

**Status:** Draft for review  
**Date:** 2026-11-02  
**Milestone:** M1 — Local advisory evaluation  
**Base commit:** `2fb1b2652aca06e60543e7d21327fa9dd45cad0b`  
**Scope:** `assurectl verify` CLI, the advisory receipt builder with semantic fail-closed validation, and nothing else

## 1. Purpose

This slice completes M1 ("Local advisory evaluation for exact Git revisions", foundation design §16): it wires the existing pipeline stages — `gitsubject.Resolve`, `contract.LoadLocal`, `policy.LoadLocal`, `evidence.LoadLocal`, `evaluation.Evaluate` — into the first production CLI surface, and emits the unsigned advisory receipt the v0 schema draft (`schemas/receipt.v0.schema.json`) describes. The receipt builder implements the semantic checks the M0 execution notes deferred ("Non-enforcing M0 receipt model"): construction is fail-closed, and no receipt is emitted unless every check passes.

## 2. Boundary revision: waiver inputs move to M2

The evaluation-slice design (§12 of `2026-11-01-m1-evaluation-design.md`) anticipated a waiver engine in this slice. This design supersedes that wording, for two reasons:

1. **M1's milestone scope (§16) contains no waiver input.** The M1 row is "Exact Git subject resolution, typed local inputs, evidence validation, findings, and unsigned advisory receipt". A waiver engine without a waiver input source is dead code.
2. **Inventing a local waiver file format now would be a security-sensitive shortcut.** A `.assurectl/` waiver file is authored by the same workspace it authorizes — the exact self-approval pattern INV-003/INV-004 and ADR-0007 exist to prevent. Meaningful waiver authorization needs an external context (CI-asserted or trusted-base) that only exists from M2/M3, and a new input format is a protocol surface requiring its own ADR.

Consequently, in this slice: `receipt.waivers` is always the empty array, `ACCEPTED_WITH_RISK` stays unreachable through the CLI, and the M0 notes' waiver semantic invariants (exact failed-requirement-to-waiver-record target matching, waiver authority/scope/timing validation) are implemented in the receipt builder so they are already fail-closed when M2 introduces waiver records. `internal/decision`'s waiver branch remains untouched and fully tested.

## 3. Non-scope

- Waiver input files, waiver parsing, or any waiver application logic (§2);
- signing, in-toto/DSSE, replay protection (M4);
- trusted CI evidence envelopes, GitHub Actions adapter (M2);
- trusted-base policy resolution, org policy layers (M3) — `policy.resolved_policy.sources` always has exactly one `project` layer (the workspace policy) plus the built-in protocol baseline layer conceptually present; this slice records the workspace policy source and the `protocol_baseline` layer with a `BUILTIN`-style source string and no `source_revision`;
- any change to `internal/gitsubject`, `internal/contract`, `internal/policy`, `internal/evidence`, `internal/evaluation`, `internal/decision`, `internal/domain`, or `schemas/*.json`.

## 4. New packages

```go
internal/receipt   // typed v0 receipt, deterministic builder, semantic validation
internal/verify    // orchestration: resolve → load → evaluate → build
```

`cmd/assurectl` gains the `verify` subcommand and only maps results to output channels and exit codes (foundation design §13: the CLI makes no domain decisions).

## 5. `assurectl verify` CLI surface

```
assurectl verify --base <git-ref> [--head <git-ref>] [--repository <uri>]
```

- `--base` is required and must be a ref the subject resolver accepts (HEAD, full 40/64-hex OID, or fully-qualified `refs/...`): the change-set boundary is caller intent and is never guessed. Missing `--base` is a usage error (exit 64).
- `--head` defaults to `HEAD`.
- `--repository` optionally pins the canonical repository identity; when omitted the resolver uses `remote.origin.url` with its `local://sha256/...` fallback and reports the local-only fact as a finding.
- Unknown flags, missing flag values, and positional arguments are usage errors (exit 64). Flag values are never echoed into error messages beyond what the resolver itself already reports (no credential echo).
- Output: the receipt JSON (compact, deterministic) goes to stdout exactly once on success; a one-line human summary (`verdict`, `decision`, receipt digest) goes to stderr. Diagnostics go to stderr.
- Exit codes: `0` decision `APPROVED`; `1` decision `REJECTED`; `2` decision `BLOCKED` or any evaluation/build error (fail closed — indistinguishable by design); `64` usage.
- The evaluation instant is the CLI's own wall-clock reading (UTC, truncated to whole seconds) passed explicitly into evaluation and receipt building; nothing downstream calls `time.Now()`.

## 6. Orchestration (`internal/verify`)

`Run(ctx, dir, Options) (Result, error)` composes, in order, failing closed at the first error:

1. `gitsubject.Resolve(ctx, dir, Options{RepositoryURI, BaseRef, HeadRef})` — exact subject; no clean/dirty weakening (INV-002);
2. `contract.LoadLocal(dir)` then `policy.LoadLocal(dir)`;
3. `evidence.LoadLocal(dir, resolved subject)` — repository mismatch fails here per the ingestion contract;
4. `evaluation.Evaluate(Input{Contract, Policy, Evidence, EvaluatedAt})`;
5. `receipt.Build(...)` with semantic validation (§7).

Two facts the resolver reports become findings so they stay visible without weakening anything: a dirty worktree (`SUBJECT_DIRTY_WORKTREE`, category `SUBJECT`, severity `WARNING`, non-blocking) and a local-only repository identity (`SUBJECT_LOCAL_ONLY`, same shape). M1 receipts are `ADVISORY` regardless; these findings are diagnostic, not gate changes.

An empty policy (zero requirements) fails closed with an error and no receipt: the v0 receipt schema requires at least one requirement result, and representing "nothing was required" as a completed evaluation would be a fabricated approval.

## 7. Receipt builder and semantic validation (`internal/receipt`)

`Build` maps the evaluation result, loaded inputs, resolved subject, and verifier identity onto the v0 receipt structure with JSON tags matching the schema draft field names exactly. The receipt is built from typed structs and marshaled deterministically (fixed field order from the struct, arrays in stable order: `evidence` sorted by id, `findings` as produced by the evaluation's deterministic ordering, `requirement_results` in policy order, `waivers` empty).

Before any receipt is returned, `Build` runs the semantic checks the M0 notes deferred — every check fails the build (error, no receipt), for **every** receipt including `BLOCKED`:

1. `requirement_results[].requirement_id` values are unique (schema cannot enforce object-property uniqueness);
2. every `requirement_results[].evidence_ids[]` entry refers to an existing top-level `evidence[].id`, and each `evidence[].digest` equals the referenced envelope's canonical digest (schema cannot compare identifiers across arrays);
3. if any requirement result is `VALID`, the top-level `evidence` set is non-empty;
4. every waiver record's target (were any present) resolves exactly to a failed requirement result; waiver records with `status: VALID` are permitted only on waivable `VALID/FAILED` results — vacuous while `waivers` is always empty, implemented so the invariant cannot be forgotten;
5. the verdict/decision pair is re-derived by calling `internal/decision.Evaluate` on the receipt's own requirement results and findings; a mismatch between the re-derived pair and the pair destined for the receipt is a build failure — the receipt records a semantically derived decision, never a supplied one (M0 notes line 38–39);
6. decisions other than `BLOCKED` carry only non-blocking findings;
7. `authority` is `ADVISORY`; a non-advisory authority value is rejected (M1 cannot produce `CI_ASSERTED`/`AUTHORITATIVE` receipts);
8. policy provenance eligibility: the recorded `policy` block marks the workspace source `UNTRUSTED`/`ADVISORY_WORKSPACE` and the `protocol_baseline` layer as builtin; the ineligible-for-authoritative rule from the M0 notes is structurally satisfied because `ADVISORY` receipts make no authority claim;
9. `evaluated_at` (and any timestamp written into the receipt) parses as RFC3339 and the evidence envelope instants embedded in the receipt respect `started_at <= finished_at` — ingestion already enforced this; the builder re-checks rather than trusting (M0 notes line 48);
10. the verifier block is complete: name `assurectl`, version from `internal/buildinfo`, protocol version `v0`.

The builder itself never mutates evaluation facts: findings, requirement results, verdict, and decision are copied, not recomputed (INV-008 discipline; only the §7.5 re-derivation runs, and it compares rather than replaces).

## 8. Determinism

Same workspace state, same base/head refs, same evaluation instant, same evaluator version ⇒ byte-identical receipt. No map iteration reaches output; every array has a documented order (§7). The receipt digest recorded on stderr is the SHA-256 of the exact emitted bytes.

## 9. Testing strategy

Test-first throughout (RED before GREEN per task):

1. receipt builder mapping table: every schema-required field present, JSON tags match the draft, advisory defaults;
2. semantic validation table: one test per §7 check, including the vacuous waiver-target check exercised with a hand-built record that must be rejected;
3. decision re-derivation: tampered verdict/decision pairs are rejected;
4. determinism: repeated builds byte-identical; shuffled input evidence changes nothing;
5. orchestrator integration tests with temporary Git repositories (same pattern as `internal/gitsubject` tests): clean worktree happy path, dirty worktree finding, local-only identity finding, evidence repository mismatch fails, empty policy fails closed, pipeline error propagation;
6. CLI tests: flag parsing (missing `--base`, unknown flag, trailing args → 64), exit-code mapping (0/1/2), output channels (receipt on stdout, summary/diagnostics on stderr);
7. no network, no new dependency, no `time.Now` outside the CLI adapter — regression greps in the plan's scope audit.

## 10. Boundary to M2

M2 ("Trusted CI evidence envelopes, protected workflow guidance, and reference GitHub Action") inherits: waiver input format + ADR (§2), CI-asserted authority receipts, trusted producer identity, and the protected-workflow guidance. The receipt built here is the wire format those layers will assert with signatures in M4.
