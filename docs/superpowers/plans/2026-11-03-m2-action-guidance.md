# M2 Reference Action and Dogfooding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Design:** `docs/superpowers/specs/2026-11-03-m2-action-guidance-design.md`  
**Base commit:** `9c36194b96392a87c784845fd4c757f6666710f2`

## Global Constraints

- Every behavior-bearing step is test-first where Go tests apply (examples); CI YAML and docs are verified by the CI run itself.
- Production Go code does not change: no `internal/**`, no `cmd/**` modifications except none; `go.mod` untouched.
- All new actions pinned to immutable SHAs; `permissions` minimal (`contents: read`); `persist-credentials: false`.
- The `verify` CI job stays non-blocking (`continue-on-error`) in this slice; flipping it is a recorded maintainer decision, not this slice's.
- Cross-version gofmt (both toolchains on a clean LF view) before push.

## File Structure

### Create

- `action.yml` — composite reference action (setup-go, pinned install, verify run, receipt artifact).
- `.assurectl/contract.json`, `.assurectl/policy.json` — this repository's own verification contract and policy.
- `examples/ci/evidence/ci-tests.json` (+ artifact) and `examples/ci/examples_syntax_test.go` — CI-shaped envelope fixture and test.
- `docs/guides/protected-workflows.md` — protected-workflow guidance.
- `docs/adr/` — none required (no protocol/trust change; the assertion boundary is restated, not extended).

### Modify

- `.github/workflows/ci.yml` — test job emits the evidence envelope + artifact; new `verify` job runs `uses: ./` with `--ci` (non-blocking).
- `examples/basic`? untouched.

## Review Focus

1. **No evaluator changes** — the diff must not touch `internal/` or `cmd/`.
2. **Envelope integrity** — the CI-written envelope's artifact digest must be computed from the exact uploaded artifact bytes; a mismatch must fail the verify job visibly.
3. **Assertion boundary preserved** — no wording anywhere claims OIDC validation, signing, or merge-grade authority.
4. **Least privilege** — the new job declares `permissions: contents: read` only and never persists credentials.

---

### Task 1: CI Example Fixture

**Files:** create `examples/ci/evidence/ci-tests.json`, artifact, `examples/ci/examples_syntax_test.go`.

- [ ] **Step 1: RED** — mirror `examples/basic`'s syntax + digest tests for `examples/ci`; the count assertion in the existing examples test covers only its own directory, so the new test is independent.
- [ ] **Step 2: Observe RED** (directory missing).
- [ ] **Step 3: Create** the envelope (producer `github-actions`, workflow identity fields, subject binding `github.com/acme/checkout`, artifact digest) and artifact.
- [ ] **Step 4: GREEN** + repository verification.
- [ ] **Step 5: Commit** `feat: add CI evidence envelope example`.

### Task 2: Reference Action

**Files:** create `action.yml`.

- [ ] **Step 1: Author** the composite action per design §3: pinned setup-go/upload-artifact, `go install …@<pinned-ref>`, the verify invocation with all documented inputs, receipt artifact upload, `working-directory` support, no `continue-on-error`.
- [ ] **Step 2: Static checks** — YAML parses (a Go test under `schemas` or a dedicated `action_test.go` may assert parseability and input names, RED first); action pinning verified by review.
- [ ] **Step 3: GREEN**, then commit `feat: add reference assurectl verify action`.

### Task 3: Repository Contract, Policy, and CI Loop

**Files:** create `.assurectl/contract.json`, `.assurectl/policy.json`; modify `.github/workflows/ci.yml`.

- [ ] **Step 1: Write** the contract and policy per design §4.
- [ ] **Step 2: Extend the `test` job** — after the suite, write the evidence envelope (subject from `GITHUB_REPOSITORY`/`GITHUB_SHA`, producer identity/workflow fields, outcome from the suite, artifact = test log with sha256 recorded) and upload evidence + artifact together.
- [ ] **Step 3: Add the `verify` job** — `needs: test`, downloads evidence + artifact into `.assurectl/evidence/`, runs `uses: ./` with `--ci`, `--base HEAD~1 --head HEAD --repository github.com/${GITHUB_REPOSITORY}`, uploads the receipt; job-level `continue-on-error: true` with the decision visible in the summary; `permissions: contents: read`.
- [ ] **Step 4: CI observation** — push and confirm the `verify` job runs, emits a `CI_ASSERTED` receipt, and its evidence binding passes the fail-closed subject cross-checks; a first-run failure here is diagnosis material, not something to paper over.
- [ ] **Step 5: Commit** `feat: verify this repository with its own CI evidence`.

### Task 4: Protected-Workflow Guidance

**Files:** create `docs/guides/protected-workflows.md`.

- [ ] **Step 1: Write** the guidance per design §5 (permissions, pinning, untrusted checkouts, artifact trust, assertion-vs-validation boundary, M3/M4 outlook).
- [ ] **Step 2: Review pass** — no claim exceeds implemented behavior.
- [ ] **Step 3: Commit** `docs: add protected workflow guidance`.

### Task 5: Full Verification, Scope Audit, and Documentation Alignment

- [ ] **Step 1: Repository verification** (CONTRIBUTING.md) + both-toolchain gofmt.
- [ ] **Step 2: Scope audit** — no `internal/`/`cmd/` changes; no dependency; no schema change.
- [ ] **Step 3: Trust-boundary re-read** — no enforcement claims; non-blocking gate recorded; assertion wording consistent.
- [ ] **Step 4: Docs-only commit** flipping the spec status to `Implemented in M2 action-guidance slice` with plan checkbox updates.
- [ ] **Step 5: Push and require exact-head CI** — this slice's own `verify` job is part of the evidence; record head SHA and run IDs here.
- [ ] **Step 6: Final review gate** against the design, foundation §11 (Tier 1), ADR-0006, ADR-0007, and `docs/threat-model/v0.md`.

## Exit Criteria

- The reference action exists, is pinned, least-privileged, and passes no failure silently.
- This repository's CI produces a real CI-asserted evidence envelope and verifies it through the action; the receipt is uploaded and its authority is `CI_ASSERTED`.
- The guidance document claims nothing beyond implemented behavior.
- No production Go code, schema, or dependency changed; exact-head CI green (including the new `verify` job) before merge readiness.
