# ADR-0004: Resolve Authoritative Project Policy from a Trusted Base

**Status:** Accepted  
**Date:** 2026-09-02

## Context

A candidate change can edit repository policy, test configuration, or workflow files. If the candidate's proposed policy evaluates the same candidate, an agent or contributor can remove requirements and approve its own weakening change.

## Decision

Authoritative evaluation resolves project policy from the trusted base revision, combined with the non-waivable protocol baseline and, later, protected organization policy. A work-unit contract may add constraints but may not weaken upstream requirements.

A policy-changing pull request is evaluated under the previously trusted policy. Its new policy becomes eligible only after the change is accepted and becomes part of a trusted base.

Policy merges are monotonic where possible: required checks accumulate, trust requirements may increase, evidence age limits may tighten, and downstream layers may narrow producer identities. Ambiguous or non-orderable conflicts block evaluation.

## Consequences

- Candidate code cannot directly relax the gate that evaluates it.
- Base revision and policy digest must be recorded in authoritative receipts.
- Local workspace policy remains advisory.
- Organization-level delegation rules require explicit design later.

## Rejected Alternatives

- **Load policy from PR head:** enables self-weakening changes.
- **Trust any repository writer:** identity alone does not protect evaluation ordering.
- **Arbitrary policy scripts:** expands the attack surface and harms determinism.
