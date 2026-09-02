# ADR-0001: Operate as a Completion Assurance Authority

**Status:** Accepted  
**Date:** 2026-09-02

## Context

Coding agents and CI jobs can report success, but the worker that benefits from completion should not unilaterally issue the authoritative completion state. A test runner or status aggregator cannot express the complete relationship between work intent, trusted policy, evidence, risk acceptance, and lifecycle state.

## Decision

AssureCTL operates as an independent completion assurance authority for software changes. It evaluates a declared work unit and exact software subject under a verification contract and effective policy, then produces a verification verdict and completion decision.

Evidence producers may run tests and checks. Agents may propose code, criteria, evidence, and advisory analysis. Neither role issues the authoritative completion decision.

## Consequences

- The core remains agent-, language-, framework-, and CI-provider-neutral.
- Execution adapters are optional producers rather than the product's authority model.
- Local execution is advisory until an independent trusted authority is introduced.
- Product claims must describe evaluated evidence and policy, not universal correctness.

## Rejected Alternatives

- **CI/test aggregator:** too narrow and conflates job status with completion authority.
- **Agent self-certification:** violates authority separation.
- **LLM judge as authority:** non-deterministic and difficult to reproduce or audit.
