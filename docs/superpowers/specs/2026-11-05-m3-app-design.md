# M3 GitHub App Architecture and OSS/Managed Boundary Design

**Status:** Draft for review  
**Date:** 2026-11-05  
**Milestone:** M3 — GitHub App and merge authority (slice 3 of the milestone)  
**Base commit:** `b4ac52c2d9f3c7f8c5c939afdc6ffe1ab8fb8728`  
**Scope:** the App's architecture decision (ADR-0010), the check protocol the App posts, the OIDC validation approach for CI producer identity, and the explicit OSS/managed split — a design slice; no hosting, no App registration, no server code in this slice

## 1. Purpose

Slices 1–2 made authoritative evaluation executable and blocking in this repository's own CI, with one documented gap: for `pull_request` events the workflow file travels with the PR, so a candidate can still edit the verifying workflow itself. Foundation design §11 Tier 2 names the closure — "a GitHub App resolves trusted policy and contract sources, validates evidence, and publishes a required check whose expected source is pinned to that App." This slice fixes that closure's design: what the App is, where its boundary sits, what it validates, and what it never claims.

## 2. ADR-0010 (authored in this slice): checks that gate merges are posted from outside the repository; the hosted App is managed scope

1. **External checks.** Merge-gating verification checks are published by a GitHub App installation — an actor outside the repository that a PR cannot edit — resolving policy and contract from the protected base and validating evidence before posting the check. Workflow-defined verification remains developer feedback; only App-posted checks are merge authority.
2. **OSS/managed split (foundation §17):** open source ships the protocol the App speaks — check-run payload schema, receipt requirements, OIDC claim requirements, conformance fixtures — and a reference implementation skeleton under `internal/appserver` + `cmd/assurectl-app` with the evaluator reused as a library; the hosted instance (webhook endpoint, secrets, App registration, operations) is managed scope. Anyone can run their own App from the OSS skeleton; the protocol does not depend on AssureCTL's hosted instance.
3. **The App validates producer identity — closing the assertion gap.** The M2 CI context was assertion-only. The App requires the evidence-producing workflow to present a GitHub Actions OIDC token; the App fetches the token's JWKS from the token issuer (network is allowed in the App — it is not the core evaluator), validates signature, issuer, audience, and the claims that bind the run to the repository and workflow, and only then treats the evidence's producer identity as validated. Evidence without a validated producer context can still be evaluated, but the resulting check is capped at `CI_ASSERTED`-grade claims and the check text says so.
4. **The App never signs and never stores.** Receipts remain unsigned until M4; the App holds no durable evidence store (retention is managed scope, M4+); replay/freshness rules are M4. The App's authority comes from resolving inputs itself, not from caching past verdicts.

## 3. Check protocol (what the App posts)

A check run named `assurectl` (singular, App-pinned source) with:

- `conclusion`: mapped from the receipt's `completion_decision` exactly as the CLI exit codes map (`APPROVED`/`ACCEPTED_WITH_RISK` → success; `REJECTED` → failure; `BLOCKED` → neutral);
- `output.title`: `"AssureCTL <verdict> (<authority>)"`;
- `output.summary`: the receipt's requirement results, findings, and decision in human form;
- `output.text` / details URL: the full receipt JSON as an artifact uploaded by the App;
- `details_url`: unset in OSS phase (no hosted UI).

The check's source pinning property is structural: the check run's `app` field names the installation; branch protection requires *that* app's check. A PR editing any workflow cannot forge, edit, or delete it.

## 4. OIDC validation approach

- Token: the evidence-producing workflow presents `ACTIONS_ID_TOKEN_REQUEST_TOKEN` claims via its job's `permissions: id-token: write`; the produced evidence envelope is accompanied by the raw token (or the App fetches it from the job context — design detail for the implementation plan);
- Validation: JWKS from `https://token.actions.githubusercontent.com/.well-known/jwks` (cached, keyed rotation-safe), signature via standard `crypto/rsa` verification, then claim checks: `iss`, `aud` (the App's configured client id), `repository`, `repository_owner`, `workflow_ref`, `workflow_sha`, `sha` — each cross-checked against the evidence envelope's subject/producer fields and the resolved subject;
- Failure posture: any missing/mismatched claim → the producer context is unvalidated → advisory-grade check with an explicit statement, never an AUTHORITATIVE check;
- No custom cryptography: standard library signature verification only; the "no custom cryptography" rule is unchanged because the validation lives in the App component, not the core evaluator.

## 5. OSS/managed split table

| Capability | OSS | Managed |
|---|---|---|
| Check-run payload protocol + fixtures | ✔ | |
| Reference App skeleton (`internal/appserver`, `cmd/assurectl-app`) reusing the evaluator as a library | ✔ | |
| OIDC validation module + conformance vectors | ✔ | |
| Hosted App instance (webhook endpoint, registration, secrets, uptime) | | ✔ |
| Organization policy control plane | | ✔ (M3+) |
| Durable evidence retention / audit search | | ✔ (M4+) |
| Managed signing (DSSE/Sigstore issuer) | | ✔ (M4) |

## 6. Phasing after this slice

1. OSS skeleton slice: `internal/appserver` webhook → subject resolution → trusted-base evaluation → check posting, with conformance fixtures and offline tests (the App's handler logic is testable without hosting: webhook payloads and OIDC tokens are fixtures);
2. Registration/ops runbook for self-hosters;
3. The managed hosted instance is an operational product decision, not a protocol one.

## 7. Non-scope of this slice

No server code, no App registration, no hosted deployment, no secrets handling, no change to the evaluator, CLI, action, or existing schemas; the check payload schema becomes a new draft only in the implementation slice (with its own review).
