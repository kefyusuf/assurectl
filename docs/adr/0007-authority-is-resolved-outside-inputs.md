# ADR-0007: Resolve Input Authority Outside the Input Document

**Status:** Accepted  
**Date:** 2026-09-02

## Context

A policy, verification contract, or evidence envelope is controlled by the party or workspace that supplies it. Allowing the document to declare itself trusted or authoritative would make authority self-rooted: an untrusted worker could set a field to `AUTHORITATIVE` without establishing an external trust relationship.

## Decision

Policy, contract, and evidence payloads do not establish their own authority. The loader or integration resolves source identity and trust from external context such as a trusted base revision, protected control plane, pinned issuer, source-pinned GitHub App, or explicitly advisory local workspace.

Receipts record the verifier's resolved input source, digest, and trust status. That metadata is output of evaluation, not a claim accepted from the input document. Overall receipt authority remains separate from input trust.

## Consequences

- Workspace files cannot promote themselves from advisory to authoritative.
- Adapters must supply provenance and trust context separately from parsed payloads.
- Policy and contract schemas remain content schemas rather than trust-root schemas.
- Future signing work must pin issuers outside the signed payload being evaluated.

## Rejected Alternatives

- **`authority` field inside policy or contract:** self-asserted and therefore not a trust root.
- **Trust based only on repository write access:** does not protect candidate-head policy evaluation.
- **Embedded public key as authority:** permits self-rooted signatures unless an external trust policy pins it.
