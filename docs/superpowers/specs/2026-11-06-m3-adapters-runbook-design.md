# M3 GitHub Adapters and Self-Hosting Runbook Design

**Status:** Draft for review  
**Date:** 2026-11-06  
**Milestone:** M3 — GitHub App and merge authority (slice 5 of the milestone)  
**Base commit:** `00d0588770a570c342d9c079aea15554a30bba67`  
**Scope:** the four real adapters behind the App's ports, wired into `cmd/assurectl-app`, plus the self-hosting runbook — all adapter behavior tested against local `httptest` servers

## 1. Purpose

The app skeleton (slice 4) left the ports nil with a fail-closed error: "adapters arrive in a later slice." This slice builds them so a self-hoster can run a real App: installation authentication, check posting, evidence artifact download, server-side worktrees, and JWKS fetching — every adapter tested against local fixture servers (`net/http/httptest`), never against the live GitHub API, and a runbook that walks registration through verification.

## 2. Adapters (`internal/appserver/github.go` + supporting files)

1. **Installation auth.** `AppJWT` signs RS256 tokens (`iss`: app id, `iat`/`exp` window) from the App's PEM private key (standard library); `InstallationToken(ctx, installationID)` exchanges a JWT for an installation token via `POST /app/installations/{id}/access_tokens`, caching per installation until near expiry. The base URL is injectable (fixture server in tests; `https://api.github.com` in production wiring).
2. **CheckPoster adapter.** `POST /repos/{owner}/{repo}/check-runs` with the slice-4 payload (name `assurectl`, conclusion, title, summary, output, receipt reference in the details text); maps API errors to errors without leaking the token.
3. **EvidenceSource adapter.** Lists artifacts for the PR's head SHA (API), downloads the evidence archive, materializes it under a temp bundle laid out as `<bundle>/.assurectl/evidence/...`, and returns the bundle plus the workflow's OIDC token (fetched via the documented `ACTIONS_ID_TOKEN_REQUEST_*` mechanism relayed by the evidence producer's job summary artifact — the exact relay contract is fixed in the runbook and asserted by one adapter test).
4. **WorktreeSource adapter.** Server-side Git: `git fetch <remote> <revision>` into a managed cache directory (the vetted execution pattern: allowlisted binary, sanitized environment, argument-list exec) and returns a worktree path. Revisions are pre-validated object ids; the remote URL comes from App configuration, never from webhook payloads.
5. **KeySource adapter.** Fetches and caches the issuer's JWKS (`/.well-known/jwks`), serves keys by `kid`, refreshes on unknown `kid` once.

## 3. Wiring

`cmd/assurectl-app` gains `--api-base-url` (fixture/testing override, defaults to the production API) and builds the real `Dependencies`; the nil-port fail-closed path remains the behavior when adapters are absent (skeleton compatibility).

## 4. Self-hosting runbook (`docs/guides/self-hosting.md`)

Steps: create and register the App (permissions: `checks: write`, `contents: read`, `metadata: read`; webhook secret; private key), configure and start `assurectl-app`, install it on target repositories, wire the evidence-producing workflow (OIDC token relay artifact), make the App's `assurectl` check required in branch protection, and verify end-to-end with a test PR. The runbook claims nothing the code does not implement and marks the hosted/managed alternative explicitly (ADR-0010).

## 5. Non-scope

- No changes to `internal/appserver`'s core, evaluator, kernel, or existing schemas; the adapters only *implement the existing ports*.
- No live-GitHub integration tests, no hosted deployment, no secrets handling beyond file reads; replay/signing stay M4.

## 6. Testing strategy

Test-first, all against local `httptest` fixture servers: JWT signing shape (header/claims, PEM parsing); installation-token exchange (valid, error, caching); CheckPoster request shape (method, path, headers, payload) and error mapping; artifact list/download/materialization (happy path, missing artifact, bundle layout); worktree fetch (real local Git: fetch an object id into cache, invalid revision rejected, remote from config only); JWKS adapter (serve keys, unknown-kid refresh, malformed JWKS); wiring test (adapters built from config, nil path preserved when unconfigured). Repository verification unchanged; both-toolchain gofmt before push.

## 7. Boundary to M4

M4 adds signing (in-toto/DSSE via Sigstore), replay protection, and retention — the App then attaches signed attestations to the checks it posts. No App redesign is anticipated.
