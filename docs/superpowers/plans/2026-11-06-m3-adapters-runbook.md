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

- [x] **Step 1: RED** — JWT shape tests (RS256 header, `iss` = app id, `iat`/`exp` window) from a PEM key; token exchange against a fixture server (method, path, bearer); token caching until near expiry; error mapping without token leakage.
- [x] **Step 2: Observe RED.**
- [x] **Step 3: Implement** the auth client with injectable base URL.
- [x] **Step 4: GREEN** + vet.
- [x] **Step 5: Commit** `feat: authenticate the app with installation tokens`.

### Task 2: CheckPoster and EvidenceSource Adapters

- [x] **Step 1: RED** — check-posting request shape (method/path/payload) and error mapping; artifact list/download/materialization into the bundle layout; the OIDC token relay contract; missing-artifact failure.
- [x] **Step 2: Observe RED.**
- [x] **Step 3: Implement** both adapters.
- [x] **Step 4: GREEN.**
- [x] **Step 5: Commit** `feat: post checks and download evidence from GitHub`.

### Task 3: WorktreeSource and KeySource Adapters

- [x] **Step 1: RED** — worktree fetch with real local Git (object id fetched into a cache dir, worktree returned; unresolved revision rejected before execution; remote taken from config only); JWKS adapter (serve keys, serve by `kid`, one refresh on unknown `kid`, malformed JWKS rejected).
- [x] **Step 2: Observe RED.**
- [x] **Step 3: Implement** both adapters.
- [x] **Step 4: GREEN.**
- [x] **Step 5: Commit** `feat: fetch worktrees and JWKS keys for the app`.

### Task 4: Wiring and Self-Hosting Runbook

- [x] **Step 1: RED** — wiring test: config builds the real `Dependencies` against fixture servers; unconfigured mode keeps the fail-closed skeleton behavior.
- [x] **Step 2: Observe RED.**
- [x] **Step 3: Implement** the wiring (`--api-base-url`); **write** `docs/guides/self-hosting.md` per design §4.
- [x] **Step 4: GREEN** for the whole repository.
- [x] **Step 5: Commit** `feat: wire the real app adapters with a self-hosting runbook`.

### Task 5: Full Verification, Scope Audit, and Documentation Alignment

- [x] **Step 1: Focused tests** across `internal/appserver`, `cmd/assurectl-app`.
- [x] **Step 2: Required repository verification** (CONTRIBUTING.md) + both-toolchain gofmt.
- [x] **Step 3: Scope audit** — changed files match the Create/Modify list; no evaluator/kernel changes; no live-API calls in tests.
- [x] **Step 4: Trust-boundary re-read** — tokens/secrets never logged; remote URLs from config only; revisions pre-validated; the runbook claims nothing unimplemented.
- [x] **Step 5: Docs-only commit** flipping the spec status to `Implemented in M3 adapters-runbook slice` with plan checkbox updates.
- [x] **Step 6: Push and require exact-head CI**; record head SHA and run IDs here.
  - Final green run: `42280201190` (Go 1.26.x pass, Go 1.27.x pass, blocking `Verify with own evidence` pass) on head `cb12ab9`;
  - formatting: pass with BOTH toolchains (1.27 local + Go 1.26.10 on a clean LF view);
  - vet: pass; race-test: pass (ubuntu-latest); both builds: pass.
  - The acceptance loop caught two real defects, neither by loosening: (1) the `NewGitHubClient` export never reached a commit (local vet passed against the working tree; CI caught the uncommitted rename — recorded as a process reminder to `git status` before every commit), and (2) the worktree adapter fixture lacked a Git identity, which only surfaces on CI hosts without one (fixed by configuring the fixture).
- [x] **Step 7: Final review gate** against the design, ADR-0010, foundation §11 (Tier 2), and `docs/threat-model/v0.md`.
  - Review performed: changed files match the plan's Create/Modify list exactly; adapter tests never touch the live API; the server-side Git execution was moved INTO the vetted package (single exec site; a Mimosa pre-write block drove this strictly better home); tokens and response bodies never reach errors; the runbook mirrors implemented flags and permissions; the fail-closed skeleton mode is preserved when unconfigured. A pre-commit Mimosa scan flags pre-existing gitsubject exec patterns (command injection) and a cross-file-taint heuristic on the CLI entry (`os.Args -> run()`); assessed as scanner false positives.

## Exit Criteria

- All four ports have real adapters, each offline-tested against local fixture servers; the App core is untouched.
- `cmd/assurectl-app` wires the real adapters from configuration and keeps the fail-closed skeleton path when unconfigured.
- The runbook enables self-hosting end-to-end without overclaiming.
- No dependency, schema, or evaluator change; exact-head CI green before merge readiness.
