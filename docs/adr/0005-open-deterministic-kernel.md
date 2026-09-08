# ADR-0005: Keep the Deterministic Evaluation Kernel Open

**Status:** Accepted  
**Date:** 2026-09-02

## Context

Assurance depends on inspectable and reproducible rules. A closed evaluator forces users to trust hidden semantics, while security through obscurity does not prevent a worker from attacking exposed integration surfaces.

## Decision

The protocol, schemas, domain model, deterministic evaluator, offline verifier, CLI, reference GitHub Action, and conformance fixtures are open source under Apache-2.0.

The kernel uses typed, finite rules and does not rely on an LLM verdict. Managed services may provide protected hosting, organization policy, identity, signing, retention, analytics, and integrations, but their decisions must remain independently verifiable by the open components.

## Consequences

- Users can inspect and reproduce decision semantics.
- Security must rest on protected authority, identity, provenance, revision binding, and policy—not hidden rules.
- Protocol compatibility and conformance fixtures become important project assets.
- Hosted differentiation comes from operational trust and integrations rather than an opaque verdict algorithm.

## Rejected Alternatives

- **Private enforcement kernel:** weakens independent verification and community trust.
- **LLM-only evaluation:** non-deterministic and unsuitable for authoritative completion.
- **Open verifier with unverifiable hosted semantics:** does not establish parity between local verification and managed decisions.
