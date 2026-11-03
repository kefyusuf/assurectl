# M2 CI-Asserted Verification Context Design

**Status:** Draft for review  
**Date:** 2026-11-03  
**Milestone:** M2 — Trusted CI evidence (slice 2 of the milestone)  
**Base commit:** `276593160f52acf6140e7e8dab7359a182d180e7`  
**Scope:** a typed GitHub Actions context adapter, subject cross-checks, and CI_ASSERTED advisory-tier receipts only

## 1. Purpose

Foundation design §11 Tier 1 defines CI-asserted evaluation: "A CI adapter supplies execution and producer context; authority `CI_ASSERTED`; merge suitability depends on protected workflow, identity, and policy configuration." This slice lets `assurectl verify` run inside a GitHub Actions workflow, cross-check the CI context against the resolved Git subject fail-closed, and emit receipts whose `authority` records `CI_ASSERTED`. It is the adapter layer the reference GitHub Action (slice 3) will call.

The threat model defers "CI/OIDC producer identity and protected workflow provenance" to later milestones: this slice validates nothing about who the runner is. It records what the runner *asserts* and refuses to emit a CI-asserted receipt when the assertions contradict the resolved subject. That distinction — assertion, not validation — is the claim boundary of this slice (INV-005 discipline; ADR-0007: authority is resolved outside inputs).

## 2. Non-scope

- OIDC token handling, JWT parsing, or any network call (deferred; the evaluator stays offline and std-lib only, and custom cryptography is forbidden — ADR-0006);
- workflow protection enforcement or pinning checks (guidance document in slice 3; enforcement needs trusted-base context, M3);
- evidence envelope production (workflows keep producing envelopes themselves; the evaluator only consumes them);
- any change to `internal/decision`, `internal/domain`, `internal/evaluation`, `internal/evidence`, `internal/waiver`, `schemas/*.json`, or `go.mod`.

## 3. Context adapter (`internal/cicontext`)

```go
type Context struct {
    Repository    string // GITHUB_REPOSITORY, e.g. "acme/checkout"
    SHA           string // GITHUB_SHA
    Ref           string // GITHUB_REF, e.g. "refs/pull/184/merge"
    WorkflowRef   string // GITHUB_WORKFLOW_REF, e.g. ".github/workflows/ci.yml@refs/pull/184/merge"
    WorkflowSHA   string // GITHUB_WORKFLOW_SHA
    RunID         string // GITHUB_RUN_ID
    RunAttempt    string // GITHUB_RUN_ATTEMPT
    EventName     string // GITHUB_EVENT_NAME
}

func Read(environ []string) (Context, error)
```

- `Read` consumes a caller-supplied environment map (the CLI adapter passes its own environment; the core stays environment-free per the M1 determinism rules).
- Required variables: `GITHUB_REPOSITORY`, `GITHUB_SHA`, `GITHUB_WORKFLOW_REF`, `GITHUB_WORKFLOW_SHA`, `GITHUB_RUN_ID`. Missing or empty required variables fail the read. `GITHUB_REF`, `GITHUB_RUN_ATTEMPT`, and `GITHUB_EVENT_NAME` are optional and recorded as supplied.
- `GITHUB_SHA` must be a full 40/64-hex lowercase object ID; `GITHUB_REPOSITORY` must match `^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*$`. No trimming, no case folding.
- `Context.CanonicalRepository()` returns `github.com/<owner>/<repo>` via the existing `gitsubject.CanonicalizeRepositoryIdentity` seam; a repository that fails canonicalization fails the read.

## 4. Orchestration changes (`internal/verify`)

`verify.Options` gains `CI *cicontext.Context`. When set:

1. subject cross-checks run after resolution, failing closed before any input loads:
   - `CI.CanonicalRepository()` must equal the resolved subject's repository URI;
   - `CI.SHA` must equal the resolved subject's head revision;
   - a mismatch is an error and no receipt is produced — a CI context that disagrees with the checked-out subject is exactly the fail-closed ambiguity AGENTS.md requires surfacing;
2. the receipt is built with authority `CI_ASSERTED`;
3. the dirty-worktree finding keeps its M1 semantics (a CI checkout is normally clean; the finding remains diagnostic);
4. the local-only finding cannot occur (the subject cross-check requires a hosted canonical identity that only a configured remote produces).

When `CI` is unset, behavior is byte-identical to M1/M2-slice-1: advisory authority, all current findings.

## 5. Receipt authority (`internal/receipt`)

`receipt.BuildInput` gains `CI *cicontext.Context`:

- `ADVISORY` stays the default and requires `CI == nil`;
- `CI_ASSERTED` requires `CI != nil`; the builder records the tier and nothing more — workflow identity lives in the evidence envelopes' producer blocks (existing schema fields), produced by the workflow itself;
- any other authority value remains rejected (M3);
- the semantic checks (re-derivation, evidence binding, waiver rules, RFC3339) run unchanged for both tiers; a CI-asserted receipt with untrusted workspace inputs is representable and matches Tier 1's "not automatically equivalent to an independent authority."

## 6. CLI surface

`assurectl verify --ci` (boolean flag): read the process environment through `cicontext.Read`, fail with exit 2 (diagnostics on stderr, no receipt) if the context is missing or malformed, and run the CI-asserted path. Without the flag nothing changes. The context values themselves are never echoed into error messages beyond the repository and run identity already recorded in receipts.

## 7. Determinism and fail-closed rules

- No wall clock, no environment reads outside the CLI adapter and `cicontext.Read(environ)`; repeated evaluations of deeply equal inputs stay byte-identical.
- Cross-check failures, malformed contexts, and unknown authority values return errors with no receipt.
- `GITHUB_*` variables not listed in §3 are never read: the adapter's input surface is closed.

## 8. Testing strategy

Test-first throughout: `cicontext` table tests (happy path, each missing required variable, malformed SHA, malformed repository, canonicalization); verify integration tests with temporary repositories (matching context approves with `CI_ASSERTED`, repository mismatch fails, SHA mismatch fails, advisory path unchanged byte-wise); receipt tests (CI_ASSERTED accepted with context, ADVISORY with context rejected, CI_ASSERTED without context rejected); CLI tests (`--ci` happy path in a fabricated environment, `--ci` without GitHub variables exits 2).

## 9. Boundary to slice 3

Slice 3 ships the reference GitHub Action (wrapping this CLI with `--ci`), the protected-workflow guidance document (permissions, OIDC posture, action pinning, untrusted-input hygiene), and CI evidence-envelope example fixtures whose producer blocks carry the workflow identity this slice cross-checks against.
