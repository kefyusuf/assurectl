# Managed Scope Architecture Design

**Status:** Draft for review  
**Date:** 2026-11-11  
**Milestone:** Post-roadmap — managed scope (per foundation §17 and ADR-0010)  
**Base commit:** `c041718fe1ba2205e36bf02e78d890c970379943`  
**Scope:** the architecture decision document for the hosted/managed AssureCTL offering — what it provides, how it interacts with the OSS protocol, and what stays open source. A design slice; no server code, no hosting, no App registration.

## 1. Purpose

The foundation design (§17) draws the OSS/managed boundary: "Apache-2.0 open-source scope includes the protocol, schemas, domain model, deterministic evaluator, offline verifier, CLI, reference GitHub Action, and conformance fixtures. Potential managed scope includes the hosted GitHub App, organization policy control plane, managed identity/signing, durable evidence retention, audit search, cross-repository analytics, change intelligence, migration orchestration, enterprise integrations, and support."

M0–M5 built the OSS half completely. This design specifies the managed half: what the hosted service provides, what the OSS protocol already supports, and where the boundary sits technically and operationally.

## 2. What the managed service provides

### 2.1 Hosted GitHub App

- A GitHub App registration operated by the AssureCTL team: webhook endpoint, private key, installation management;
- check runs posted from outside the repository (ADR-0010's external merge authority);
- the hosted instance runs the same OSS `assurectl-app` skeleton behind managed infrastructure — same evaluator, same fail-closed semantics, same receipt format;
- OIDC validation of CI producer identity (JWKS fetch, claim binding) — closing the assertion-only gap of the self-hosted path.

### 2.2 Organization policy control plane

- A management API for organization-level policy layers (foundation §8 layer 2);
- org policy documents are versioned, signed, and served to the hosted App for trusted-base resolution (extending ADR-0004 beyond the single-repo base);
- the control plane enforces: org policy cannot be weakened by project policy; producer identity allowlists may only narrow downstream;
- the OSS evaluator consumes org policy as a `policy_source` layer with `layer: organization` and `trust_status: TRUSTED` — the vocabulary already supports this.

### 2.3 Sigstore keyless signing

- The hosted App signs receipts with Fulcio-issued short-lived certificates keyed to the App's OIDC identity;
- receipts become in-toto Statements signed via DSSE with a Sigstore keyless signature instead of (or in addition to) a local Ed25519 key;
- Rekor transparency log entries provide public verifiability of when each receipt was signed;
- verification (the OSS `verify-receipt` path) requires a trusted root that includes the Sigstore CA — the root format extends with a `sigstore` key type (protocol change in a future schema draft, not this slice);
- no custom cryptography: Sigstore is a standard identity mechanism (ADR-0006).

### 2.4 Durable evidence retention and audit search

- The hosted App stores evidence envelopes, receipts, and artifacts with the subject's change-set digest as the retrieval key;
- retention policies per organization (duration, immutability);
- audit search: query receipts by repository, work-unit type, decision, date range, issuer, requirement ID;
- the OSS CLI gains a `fetch-receipt` command for retrieval from the hosted store (API contract defined in the implementation slice).

## 3. What stays open source

- The entire evaluator pipeline (`internal/attestation` through `internal/verify`) — unchanged;
- the reference GitHub Action (`action.yml`);
- the App skeleton (`internal/appserver`, `cmd/assurectl-app`);
- all schema drafts (`schemas/`);
- all conformance vectors (`conformance/`);
- all ADRs, guides, and design specs;
- `verify-receipt` for offline verification of managed-signed receipts (anyone can verify what the hosted service signed).

The managed service is a deployment of OSS components behind managed infrastructure — not a fork with proprietary extensions.

## 4. Business model alignment

- The OSS protocol creates the standard; the managed service provides convenience, scale, and operations;
- self-hosters get full merge authority without depending on any hosted service;
- the hosted App is for teams that prefer managed operations;
- no vendor lock-in: receipts produced by the hosted App verify offline with the OSS `verify-receipt` command against a trusted root obtained from the App's public JWKS endpoint.

## 5. Non-scope

- Implementation of any managed component (server code, infrastructure, deployment);
- pricing, SLA, or support terms;
- Sigstore infrastructure deployment (the managed service uses the public Sigstore ecosystem, not a private instance);
- protocol changes (the OSS vocabulary and schemas already support the managed scope's needs, except the Sigstore root extension noted in §2.3).

## 6. Phasing

1. **Phase 1 (implementation slice):** hosted App deployment using the OSS skeleton + OIDC validation;
2. **Phase 2:** org policy control plane API + signed org policy serving;
3. **Phase 3:** Sigstore keyless signing integration;
4. **Phase 4:** retention/audit search + `fetch-receipt` CLI command.

Each phase is a separate slice with its own design and implementation plan.
