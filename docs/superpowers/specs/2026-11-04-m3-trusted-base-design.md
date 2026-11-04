# M3 Trusted-Base Policy and Contract Resolution Design

**Status:** Draft for review  
**Date:** 2026-11-04  
**Milestone:** M3 — GitHub App and merge authority (slice 1 of the milestone)  
**Base commit:** `15ade46128fac73dee41b894b08597e9ae3e436d`  
**Scope:** resolving the project policy and verification contract from the base revision, trusted-base receipt provenance, and the `--trusted-base` CLI mode only

## 1. Purpose

ADR-0004's core rule — "authoritative evaluation resolves project policy from the trusted base revision" — has been protocol language since M0; every evaluation so far has still read `.assurectl/` inputs from the candidate workspace (advisory by source context, ADR-0007). This slice makes the rule executable: in trusted-base mode, the evaluator reads the project policy **and the verification contract** from the subject's resolved base revision via Git, marks them `TRUSTED` because of that source context, and emits `AUTHORITATIVE` receipts whose provenance markers resolve to the subject's actual base object — the M0 execution notes' deferred invariant.

This is the semantic prerequisite for the rest of M3 (source-pinned required checks, merge authority, and the eventual flip of this repository's dogfooding gate to blocking). The hosted GitHub App itself is deliberately **not** in this slice: it is a hosted component on the managed boundary (foundation §17) and gets its own design after the evaluator can express authoritative evaluation offline.

## 2. Claim boundary

`AUTHORITATIVE` receipts produced by this slice still assert local facts only: the policy and contract came from the recorded base revision of the subject, via the local Git object store, with digests recorded. Nothing validates *which* base the wider world trusts (branch protection, required checks, App identity) — that is M3 slices 2+. The claim is therefore narrower than "merge-grade": it is "the gate the candidate is evaluated under is exactly the one its base revision carries, and the candidate cannot weaken it."

## 3. Closed-vocabulary extensions

- `inputmeta.TrustStatus` gains `TRUSTED` (besides `UNTRUSTED`).
- `inputmeta.AuthorityBasis` gains `TRUSTED_BASE` (besides `ADVISORY_WORKSPACE`).
- `receipt` validation: `AUTHORITATIVE` becomes producible — and requires `TRUSTED` contract and policy inputs (the receipt schema already conditions this) plus exactly one project-layer source carrying `revision_binding: SUBJECT_BASE` and no `source_revision` (M0 notes line 30/45; the ineligible-provenance rule).
- No other vocabulary or schema change; the receipt schema is untouched.

## 4. Base-blob reader (`internal/gitsubject` seam)

Export one narrow seam from the resolver's existing vetted machinery:

```go
func ReadBlob(ctx context.Context, worktree, revision, path string) ([]byte, error)
```

- Uses the same `trustedGitExecutable`, sanitized environment, and argument-list execution as `runGit` (no new execution pattern; the Mimosa flags on pre-existing `exec.CommandContext` usage are unchanged).
- `path` must be `.assurectl/policy.json` or `.assurectl/contract.json` — an allowlist, not free-form; any other path is rejected before Git runs.
- `revision` must already be a resolved object ID (the subject's base revision; never a free ref from the caller).
- Missing blob at the base revision is an explicit, error-grade fact: a base without a contract or policy cannot produce authoritative evaluation (fail closed; advisory mode is unaffected).

## 5. Loaders

`policy.LoadBase(ctx, root, revision)` and `contract.LoadBase(ctx, root, revision)` mirror the workspace loaders' strict decoding and validation exactly — same schema checks, same canonical digest discipline — with three differences: the bytes come from `ReadBlob`, `Source` is `base:<revision>:<path>`, and `TrustStatus`/`AuthorityBasis` are `TRUSTED`/`TRUSTED_BASE`. No fallback to the workspace file: trusted-base mode reads only from the base.

Layer assembly for the receipt stays as built in M1 (builtin protocol baseline + project layer); in trusted-base mode the project layer additionally carries `revision_binding: SUBJECT_BASE`.

## 6. Orchestration and CLI

`verify.Options` gains `TrustedBase bool`; the CLI gains `--trusted-base`:

- resolution runs unchanged; the base revision is the subject's already-resolved `BaseRevision` object;
- policy and contract load from the base; evidence, waivers, evaluation, and findings are unchanged (the evaluated change still binds the head);
- the receipt is built with authority `AUTHORITATIVE`;
- exit codes and output channels are unchanged;
- advisory mode (no flag) is byte-identical to today, including the dogfooding CI loop.

Combination rule: `--trusted-base` composes with `--ci` and `--waivers` (waiver provenance rules from ADR-0008 still bound: this slice produces authoritative receipts whose waiver set remains subject to the same fail-closed checks; authoritative *waiver provenance beyond explicit files* is future work — until then, an authoritative receipt with a non-empty waiver set is accepted only if every record carries a resolved `VALID` status with matching scope, exactly as enforced today).

## 7. Receipt builder validation

Beyond the existing semantic checks, in `AUTHORITATIVE` mode the builder additionally verifies:

1. both `contract` and `policy` blocks record `trust_status: TRUSTED`;
2. the policy's project layer carries `revision_binding: SUBJECT_BASE` and no `source_revision`;
3. the orchestrator's provenance is real: `BuildInput` gains the loaded-from-base markers (the loaders are the only producers of `TRUSTED` inputs, and the builder rejects hand-constructed `TRUSTED` inputs whose source string is not a `base:<40-or-64-hex>:<path>` form);
4. `BLOCKED` authoritative receipts (indeterminate inputs) remain possible and skip no check.

## 8. Testing strategy

Test-first throughout: `ReadBlob` table (happy path, missing blob, non-allowlisted path, unresolved revision, symlinked worktree); `LoadBase` tests mirroring the workspace loader tables plus trust markers; verify integration tests with temporary repositories (candidate workspace policy differs from base policy → authoritative receipt reflects the BASE policy; missing base policy → error; advisory mode byte-identical); receipt tests (AUTHORITATIVE accepted with trusted provenance, rejected without, `SUBJECT_BASE` marker enforced); determinism (same base → byte-identical receipts); both-toolchain gofmt before push.

## 9. Boundary to the next M3 slices

Slice 2: source-pinned required check mechanics and the flip of this repository's dogfooding gate to blocking with `--trusted-base` receipts (a maintainer decision recorded as such). Slice 3: the GitHub App design itself — a hosted component whose OSS/managed split (foundation §17) gets its own spec, including OIDC validation of the CI context and replay protection handoff to M4.
