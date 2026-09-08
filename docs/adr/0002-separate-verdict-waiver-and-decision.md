# ADR-0002: Separate Evidence, Verdict, Waiver, and Completion Decision

**Status:** Accepted  
**Date:** 2026-09-02

## Context

A single `PASS/FAIL` field cannot distinguish missing evidence, a failed observed check, stale proof, unauthorized production, approved risk acceptance, and the final lifecycle decision. Treating a waiver as a pass rewrites technical truth and damages auditability.

## Decision

AssureCTL models these concerns separately:

- evidence state: `VALID`, `INVALID`, `MISSING`, `STALE`, `UNTRUSTED`;
- observed outcome: `PASSED`, `FAILED`, `ERROR`;
- verification verdict: `PASSED`, `FAILED`, `INDETERMINATE`;
- finding: a normalized fact with a stable code;
- waiver: scoped, authorized risk acceptance;
- completion decision: `APPROVED`, `REJECTED`, `BLOCKED`, `ACCEPTED_WITH_RISK`.

A waiver never changes evidence state, observed outcome, findings, or verification verdict. `ACCEPTED_WITH_RISK` requires established evidence and valid waivers for every failed, explicitly waivable requirement. Any indeterminate required verification blocks completion.

## Consequences

- Audit records preserve known failures even when risk is accepted.
- Missing, stale, invalid, and untrusted evidence cannot be waived into passing evidence.
- Consumers must inspect both verdict and completion decision.
- The decision precedence requires explicit tests.

## Rejected Alternatives

- **One verdict enum:** hides why verification failed or could not be established.
- **Waived pass:** falsely represents a failed check as successful.
- **Human approval overrides all states:** permits bypass of non-waivable protocol invariants.
