# M3 OSS Reference App Skeleton Design

**Status:** Implemented in M3 app-skeleton slice  
**Date:** 2026-11-06  
**Milestone:** M3 — GitHub App and merge authority (slice 4 of the milestone)  
**Base commit:** `e8d982137f13c8f702ea017e83add98add935e40`  
**Scope:** the offline-testable core of the reference App — check protocol schema, webhook surface, ports-and-adapters evaluation pipeline, OIDC validator, and the `assurectl-app` command skeleton — with no network adapters exercised in this slice

## 1. Purpose

ADR-0010 fixed the App's architecture: merge-gating checks are posted from outside the repository; the protocol is open source; the hosted instance is managed. This slice builds the open-source core that makes self-hosting real: every decision-making path of the App as pure, offline-tested code, with network-facing adapters reduced to thin ports that later slices (or self-hosters) wire to live credentials.

## 2. Architecture: ports around a pure core

```go
package appserver

type PullRequestEvent struct { InstallationID, Repository (owner/name), Action string; BaseSHA, HeadSHA, PRNumber int64 }

type Dependencies struct {
    WorktreeSource  // provides a local worktree for (repository, revision)
    EvidenceSource  // provides []evidence.Loaded for the pull request
    TokenValidator  // validates the CI OIDC token; returns validated claims or unvalidated
    CheckPoster     // posts the check run; records what was posted
}

func EvaluatePullRequest(ctx context.Context, event PullRequestEvent, deps Dependencies) (Outcome, error)
```

- The core composes the exact evaluator pipeline that already exists: resolve the subject from `BaseSHA`/`HeadSHA` inside the fetched worktree (`gitsubject.Resolve`), load policy + contract from the base (`LoadBase`), ingest evidence (`evidence.LoadLocal` over the evidence source's materialized bundle), evaluate (`evaluation.Evaluate` with `Subject`, `Waivers`, `EvaluatedAt`), and build the receipt (`receipt.Build`) with `AUTHORITATIVE` authority when the producer context validated, otherwise `CI_ASSERTED` (per ADR-0010's cap).
- Every dependency is an interface; fixtures implement all four. No HTTP client, no GitHub API call, no JWKS fetch in the core — the offline guarantee the previous design promised.

## 3. Webhook surface (`internal/appserver/http.go`, `cmd/assurectl-app`)

- `POST /webhook`: verifies `X-Hub-Signature-256` (HMAC-SHA256 over the raw body with the configured webhook secret — standard library only), rejects unsigned/mismatched requests with 401, routes `pull_request` actions `opened`/`synchronize` to the core, answers everything else `200` (ignored), and never logs secret material or raw tokens.
- `cmd/assurectl-app`: flags `--app-id`, `--private-key-path`, `--webhook-secret-path`, `--listen` (secrets from files/env, never flags-in-logs); starts the `net/http` server. The command is wiring only (design §13 CLI discipline).

## 4. Check payload protocol (new schema draft)

`schemas/check-payload.v0.schema.json`: the exact JSON the CheckPoster emits — `name: "assurectl"`, `conclusion` (`success`/`failure`/`neutral` mapped from `completion_decision` per the CLI exit-code mapping), `title` (`"AssureCTL <verdict> (<authority>)"`), `summary`, and a receipt reference block (`subject` change-set digest, `receipt` sha256, `authority`). A new draft is allowed here per the App design §7 and is paired with fixture tests; no existing schema changes.

## 5. OIDC validator (`internal/appserver/oidc.go`)

`Validate(token string, expect ExpectedClaims) (ValidatedClaims, error)` with an injectable `KeySource` (the JWKS-fetching adapter is later/managed). Standard-library RSA PKCS#1 v1.5 SHA-256 signature verification, `exp`/`iss`/`aud` checks, then the binding claims from ADR-0010 §4 (`repository`, `repository_owner`, `workflow_ref`, `workflow_sha`, `sha`) cross-checked against the evidence envelope and the resolved subject. Test fixtures sign tokens with test RSA keys generated in-process. Unvalidated context never produces an `AUTHORITATIVE` check.

## 6. Non-scope

- Real GitHub API adapters (installation tokens, artifact download, check posting over the network) — thin adapters, wired and integration-tested in a later slice or by self-hosters;
- App registration, hosting, deployment, secrets operations (managed scope, ADR-0010);
- replay/freshness rules and signing (M4);
- changes to `internal/evaluation`, `internal/receipt`, `internal/gitsubject`, `internal/evidence`, or existing schemas.

## 7. Testing strategy

Test-first throughout, fully offline: check-payload schema fixtures; HMAC verification table (valid, missing, malformed, wrong secret, replayed body mismatch); routing table; core pipeline with fixture ports (approved path, rejected path, evidence failure, worktree failure, unvalidated producer caps authority); OIDC validator with in-process RSA keys (valid token, each wrong claim, expired, wrong signature); command wiring (config parsing, flag validation). Determinism: the same fixture event produces byte-identical check payloads.

## 8. Boundary to the next slices

Slice 5: real GitHub adapters behind the ports (installation auth, artifact download, check posting) plus a self-hosting runbook; M4: signing, replay protection, retention.
