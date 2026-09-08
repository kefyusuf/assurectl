# M1 Git Subject Resolver Implementation Plan

**Status:** Active  
**Date:** 2026-09-08  
**Milestone:** M1 — Local advisory evaluation  
**Scope:** exact Git subject resolution only

## Goal

Implement the smallest M1 component that can resolve an immutable Git subject for later advisory verification. The resolver produces a canonical repository identity, exact base/head commit object IDs, the versioned `assurectl.git-change-set/v0` digest, and advisory workspace state.

This plan does **not** add `assurectl verify`, contract or policy loading, evidence loading/validation, receipt construction, semantic receipt acceptance, CI trust, or authoritative completion.

## Accepted invariants

The implementation follows ADR-0003 and the consolidated foundation design:

- subject identity is repository + exact base revision + exact head revision + versioned change-set digest;
- branch names and pull-request numbers are not immutable subject identity;
- repository identity resolves in precedence order: explicit caller URI, normalized local `origin`, local advisory fallback;
- common SSH and HTTPS forms for the same hosted repository normalize to one canonical identity;
- dirty worktrees are detectable and remain advisory-only;
- no network fetch occurs during subject resolution;
- Git commands use fixed argument vectors and never pass caller input through a shell;
- ambiguous or malformed repository identity and unresolved revisions fail closed.

## Package boundary

Add `internal/gitsubject` only.

Planned API:

```go
type Options struct {
    RepositoryURI string
    BaseRef       string
    HeadRef       string
}

type Resolution struct {
    Subject   domain.Subject
    Dirty     bool
    LocalOnly bool
}

func Resolve(ctx context.Context, worktree string, opts Options) (Resolution, error)
```

No public Go API is introduced.

## Resolution algorithm

1. Resolve and validate the repository worktree root using local Git metadata.
2. Reject non-worktree/bare-repository inputs.
3. Resolve `BaseRef` and `HeadRef` to exact commit object IDs with a fixed `git rev-parse --verify --end-of-options <ref>^{commit}` argument vector.
4. Validate resolved object IDs as lowercase 40- or 64-character hexadecimal values.
5. Resolve repository identity:
   - use explicit `RepositoryURI` when supplied;
   - otherwise read only repository-local `remote.origin.url` values;
   - normalize supported hosted SSH/HTTPS forms;
   - fail closed when multiple distinct origin identities are ambiguous;
   - if no origin exists, derive a non-path-leaking local advisory identifier from the normalized worktree root.
6. Detect tracked/untracked worktree changes with porcelain output.
7. Compute:

```text
sha256(
  "assurectl.git-change-set/v0\n" +
  canonical_repository_uri + "\n" +
  full_base_object_id + "\n" +
  full_head_object_id + "\n"
)
```

8. Return `domain.Subject` plus `Dirty` and `LocalOnly` advisory metadata.

## Repository URI normalization

M1.1 supports a deliberately small, deterministic identity surface:

- SCP-like SSH: `git@host:owner/repo.git`;
- `ssh://[user@]host/...`;
- `https://host/...` and `http://host/...` as identity inputs;
- canonical output is `host/path` with lowercase host and a trailing `.git` removed;
- credentials, query strings, fragments, empty repository paths, dot segments, and traversal segments are rejected;
- explicit caller identity uses the same canonicalization as origin identity;
- local fallback is `local://sha256/<hex-digest>` and sets `LocalOnly=true`.

The resolver does not contact the remote and does not infer hosting-provider trust.

## TDD sequence

### RED 1 — pure identity and digest contract

Add tests before implementation for:

- HTTPS and SCP/SSH forms normalizing to the same identity;
- lowercase host / preserved repository path semantics;
- `.git` removal;
- rejection of credentials, query/fragment, dot/traversal, malformed or empty paths;
- deterministic `assurectl.git-change-set/v0` digest with an independently computed expected value;
- rejection of malformed object IDs.

Verify the tests fail because `internal/gitsubject` behavior does not exist.

### GREEN 1 — pure helpers

Implement only enough URI/OID/digest logic to satisfy RED 1.

### RED 2 — temporary Git repository resolution

Add integration tests using local temporary repositories for:

- exact base/head commit resolution;
- explicit URI precedence over origin;
- origin fallback;
- no-origin local advisory fallback without leaking the worktree path;
- clean versus dirty/untracked worktree state;
- non-repository and bare-repository failure;
- unresolved/invalid refs failing closed;
- repeatability for identical inputs.

### GREEN 2 — resolver

Implement `Resolve` using `os/exec` with fixed Git argument vectors. Do not invoke a shell or perform network operations.

## Verification

Required before PR review:

```bash
mapfile -d '' files < <(find . -name '*.go' -type f -not -path './vendor/*' -print0)
test -z "$(gofmt -l "${files[@]}")"
go vet ./...
go test -race ./...
go build -trimpath -o /tmp/assurectl ./cmd/assurectl
```

GitHub Actions remains the compatibility authority for the committed Go 1.26.x / 1.27.x matrix.

## Exit criteria

This slice is complete only when:

- the resolver returns exact immutable Git subject data for local repositories;
- repository identity precedence is deterministic and tested;
- dirty and local-only states are explicit rather than silently promoted;
- malformed/ambiguous inputs fail closed;
- no network or shell execution path is introduced;
- exact-head CI is green;
- review findings are addressed against the current head.

Receipt semantics and the `verify` CLI remain out of scope for this PR.