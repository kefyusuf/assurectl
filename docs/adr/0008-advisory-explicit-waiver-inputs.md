# ADR-0008: Waiver Inputs Are Advisory, Explicit, and Never Authoritative

**Status:** Accepted  
**Date:** 2026-11-02

## Context

M1 emitted receipts with permanently empty waiver sets and left `ACCEPTED_WITH_RISK` unreachable: a local waiver input was recognized as a security-sensitive protocol surface (see the M1 verify-receipt design, section 2). The M0 execution notes defer exact failed-requirement-to-waiver targeting, waiver authority/scope/timing validation, and waiver-record emission to semantic M1+ enforcement — the receipt builder implements those checks since the M1 verify-receipt slice, but no input could supply records.

A waiver is explicit risk acceptance (foundation design §10). The self-authorization hazard is concrete: a repository that ships its own waiver file inside the evaluated candidate could silently neutralize its own gate — the same self-weakening pattern ADR-0004 and ADR-0007 exist to prevent for policy and contract authority.

## Decision

1. **Explicit opt-in only.** Waiver records load exclusively from a path passed as `assurectl verify --waivers <path>`. There is no auto-discovery: absence of the flag means an empty waiver set, regardless of what the workspace contains.
2. **Advisory only.** Waiver-bearing receipts carry authority `ADVISORY`. When M3 introduces authoritative receipts, waiver provenance becomes subject to the same source-eligibility rules as policy provenance; this ADR fixes that waiver inputs from an evaluated candidate are ineligible for authoritative application.
3. **Never authoritative over facts.** Applying a waiver sets `RequirementResult.WaiverStatus` and nothing else: evidence state, observed outcome, findings (beyond the waiver findings themselves), and the verification verdict are immutable to it (INV-008; ADR-0002). The deterministic kernel's `ACCEPTED_WITH_RISK` branch remains the only path from a waiver to a decision.
4. **Policy bound is absolute.** No input can waive a non-waivable requirement; a record targeting a non-waivable or non-failed requirement is rejected, recorded, and surfaced with a non-blocking finding while the failure keeps driving `REJECTED`.
5. **Records are auditable claims.** Each record carries its canonical digest, approver identity and authority, exact scope (repository, work unit, optional head revision), and issue/expiry timestamps. `status` in the input is a claim; the engine re-derives applicability against the explicit evaluation instant.
6. **No custom cryptography.** The digest is the canonical SHA-256 of the validated record; no signature, no self-rooted trust (ADR-0006).

## Consequences

- `ACCEPTED_WITH_RISK` becomes reachable end-to-end for advisory evaluation.
- The receipt's waiver array carries applied and rejected records with digests, making risk acceptance auditable without making it trusted.
- M3 authoritative receipts must extend the policy provenance-eligibility machinery to waiver sources before any waiver can influence an authoritative decision.
- CI/trusted contexts (M2 slice 2+) may supply waiver records from external sources; the explicit-input rule still holds.

## Rejected Alternatives

- **Auto-discovered `.assurectl/waivers.json`:** silently lets a candidate authorize its own risk acceptance.
- **Blocking findings for rejected waivers:** would misreport a known technical failure as indeterminate; rejection already fails closed by refusing application.
- **Waiver mutations of evidence state:** breaks INV-008 and the verdict-before-waivers ordering of ADR-0002.
