# M1 Git Subject Resolver Implementation Plan

**Status:** Implemented; final exact-head review pending  
**Date:** 2026-09-08  
**Milestone:** M1 — Local advisory evaluation  
**Scope:** exact Git subject resolution only

## Goal

Implement the smallest M1 component that resolves a revision-bound local Git subject for later advisory verification. The resolver returns canonical repository identity, exact base/head commit object IDs, the versioned `assurectl.git-change-set/v0` digest, and advisory workspace state.

This slice does **not** add `assurectl verify`, contract/policy/evidence loaders, receipt construction or acceptance, recursive submodule assurance, CI trust, signing, or authoritative completion.

## Final invariants

The implementation follows ADR-0003 and fails closed on ambiguous local Git state:

- subject identity is repository + exact base revision + exact head revision + versioned change-set digest;
- caller path must be inside the discovered worktree and both must resolve to the same absolute Git-directory identity;
- `BaseRef`/`HeadRef` inputs are limited to `HEAD`, full 40/64-character object IDs, or fully-qualified `refs/...` names; ambiguous short refs are rejected;
- the resolved checkout `HEAD` must equal the resolved subject head before the worktree can be considered clean;
- dirty worktrees remain advisory-only; `assume-unchanged`, `skip-worktree`, gitlinks, executable-bit changes, symlink-type changes, weakened stat-cache configuration, and external clean/process filters cannot produce a clean claim;
- repository-controlled fsmonitor and conversion-filter commands are not executed as part of a clean claim;
- no explicit or partial-clone lazy fetch is permitted, and replacement refs are disabled;
- Git environment/config redirection is stripped, including mixed-case variants, before subprocess execution;
- resolver Git subprocesses use a small absolute-path executable allowlist instead of resolving a bare `git` through caller-controlled `PATH`;
- malformed repository identity errors do not echo credential-bearing parser input;
- repository URI normalization preserves identity-significant transport/path distinctions unless the host is explicitly allowlisted for provider-level cross-transport equivalence;
- no caller-controlled value is passed through a shell.

## Package boundary

Only `internal/gitsubject` is introduced.

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

1. Normalize the caller path and resolve its absolute Git directory.
2. Resolve `--show-toplevel`, preserve significant filesystem whitespace, normalize symlinks, require the caller path to remain inside that root, and require caller/root Git-directory identity equality.
3. Reject bare/non-worktree inputs.
4. Accept only canonical ref inputs (`HEAD`, full object ID, or fully-qualified `refs/...`) and resolve base/head to exact commit object IDs with replacement refs and lazy fetching disabled.
5. Resolve repository identity using explicit URI > local `origin` > local advisory fallback.
6. Canonicalize supported HTTP(S)/SSH inputs without collapsing identity-significant self-hosted transport/path semantics.
7. Require checkout `HEAD == resolved head` for a clean claim.
8. Before porcelain status, conservatively report dirty for external clean/process filters, hidden index flags, gitlinks, or other unverified state.
9. Run status with `core.fsmonitor=false`, `core.filemode=true`, `core.symlinks=true`, `core.trustctime=true`, `core.checkStat=default`, and `core.ignoreStat=false`.
10. Compute ADR-0003 v0 digest:

```text
sha256(
  "assurectl.git-change-set/v0\n" +
  canonical_repository_uri + "\n" +
  full_base_object_id + "\n" +
  full_head_object_id + "\n"
)
```

11. Return `domain.Subject` plus `Dirty` and `LocalOnly` advisory metadata.

## Repository URI normalization

M1.1 deliberately supports a narrow identity surface:

- HTTP(S): `http://host/path` and `https://host/path`;
- SSH URL: explicit `ssh://git@host/path` only;
- SCP-like SSH: relative `git@host:path` only; absolute SCP-like paths are rejected;
- omitted or non-`git` SSH usernames, explicit ports, credentials/passwords, query/fragment data, percent encoding, surrounding whitespace, empty/dot/traversal segments, and malformed hosts/paths are rejected;
- parse failures return a generic error and do not echo malformed credential-bearing input;
- GitHub, GitLab, and Bitbucket public hosts are explicitly allowlisted to normalize supported transport forms to common `host/path` identity;
- generic/self-hosted hosts preserve transport/path semantics, e.g. SCP-like and `ssh://` forms do not collapse to the same identity;
- local fallback is `local://sha256/<digest>` and sets `LocalOnly=true`.

The resolver does not contact the remote and does not infer remote authority or trust.

## Git subprocess isolation

Every resolver Git subprocess is direct `exec.CommandContext` execution with a trusted absolute executable path selected from a small OS-specific allowlist. Caller `PATH` cannot select the resolver's Git binary.

The child environment/config boundary additionally:

- removes repository/object/index/worktree redirection variables;
- removes Git config injection, exec-path, namespace, trace, askpass, and SSH-command overrides case-insensitively;
- pins `GIT_CONFIG_GLOBAL` to `os.DevNull` and `GIT_CONFIG_NOSYSTEM=1`;
- pins `GIT_TERMINAL_PROMPT=0`, `GIT_NO_LAZY_FETCH=1`, `GIT_NO_REPLACE_OBJECTS=1`, and `GIT_OPTIONAL_LOCKS=0`;
- injects `core.fsmonitor=false`, `core.filemode=true`, `core.symlinks=true`, `core.trustctime=true`, `core.checkStat=default`, and `core.ignoreStat=false`;
- detects configured `filter.*.clean` / `filter.*.process` entries and reports the workspace conservatively dirty before `git status` can invoke them.

Repository-local `remote.origin.url` remains readable solely as an identity input; it does not establish authority.

## TDD and review evidence

The PR retains the full commit-by-commit RED/GREEN history. Major checkpoints include:

- RED/GREEN foundation for URI/digest helpers and `Resolve`;
- review hardening for credential leakage, fsmonitor, lazy fetch, replacement refs, trace/config redirection, and hidden index state;
- RED `2f0dc3b...` / GREEN `588f899...` for implicit SSH user and `skip-worktree`;
- RED `6b94adf...` / GREEN `5fd8502...` for checkout redirection;
- RED `148da7d...` / GREEN `15a80d9...` for conservative gitlink handling;
- RED `ad4dda0...` / GREEN `9c5644f...` for SCP absolute-path and executable-bit behavior;
- RED `9a7ac10...`, CI #85, proving repository clean-filter execution; GREEN `a024a35...` prevents it;
- RED `b80af538...`, CI #87, reproducing caller/root containment, checkout-head mismatch, ambiguous short refs, PATH hijack, self-hosted SSH identity collapse, malformed-URI credential leakage, symlink false-clean, and mixed-case environment bypasses;
- final production hardening in `c1611c5...`, `1ce181b...`, `cd98f7f...`, and `f901d83...`.

CI #91 / `34305521697` on `f901d83a2105029d3927ddf1c99ab52631aa934d` passed formatting, vet, race tests, coverage, and CLI build on both Go 1.26.x and 1.27.x before this documentation-only alignment commit.

## Verification

Required before merge:

```bash
mapfile -d '' files < <(find . -name '*.go' -type f -not -path './vendor/*' -print0)
test -z "$(gofmt -l "${files[@]}")"
go vet ./...
go test -race ./...
go build -trimpath -o /tmp/assurectl ./cmd/assurectl
```

GitHub Actions is the compatibility authority for the committed Go 1.26.x / 1.27.x matrix. Any final documentation-only head must still pass the exact-head workflow before merge.

## Exit criteria

This slice is complete only when:

- exact repository/base/head/change-set identity is deterministic;
- caller checkout cannot be rebound by repository configuration;
- a clean result is bound to the resolved head;
- ambiguous refs and repository identities fail closed;
- hidden index state, executable/symlink mode changes, stat-cache weakening, gitlinks, and external clean filters cannot produce a clean claim;
- malformed identity errors do not expose raw credential-bearing input;
- caller `PATH` cannot substitute the Git executable;
- no remote fetch, repository-controlled command execution, inherited Git trace write, or shell execution path remains in this slice;
- exact-head CI is green;
- exact-head Codex/CodeRabbit findings are addressed and all review threads are resolved.

Receipt semantics, recursive submodule assurance, and the `verify` CLI remain out of scope for PR #2.
