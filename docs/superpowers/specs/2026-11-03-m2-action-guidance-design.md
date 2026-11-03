# M2 Reference Action, Dogfooding Gate, and Workflow Guidance Design

**Status:** Draft for review  
**Date:** 2026-11-03  
**Milestone:** M2 — Trusted CI evidence (slice 3, final slice of the milestone)  
**Base commit:** `9c36194b96392a87c784845fd4c757f6666710f2`  
**Scope:** the reference GitHub Action, this repository's own CI-asserted evidence loop, and the protected-workflow guidance only

## 1. Purpose

M2's milestone row is "Trusted CI evidence envelopes, protected workflow guidance, and reference GitHub Action." Slices 1–2 built the evaluator side (waiver records; the `--ci` adapter with fail-closed subject cross-checks). This slice closes the loop: a reference GitHub Action wraps `assurectl verify --ci`; this repository's own CI produces real CI-asserted evidence for its own test runs and verifies it through the action; and a guidance document captures the protected-workflow posture the evaluator assumes but cannot enforce.

## 2. Claim boundary

The action and the dogfooding loop assert and record; they validate nothing about the runner's identity (no OIDC validation, no signing — threat model defers both). The receipt's `CI_ASSERTED` tier says "a GitHub Actions runner asserted this context and the assertions agreed with the resolved subject", nothing stronger. The guidance document states the same boundary.

## 3. Reference action (`action.yml`, composite)

- `runs.using: composite`; steps pinned to immutable SHAs (same discipline as `ci.yml`).
- Inputs: `base-ref` (required), `head-ref` (optional, default `HEAD`), `repository` (optional), `waivers-path` (optional), `go-version` (optional, default `1.27.x`), `working-directory` (optional, default `.`).
- Steps:
  1. actions/setup-go (pinned) with the `go-version` input;
  2. `go install github.com/kefyusuf/assurectl/cmd/assurectl@<pinned-ref>` — the action pins an exact module version so the verifier's identity is reproducible per action revision;
  3. run `assurectl verify --ci --base … [--head …] [--repository …] [--waivers …]` with `working-directory`, writing the receipt to `$GITHUB_WORKSPACE/assurectl-receipt.json` and streaming stderr to the job summary;
  4. actions/upload-artifact (pinned) uploads `assurectl-receipt.json`.
- The action exposes no outputs beyond the artifact and the exit code; it never masks a failing verification (no `continue-on-error`).
- The action is documentation-grade reference material: this repository's CI is its executable test.

## 4. Dogfooding gate (`.assurectl` in this repository)

This repository adopts its own protocol for its own test suite:

- `.assurectl/contract.json`: work unit `id: assurectl-self-test`, `type: commit`, objective; acceptance criterion `crit-ci-tests` referencing requirement `ci-tests`.
- `.assurectl/policy.json`: requirement `ci-tests`, `evidence_type: test-result`, `waivable: false`, `allowed_producer_types: ["github-actions"]`.
- The CI `test` job gains a final step that writes `.assurectl/evidence/ci-tests.json`: an evidence envelope with `producer.type: github-actions`, `producer.identity: repo:${GITHUB_REPOSITORY}:ref:${GITHUB_REF}`, `producer.workflow: .github/workflows/ci.yml`, `producer.workflow_revision: ${GITHUB_WORKFLOW_SHA}`, subject `{repository_uri: github.com/${GITHUB_REPOSITORY}, revision: ${GITHUB_SHA}}`, invocation timestamps from the job, `outcome` from the suite result, and an artifact (the go test output log, uploaded alongside) whose sha256 the envelope records. The envelope is written by a shell step (this is CI plumbing, not evaluator code; the evaluator consumes it like any other envelope).
- A new `verify` job needs the test job's evidence: it downloads the uploaded evidence + artifact into `.assurectl/evidence/`, then runs `uses: ./` (the reference action) with `--ci`. On `pull_request`, `GITHUB_SHA` is the merge commit and the checkout matches it, so the subject cross-checks hold; the workflow records `--base` explicitly (`HEAD~1`) rather than guessing.
- The `verify` job is non-blocking at first (`continue-on-error: true` at the job level with the failure still visible in the summary) — adopting the gate on this repository is a policy decision for the maintainer, not something this slice forces; flipping it to blocking is a one-line change recorded in the guidance.

## 5. Protected-workflow guidance (`docs/guides/protected-workflows.md`)

A guidance document (not enforcement) covering: least-privilege `permissions` (contents: read, `persist-credentials: false`), action pinning to immutable SHAs, why `pull_request` targets untrusted checkouts, artifact trust (verify only evidence whose artifacts you can re-digest), the assertion-vs-validation boundary of `CI_ASSERTED`, and what M3/M4 will add (OIDC validation, trusted-base policy, signing). The document claims no protection the tooling does not implement.

## 6. Example fixtures (`examples/ci/`)

One evidence envelope plus artifact mirroring the CI loop (producer `github-actions`, workflow identity fields populated), with a syntax/digest test mirroring `examples/basic`. The unstable-v0 status note applies unchanged.

## 7. Non-scope

- No production Go code changes (the evaluator already consumes everything this slice produces);
- no schema changes, no new ADR (the assertion boundary was established in the ci-context slice; the guidance restates it);
- no branch protection, required-check, or merge-authority behavior (M3);
- no OIDC token handling anywhere.

## 8. Testing strategy

1. `examples/ci` syntax + artifact-digest tests mirror `examples/basic` (RED first);
2. the envelope the CI step writes is validated implicitly by the `verify` job — a malformed envelope fails the job visibly;
3. action correctness is exercised end-to-end by the `verify` job itself (`uses: ./`); its exit code and receipt authority are asserted in the workflow log;
4. repository verification (gofmt/vet/test/build) unchanged; both-toolchain gofmt before push.

## 9. Boundary to M3

M3 introduces the GitHub App, trusted-base policy resolution, source-pinned required checks, and merge authority — at which point this repository's gate can flip to blocking with an authoritative receipt, and waiver provenance becomes source-checked.
