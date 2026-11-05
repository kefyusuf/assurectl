# ADR-0010: Merge-Gating Checks Are Posted from Outside the Repository

**Status:** Accepted  
**Date:** 2026-11-05

## Context

Workflow-defined verification (M2/M3 slices 1–2) has a structural gap: for `pull_request` events GitHub runs the workflow file from the merge ref, so the candidate can edit the verifying workflow itself. The pinned verifier binary and the base's gate bound that window, but only an actor outside the repository closes it. Foundation design §11 Tier 2 names that actor: a GitHub App publishing a required check whose expected source is pinned to the App. Foundation §17 also asks where the App sits between the Apache-2.0 open-source scope and the potential managed scope.

## Decision

1. **Merge authority lives in App-posted checks.** Only a check run published by a GitHub App installation gates merges. Workflow-defined verification remains developer feedback. A PR cannot forge, edit, or delete a check posted by an app it does not control.
2. **The protocol is open source; the hosted instance is managed.** Open source ships the check-payload protocol, conformance fixtures, OIDC claim requirements, and a reference App skeleton that reuses the evaluator as a library. The hosted App instance — webhook endpoint, registration, secrets, operations — is managed scope (foundation §17). Self-hosting is a first-class path; the protocol does not depend on AssureCTL's hosted instance.
3. **The App validates producer identity with standard OIDC.** Evidence presented for authoritative checks carries a GitHub Actions OIDC token; the App verifies signature, issuer, audience, and the claims binding the run to the repository, workflow, and revision via JWKS fetched over the network (permitted in the App; the core evaluator remains offline). Unvalidated producer context caps the check at advisory-grade claims. Standard library signature verification only — no custom cryptography (ADR-0006).
4. **The App resolves inputs itself.** Policy and contract come from the protected base revision (ADR-0004, ADR-0009); evidence is ingested with the same fail-closed ingestion as everywhere else. The App never caches verdicts, never signs (M4), and holds no durable evidence store.

## Consequences

- The workflow-edit caveat of the gate-flip slice is closed: even a PR that disables its own verification workflow faces the App's check.
- OSS self-hosters get merge authority without depending on any hosted service.
- The App component may use network and standard cryptography primitives; the "no arbitrary network fetch from the core evaluator" rule is unchanged because the App is not the core evaluator.
- Replay protection, freshness rules, and signing arrive with M4, at which point App-posted receipts can be backed by signed attestations.

## Rejected Alternatives

- **Required workflow checks alone:** the workflow file travels with the PR; the gate can be edited by the candidate.
- **Repository-hosted bot (a workflow reacting to workflow_run):** still inside the repository's workflow graph and editable by the candidate.
- **Making the hosted App the only path:** violates the open-source boundary in foundation §17; self-hosting must remain possible.
- **Custom token formats or self-rooted keys:** ADR-0006 forbids them; standard OIDC is the identity mechanism.
