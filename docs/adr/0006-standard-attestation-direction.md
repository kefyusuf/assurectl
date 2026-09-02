# ADR-0006: Use Standard Attestation Building Blocks

**Status:** Accepted  
**Date:** 2026-09-02

## Context

Signing a custom JSON structure requires canonicalization, key management, identity, replay protection, rotation, and interoperability decisions. Inventing a proprietary cryptographic format would add risk before the completion semantics are stable.

## Decision

M0 emits no receipts. M1, the first receipt-producing milestone, emits unsigned advisory receipts only. A future authoritative signed format will align with:

- in-toto Statement for the subject and predicate envelope;
- an AssureCTL-specific completion-decision predicate;
- DSSE for signing envelope semantics;
- Sigstore/OIDC or another standard identity mechanism for managed signing.

The project does not define a custom signature primitive or allow a certificate to establish trust by embedding its own key. Stable signing requires a separate protocol version, threat model, replay policy, issuer lifecycle, and conformance fixtures.

## Consequences

- Early implementation can focus on correct subject, policy, evidence, verdict, waiver, and decision semantics.
- Receipt structures must remain suitable for later attestation wrapping.
- Signed authority is deferred until identity and lifecycle controls are specified.
- Local unsigned receipts are explicitly non-authoritative.

## Rejected Alternatives

- **Custom canonical JSON plus bespoke signatures:** unnecessary cryptographic and interoperability risk.
- **Self-rooted keys in receipts:** allows the worker to become its own authority.
- **Sign every raw test output:** creates operational noise without solving policy or provenance semantics.
