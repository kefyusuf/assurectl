# Protected Workflow Guidance

This document describes the operational posture `assurectl verify --ci` assumes when it runs inside GitHub Actions. It is guidance, not enforcement: the evaluator implements none of the protections below — it records what a runner asserts (authority tier `CI_ASSERTED`) and fails closed when those assertions contradict the resolved Git subject. Everything stronger is future work (see the outlook at the end).

## What CI_ASSERTED means — and does not mean

A receipt with `authority: CI_ASSERTED` says: *a GitHub Actions runner supplied a verification context, that context agreed with the resolved repository identity and head revision, and the evaluation that followed is reproducible from the recorded inputs.* It does **not** say:

- that the runner's identity was cryptographically validated (OIDC token validation is deferred to a later milestone);
- that the workflow was protected against edits by the candidate change;
- that the receipt is merge-grade (foundation design §11: Tier 1 is "not automatically equivalent to an independent authority").

Treat `CI_ASSERTED` receipts as strong developer feedback and a building block for a gate — not as a gate.

## Least-privilege workflow permissions

Workflows that run verification should declare the narrowest permission set:

```yaml
permissions:
  contents: read
```

and check out with `persist-credentials: false` so the job cannot push under the runner's token. A verification job that can write to the repository undermines the separation between the worker proposing a change and the authority evaluating it (INV-001).

## Pin every action to an immutable SHA

Tag references are mutable. Pin third-party actions to full commit SHAs with a version comment, exactly as this repository's workflows do:

```yaml
uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
```

The reference action (`action.yml`) pins its steps the same way and pins the exact `assurectl` module revision it installs, so a given action revision always runs the same verifier.

## Treat `pull_request` checkouts as untrusted

Workflows triggered by `pull_request` from forks run against a merge commit of untrusted code with no repository secrets. Keep verification jobs in that trigger class until you have a trusted identity mechanism. AssureCTL's subject cross-checks (CI `GITHUB_SHA` and `GITHUB_REPOSITORY` must match the resolved subject, else no receipt) are consistency checks, not trust grants.

## Only verify evidence whose artifacts you can re-digest

Evidence envelopes carry artifact digests; the evaluator re-hashes every artifact during ingestion and fails closed on mismatch. When you wire evidence production into your own workflow:

- upload the artifact and the envelope **together**, from the same job, in the same run;
- compute the digest over the exact bytes you upload (line-ending conversions corrupt digests — keep artifact generation on one platform);
- prefer evidence bound to the exact revision being verified; anything else is recorded as `STALE` by the evaluator and blocks completion.

## Explicit inputs, never discovery

Waivers load only from `--waivers <path>` (ADR-0008): a repository cannot ship a silent self-authorization. The same posture applies to your own integration — anything that relaxes the gate should be an explicit, auditable input, not an ambient file.

## Making the check required

This repository runs its own dogfooding verification blocking under the base gate: the `Verify with own evidence` job runs `assurectl verify --trusted-base --ci` and fails the PR's CI on a `REJECTED`/`BLOCKED` decision or any pipeline error. The receipt's authority is `AUTHORITATIVE` — the policy and contract came from the PR's base revision, so a candidate that edits its own `.assurectl/` files does not change the gate it is evaluated under.

To make the check *required* for merges, add it in branch protection (repository → Settings → Branches → Branch protection rule for `main`):

1. Enable **Require status checks to pass before merging**;
2. add `Verify with own evidence` to the required checks;
3. optionally require branches to be up to date before merging.

Three boundaries to keep in mind:

- **The workflow file travels with the PR.** For `pull_request` events GitHub runs the workflow definition from the merge ref, so a PR can still edit `ci.yml`. Two facts bound the window: the action installs `assurectl` at an immutable module revision, and the receipt records the base's gate. Making the check required closes most of the rest: removing the job from the workflow then shows as a missing required check.
- **The definitive closure is the GitHub App** (next slice): checks posted from outside the repository cannot be edited by the PR.
- **`AUTHORITATIVE` receipts assert local facts**: the gate documents came from the recorded base revision. They do not validate the runner's identity or replace branch protection.

## Outlook

Later milestones replace the assertion boundary with validation: trusted-base policy resolution and a source-pinned GitHub App (M3), in-toto/DSSE/Sigstore-signed attestations with replay protection (M4). Until then, keep the verification job visible but non-blocking; when authoritative receipts arrive, flipping the gate to required is a branch-protection decision backed by a receipt whose provenance the evaluator has actually checked.
