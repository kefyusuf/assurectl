# M3 GitHub Adapters and Runbook Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Design:** `docs/superpowers/specs/2026-11-06-m3-adapters-runbook-design.md`  
**Base commit:** `00d0588770a570c342d9c079aea15554a30bba67`

## Global Constraints

- Every step is test-first: RED must fail for the intended reason; all adapter tests run against local `httptest` fixture servers or real local Git — never the live GitHub API.
- Production changes: new `internal/appserver/github.go` (+ supporting adapter files/tests), `cmd/assurectl-app` wiring, `docs/guides/self-hosting.md`. No changes to the appserver core, evaluator, kernel, existing schemas, `go.mod`.
- Standard-library crypto only; the Git execution pattern stays the vetted one (allowlisted binary, sanitized environment, argument-list exec, pre-validated object ids).
- Tokens and secrets never appear in error messages or logs; base URLs are injectable.
- Cross-version gofmt (both toolchains on a clean LF view) before every push.

## File Structure

### Create

- `internal/appserver/github.go` — App JWT, installation tokens, CheckPoster/EvidenceSource/WorktreeSource/KeySource adapters.
- `internal/appserver/github_test.go` — httptest/Git-fixture adapter tests.
- `docs/guides/self-hosting.md` — the runbook.

### Modify

- `cmd/assurectl-app/main.go` (+ test) — real `Dependencies` wiring with `--api-base-url`.
- `docs/superpowers/specs/2026-11-06-m3-adapters-runbook-design.md` — status only after verified implementation.

---

### Task 1: App JWT and Installation Tokens

- [ ] **Step 1: RED** — JWT shape tests (RS256 header, `iss` = app id, `iat`/`exp` window) from a PEM key; token exchange against a fixture server (method, path, bearer); token caching until near expiry; error mapping without token leakage.
- [ ] **Step 2: Observe RED.**
- [ ] **Step 3: Implement** the auth client with injectable base URL.
- [ ] **Step 4: GREEN** + vet.
- [ ] **Step 5: Commit** `feat: authenticate the app with installation tokens`.

### Task 2: CheckPoster and EvidenceSource Adapters

- [ ] **Step 1: RED** — check-posting request shape (method/path/payload) and error mapping; artifact list/download/materialization into the bundle layout; the OIDC token relay contract; missing-artifact failure.
- [ ] **Step 2: Observe RED.**
- [ ] **Step 3: Implement** both adapters.
- [ ] **Step 4: GREEN.**
- [ ] **Step 5: Commit** `feat: post checks and download evidence from GitHub`.

### Task 3: WorktreeSource and KeySource Adapters

- [ ] **Step 1: RED** — worktree fetch with real local Git (object id fetched into a cache dir, worktree returned; unresolved revision rejected before execution; remote taken from config only); JWKS adapter (serve keys, serve by `kid`, one refresh on unknown `kid`, malformed JWKS rejected).
- [ ] **Step 2: Observe RED.**
- [ ] **Step 3: Implement** both adapters.
- [ ] **Step 4: GREEN.**
- [ ] **Step 5: Commit** `feat: fetch worktrees and JWKS keys for the app`.

### Task 4: Wiring and Self-Hosting Runbook

- [ ] **Step 1: RED** — wiring test: config builds the real `Dependencies` against fixture servers; unconfigured mode keeps the fail-closed skeleton behavior.
- [ ] **Step 2: Observe RED.**
- [ ] **Step 3: Implement** the wiring (`--api-base-url`); **write** `docs/guides/self-hosting.md` per design §4.
- [ ] **Step 4: GREEN** for the whole repository.
- [ ] **Step 5: Commit** `feat: wire the real app adapters with a self-hosting runbook`.

### Task 5: Full Verification, Scope Audit, and Documentation Alignment

- [ ] **Step 1: Focused tests** across `internal/appserver`, `cmd/assurectl-app`.
- [ ] **Step 2: Required repository verification** (CONTRIBUTING.md) + both-toolchain gofmt.
- [ ] **Step 3: Scope audit** — changed files match the Create/Modify list; no evaluator/kernel changes; no live-API calls in tests.
- [ ] **Step 4: Trust-boundary re-read** — tokens/secrets never logged; remote URLs from config only; revisions pre-validated; the runbook claims nothing unimplemented.
- [ ] **Step 5: Docs-only commit** flipping the spec status to `Implemented in M3 adapters-runbook slice` with plan checkbox updates.
- [ ] **Step 6: Push and require exact-head CI**; record head SHA and run IDs here.
- [ ] **Step 7: Final review gate** against the design, ADR-0010, foundation §11 (Tier 2), and `docs/threat-model/v0.md`.

## Exit Criteria

- All four ports have real adapters, each offline-tested against local fixture servers; the App core is untouched.
- `cmd/assurectl-app` wires the real adapters from configuration and keeps the fail-closed skeleton path when unconfigured.
- The runbook enables self-hosting end-to-end without overclaiming.
- No dependency, schema, or evaluator change; exact-head CI green before merge readiness.
