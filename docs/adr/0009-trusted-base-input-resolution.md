# ADR-0009: Resolve Project Policy and Contract from the Subject's Base Revision

**Status:** Accepted  
**Date:** 2026-11-04

## Context

ADR-0004 fixed the rule — authoritative evaluation resolves project policy from the trusted base revision, and a candidate cannot weaken the gate that evaluates it — but every evaluation so far has read `.assurectl/` inputs from the candidate workspace. Those inputs are advisory because of their source context (ADR-0007), and the receipt vocabulary has had no way to express trust earned from a source. Meanwhile this repository's own CI produces CI-asserted receipts whose gate is still the candidate's own policy file.

## Decision

1. **Trusted-base mode is explicit.** `assurectl verify --trusted-base` reads the project policy and the verification contract from the subject's already-resolved base revision through the vetted Git execution path. The mode reads nothing else; a missing base document is an error, never a workspace fallback.
2. **Trust is earned by source, never asserted.** The closed vocabulary gains `TRUSTED` and `TRUSTED_BASE`. Only the base loaders may produce them: an input is trusted because it was read from the recorded base revision of the subject, not because any field says so. The receipt builder rejects `TRUSTED` inputs whose source string is not a `base:<object-id>:<path>` form — hand-forged trust fails closed.
3. **The blob path is allowlisted.** `ReadBlob` executes `git show <revision>:<path>` only for `.assurectl/policy.json` and `.assurectl/contract.json`, reusing the resolver's trusted executable, sanitized environment, and argument-list execution. No new execution pattern, no free-form paths, no shell.
4. **Authoritative receipts carry resolvable provenance.** `AUTHORITATIVE` receipts require `TRUSTED` contract and policy inputs and a project policy layer marked `revision_binding: SUBJECT_BASE` with no free `source_revision` — the marker resolves to the receipt subject's actual base object (M0 execution notes).
5. **The claim stays local.** This slice's authoritative claim is "the gate the candidate is evaluated under is exactly the one its base revision carries". Branch protection, required-check identity, and runner validation remain later M3 work.

## Consequences

- A policy- or contract-changing pull request is evaluated under the base's documents; its own become eligible only after acceptance into a base (ADR-0004).
- The receipt's provenance markers become semantically checkable, enabling source-pinned required checks (M3 slice 2) and the eventual gate flip.
- Advisory mode is unchanged: no flag, byte-identical behavior and receipts.
- OIDC/runner identity validation, organization policy layers, and waiver provenance beyond explicit files remain future work.

## Rejected Alternatives

- **Reading the candidate's policy with a TRUSTED label:** recreates self-weakening; trust must come from source context.
- **Free-form `git show <path>`:** expands the execution surface for no protocol gain.
- **Auto-selecting trusted-base mode when a base exists:** mode choice is caller intent; the advisory loop must stay byte-identical.
