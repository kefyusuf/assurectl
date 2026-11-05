# M3 OSS App Skeleton Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Design:** `docs/superpowers/specs/2026-11-06-m3-app-skeleton-design.md`  
**Base commit:** `e8d982137f13c8f702ea017e83add98add935e40`

## Global Constraints

- Every step is test-first: RED must fail for the intended reason before implementation; all tests are offline (fixture ports, in-process RSA keys).
- Production changes: new `internal/appserver`, new `cmd/assurectl-app`, new `schemas/check-payload.v0.schema.json` (+ schema-syntax test count 5 → 6). No changes to `internal/evaluation`, `internal/receipt`, `internal/gitsubject`, `internal/evidence`, `internal/waiver`, `internal/decision`, `internal/domain`, existing schemas, `go.mod`.
- No network access in the core or its tests; standard-library crypto only (HMAC-SHA256, RSA PKCS#1 v1.5 SHA-256); no custom cryptography.
- Secrets appear in config parsing only; never logged, never echoed.
- Cross-version gofmt (both toolchains on a clean LF view) before every push.

## File Structure

### Create

- `schemas/check-payload.v0.schema.json` — the App's check-run payload draft.
- `internal/appserver/payload_test.go`, `internal/appserver/webhook.go` + `webhook_test.go`, `internal/appserver/pipeline.go` + `pipeline_test.go`, `internal/appserver/oidc.go` + `oidc_test.go`.
- `cmd/assurectl-app/main.go` + `main_test.go` — wiring skeleton.

### Modify

- `schemas/schema_syntax_test.go` — draft count 5 → 6 and the payload-shape test.
- `docs/superpowers/specs/2026-11-06-m3-app-skeleton-design.md` — status only after verified implementation.

---

### Task 1: Check Payload Schema

- [ ] **Step 1: RED** — schema-syntax count 5 → 6; payload-shape test (`name: "assurectl"`, conclusion enum, receipt-reference block with change-set digest + receipt sha256 + authority).
- [ ] **Step 2: Observe RED.**
- [ ] **Step 3: Write** the draft.
- [ ] **Step 4: GREEN.**
- [ ] **Step 5: Commit** `feat: define the assurectl check payload protocol`.

### Task 2: Webhook Surface

- [ ] **Step 1: RED** — HMAC table (valid signature accepts, missing/malformed/wrong-secret rejected with 401); routing table (`pull_request` opened/synchronize → core called, other actions and events → ignored 200, core not called); no secret material in error output.
- [ ] **Step 2: Observe RED.**
- [ ] **Step 3: Implement** `webhook.go` (signature verification, routing, JSON event parsing).
- [ ] **Step 4: GREEN.**
- [ ] **Step 5: Commit** `feat: verify and route app webhooks`.

### Task 3: Evaluation Pipeline Core

- [ ] **Step 1: RED** — pipeline tests with fixture ports: approved path emits an AUTHORITATIVE check payload carrying the receipt digests; unvalidated producer caps the payload to CI_ASSERTED claims; REJECTED/BLOCKED map to failure/neutral conclusions; worktree/evidence failures produce errors with no payload; byte-identical payloads for identical fixture events.
- [ ] **Step 2: Observe RED.**
- [ ] **Step 3: Implement** `pipeline.go` composing gitsubject/LoadBase/evidence/evaluation/receipt through the injected ports.
- [ ] **Step 4: GREEN.**
- [ ] **Step 5: Commit** `feat: compose the app evaluation pipeline`.

### Task 4: OIDC Validator

- [ ] **Step 1: RED** — in-process RSA keys sign fixture tokens: valid token yields validated claims; each wrong binding claim (`repository`, `workflow_sha`, `sha`, `iss`, `aud`) fails; expired fails; wrong signature fails; missing token yields the unvalidated outcome.
- [ ] **Step 2: Observe RED.**
- [ ] **Step 3: Implement** `oidc.go` with the injectable `KeySource`.
- [ ] **Step 4: GREEN.**
- [ ] **Step 5: Commit** `feat: validate CI producer identity with standard OIDC`.

### Task 5: Command Skeleton

- [ ] **Step 1: RED** — flag/config tests: missing required flags exit 64 with usage; secrets read from files; `--listen` default; the HTTP handler is mounted at `/webhook`.
- [ ] **Step 2: Observe RED.**
- [ ] **Step 3: Implement** `cmd/assurectl-app`.
- [ ] **Step 4: GREEN** for the whole repository.
- [ ] **Step 5: Commit** `feat: add the assurectl-app command skeleton`.

### Task 6: Full Verification, Scope Audit, and Documentation Alignment

- [ ] **Step 1: Focused tests** across `internal/appserver`, `cmd/assurectl-app`, `schemas`.
- [ ] **Step 2: Required repository verification** (CONTRIBUTING.md) + both-toolchain gofmt.
- [ ] **Step 3: Scope audit** — changed files match the Create/Modify list; no network calls; no secret logging; dogfooding CI loop unchanged.
- [ ] **Step 4: Trust-boundary re-read** — unvalidated producer context caps at CI_ASSERTED; no verdict caching; no signing; no storage.
- [ ] **Step 5: Docs-only commit** flipping the spec status to `Implemented in M3 app-skeleton slice` with plan checkbox updates.
- [ ] **Step 6: Push and require exact-head CI**; record head SHA and run IDs here.
- [ ] **Step 7: Final review gate** against the design, ADR-0010, foundation §11 (Tier 2), and `docs/threat-model/v0.md`.

## Exit Criteria

- The App's decision-making core is pure, offline-tested, and composes the existing evaluator unchanged through four ports.
- The check payload protocol has a schema draft with fixture tests; conclusions map exactly like CLI exit codes.
- OIDC validation uses standard cryptography with an injectable key source; unvalidated context never produces AUTHORITATIVE checks.
- The command skeleton mounts the webhook surface with secrets handled fail-closed.
- No dependency, kernel, evaluator, or existing-schema change; exact-head CI green before merge readiness.
