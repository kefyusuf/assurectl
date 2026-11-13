# AssureCTL Project Status

**Date:** 2026-11-13  
**Milestone:** M0–M5 complete + managed scope designed  
**Repository:** [github.com/kefyusuf/assurectl](https://github.com/kefyusuf/assurectl)

## What AssureCTL does

AssureCTL is a deterministic, revision-bound assurance authority for software changes. When a coding agent, migration tool, or contributor proposes a change, AssureCTL evaluates machine-verifiable evidence against an explicit contract and trusted policy — and decides what the evidence permits. Agent prose, self-authored status files, and self-generated assertions are never treated as evidence.

## Architecture

```
CLI / GitHub App
       ↓
subject resolver (Git revision, fail-closed ambiguity rejection)
       ↓
contract loader + policy resolver (trusted-base or workspace)
       ↓
evidence loader (artifact digest verification, set-wide fail-closed)
       ↓
evaluation engine (MISSING/STALE/UNTRUSTED/VALID + findings)
       ↓
waiver engine (explicit opt-in, never auto-discovered)
       ↓
decision kernel (APPROVED/REJECTED/ACCEPTED_WITH_RISK/BLOCKED)
       ↓
receipt builder (in-toto statement over DSSE with Ed25519)
```

## What's built

| Capability | Status |
|---|---|
| Git subject resolution (fail-closed ambiguity, dirty detection, local-only identity) | ✅ |
| Evidence envelope ingestion (artifact digest, stale/untrusted/missing, set-wide fail-closed) | ✅ |
| Requirement evaluation (MISSING/STALE/UNTRUSTED/VALID normalization, findings) | ✅ |
| Waiver engine (explicit opt-in, rotation, revocation, scope/timing) | ✅ |
| Decision kernel (deterministic precedence, kernel-verified receipts) | ✅ |
| CLI (`verify`, `sign-receipt`, `verify-receipt`, `sign-root`, `init`) | ✅ |
| GitHub Actions OIDC producer identity validation | ✅ |
| Trusted-base resolution (policy + contract from base revision) | ✅ |
| Signed trusted-root rotation + revocation | ✅ |
| Reference GitHub Action + self-hostable App | ✅ |
| Conformance vectors (16 JSON files across 4 protocol surfaces) | ✅ |
| Dogfooding gate (blocking AUTHORITATIVE receipts on every PR) | ✅ |

## Trust tiers

| Tier | Authority | How | Merge-grade |
|---|---|---|---|
| Advisory | `ADVISORY` | Local workspace evaluation, no flags | No |
| CI-asserted | `CI_ASSERTED` | GitHub Actions runner assertion, subject cross-checks enforced | Depends on workflow protection |
| Authoritative | `AUTHORITATIVE` | Trusted-base resolution + validated producer identity + external check posting | Yes (with App + branch protection) |

## Architecture decisions

| ADR | Decision |
|---|---|
| ADR-0001 | Completion assurance authority |
| ADR-0002 | Separate verdict, waiver, and decision |
| ADR-0003 | Revision-bound subjects |
| ADR-0004 | Trusted-base policy resolution |
| ADR-0005 | Open deterministic kernel |
| ADR-0006 | Standard attestation direction |
| ADR-0007 | Authority is resolved outside inputs |
| ADR-0008 | Waiver inputs are advisory, explicit, never authoritative |
| ADR-0009 | Resolve policy/contract from base revision |
| ADR-0010 | Merge-gating checks posted from outside the repository |

## Roadmap status

| Milestone | Scope | Status |
|---|---|---|
| M0 | Foundation, domain semantics, decision table, governance | ✅ |
| M1 | Local advisory evaluation for exact Git revisions | ✅ |
| M2 | Trusted CI evidence envelopes, waiver engine, reference action | ✅ |
| M3 | Trusted-base resolution, GitHub App, merge authority, OSS skeleton + adapters | ✅ |
| M4 | Signed receipts (in-toto/DSSE/Ed25519), trusted-root rotation, conformance | ✅ |
| M5 | Migration producer integration (`init`, examples, guide) | ✅ |

## Remaining managed-scope items

- Sigstore keyless signing (Fulcio/Rekor) — managed deployment
- Organization policy control plane — managed deployment
- Durable evidence retention + audit search — managed deployment
- Transparency log integration — managed deployment

## Remaining maintainer actions

- Branch protection: `assurectl` check is required (configured via GitHub API)
- Optional: require branches to be up to date before merging

## Guides

- [Protected workflows](docs/guides/protected-workflows.md) — required-check posture
- [Self-hosting](docs/guides/self-hosting.md) — run your own App instance
- [Migration producers](docs/guides/migration-producers.md) — integrate migration tools
