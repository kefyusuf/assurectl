# Agent Instructions

These rules apply to coding agents and other automated contributors working in this repository.

## Required reading

Before editing, read:

1. `docs/superpowers/specs/2026-09-02-assurectl-foundation-design.md`;
2. the implementation plan for the active milestone;
3. relevant files in `docs/adr/`;
4. `docs/threat-model/v0.md`;
5. `CONTRIBUTING.md`.

## Non-negotiable rules

- Do not claim completion without fresh command output from the full relevant verification set.
- Write a failing test before production code for behavior changes and confirm it fails for the intended reason.
- Do not weaken, delete, skip, or relabel tests, policy, evidence requirements, or security checks merely to obtain a passing result.
- Keep LLM analysis advisory. Authoritative domain decisions must remain deterministic.
- Preserve the separation between evidence state, observed outcome, finding, verification verdict, waiver, and completion decision.
- A waiver is risk acceptance; it never turns failed or indeterminate verification into passing verification.
- Do not allow a candidate change to select a weaker policy for evaluating itself.
- Do not add secrets, credentials, private evidence contents, customer data, or production keys.
- Do not fetch arbitrary network resources from the core evaluator.
- Do not add custom cryptography or a self-rooted trust model.
- Stop and surface ambiguity instead of inventing a security-sensitive assumption.

## Scope control

Implement only the active milestone. M0 does not include local `verify`, Git subject resolution, YAML parsing, command execution, a GitHub App, signing, migration generation, SaaS features, or an LLM judge.

## Required handoff

Report:

- files changed;
- decisions affected;
- commands run with exit status;
- known limitations;
- any unverified claim or unavailable environment.
