# M3 Gate Flip and Required-Check Pinning Design

**Status:** Implemented in M3 gate-flip slice  
**Date:** 2026-11-05  
**Milestone:** M3 — GitHub App and merge authority (slice 2 of the milestone)  
**Base commit:** `cb2af1cf62fdfee76c7dbd827148e33c2cc8e3c9`  
**Scope:** flipping this repository's dogfooding verify job to blocking under `--trusted-base`, the action's trusted-base input, and the documented limits of workflow-level source pinning

## 1. Purpose

M3 slice 1 made `--trusted-base` executable: the gate documents now come from the subject's base revision, and `AUTHORITATIVE` receipts carry resolvable provenance. The dogfooding verify job has been non-blocking since M2 by recorded decision, pending exactly this capability. This slice flips it: this repository's own PRs are verified under the base's gate with `--trusted-base --ci`, the job becomes required-grade (blocking), and the action gains the input to do the same for adopters.

## 2. What blocking means here — and its honest limit

- **Blocking:** the `Verify with own evidence` job loses `continue-on-error`; a REJECTED/BLOCKED decision or any pipeline error fails the PR's CI. Branch protection (the repository-settings rule that makes the check *required*) is a maintainer action in GitHub settings, listed step-by-step in the guidance and performed by the maintainer, not by tooling in this slice.
- **Honest limit (documented, not hidden):** for `pull_request` events GitHub runs the workflow file from the merge ref, so a PR can still edit `.github/workflows/ci.yml` itself. Two facts bound that window: (1) the verifier binary is pinned — the action installs `assurectl` at an immutable module revision regardless of workflow edits; (2) the receipt records the base's gate, so a weakened *policy* has no effect. The remaining exposure — a PR disabling the verify job — is closed by making the check *required* in branch protection and, definitively, by the GitHub App (slice 3), whose checks are posted from outside the repository. The guidance states all of this.

## 3. Changes

1. **`action.yml`:** new boolean input `trusted-base` (default `false`) passed through as `--trusted-base`; the shape test pins it (RED first).
2. **`.github/workflows/ci.yml`:** the `verify` job drops `continue-on-error`, adds `trusted-base: true`, and keeps the summary step. The evidence-upload `if: always()` stays so failed suites still upload diagnostics.
3. **`docs/guides/protected-workflows.md`:** a "required check" section: exact branch-protection steps, the workflow-edit caveat above and its closure path, and what an AUTHORITATIVE receipt does and does not claim.
4. **No evaluator changes, no schema changes, no new ADR** — the capability and its boundaries were established in slices 1–2 (ADR-0009, ci-context design); this slice adopts them.

## 4. Expected dogfooding behavior after the flip

For a PR against `main`: base revision = the PR base object (carries `.assurectl/policy.json` requiring `ci-tests` from `github-actions` producers); the CI test job emits evidence binding `GITHUB_SHA`; the verify job runs `--trusted-base --ci` with the PR base object id as `--base`. Expected receipt: `authority: AUTHORITATIVE`, decision `APPROVED` when the suite passes, `REJECTED` when it fails — under the base's policy even if the candidate edits its own policy file. A PR that deletes `.assurectl/` documents from its branch does not affect the gate (the base still carries them).

## 5. Testing strategy

1. Action shape test extended for the `trusted-base` input (RED first);
2. workflow YAML parse checks during authoring;
3. the flipped job's first run is the acceptance test: a green PR must produce an `AUTHORITATIVE` receipt; a deliberately weakened candidate policy (already covered by slice 1's integration tests) must not influence the receipt's policy digest — the CI log's receipt is inspected and recorded;
4. repository verification unchanged; both-toolchain gofmt before push.

## 6. Boundary to slice 3

The GitHub App design: OIDC-validated CI identity, checks posted from outside the repository (true source pinning), trusted-base policy retrieval from the protected base, and the OSS/managed boundary decision (foundation §17). M4 adds signing and replay protection.
