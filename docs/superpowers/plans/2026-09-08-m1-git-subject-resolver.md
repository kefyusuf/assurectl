# M1 Git Subject Resolver Implementation Plan

**Status:** Implemented; final exact-head review pending  
**Date:** 2026-09-08  
**Milestone:** M1 — Local advisory evaluation  
**Scope:** exact Git subject resolution only

## Goal

Implement the smallest M1 component that can resolve an immutable Git subject for later advisory verification. The resolver produces a canonical repository identity, exact base/head commit object IDs, the versioned `assurectl.git-change-set/v0` digest, and advisory workspace state.

This plan does **not** add `assurectl verify`, contract or policy loading, evidence loading/validation, receipt construction, semantic receipt acceptance, recursive submodule assurance, CI trust, or authoritative completion.

## Accepted invariants

The implementation follows ADR-0003 and the consolidated foundation design:

- subject identity is repository + exact base revision + exact head revision + versioned change-set digest;
- branch names and pull-request numbers are not immutable subject identity;
- repository identity resolves in precedence order: explicit caller URI, normalized local `origin`, local advisory fallback;
- only explicitly supported SSH/HTTP(S) forms are canonicalized; SSH requires the explicit `git` username, while omitted/other usernames, ports, credentials, percent-encoding, surrounding whitespace, and ambiguous paths fail closed;
- the discovered worktree root must resolve back to the same absolute Git directory as the caller path, so repository-local `core.worktree` cannot redirect resolution into another checkout;
- dirty worktrees are detectable and remain advisory-only; `assume-unchanged`, `skip-worktree`, and unverified gitlink/submodule entries are treated conservatively as dirty rather than permitting a clean claim;
- recursive submodule cleanliness is not claimed in this slice;
- no network fetch occurs during subject resolution, including partial-clone lazy fetching;
- replacement refs are disabled during revision resolution;
- repository-controlled fsmonitor hooks are disabled for all resolver Git subprocesses;
- system/global Git configuration, terminal credential prompting, Git trace redirection, Git exec/SSH overrides, and repository/object/index/worktree environment redirection cannot influence resolver subprocesses;
- Git commands use fixed argument vectors and never pass caller input through a shell;
- ambiguous or malformed repository identity and unresolved revisions fail closed.

## Package boundary

Add `internal/gitsubject` only.

Implemented API:

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

1. Resolve the absolute Git directory from the caller path, then resolve the worktree root using local Git metadata. Parse Git path output by removing only Git's record terminator; preserve significant spaces/tabs and reject unexpected record separators.
2. Normalize the discovered root and require its absolute Git-directory identity to equal the caller path's Git-directory identity. A `core.worktree` redirect into another repository/checkout fails closed.
3. Reject non-worktree/bare-repository inputs.
4. Resolve `BaseRef` and `HeadRef` to exact commit object IDs with a fixed `git rev-parse --verify --end-of-options <ref>^{commit}` argument vector while replacement refs and lazy fetching are disabled.
5. Validate resolved object IDs as lowercase 40- or 64-character hexadecimal values.
6. Resolve repository identity:
   - use explicit `RepositoryURI` when supplied;
   - otherwise read only repository-local `remote.origin.url` values;
   - normalize only the supported SSH/HTTP(S) forms;
   - fail closed when multiple distinct origin identities are ambiguous;
   - never include a rejected raw origin value in an error message;
   - if no origin exists, derive a non-path-revealing local advisory identifier from the normalized worktree root.
7. Inspect index flags and modes before porcelain status. Any `assume-unchanged` entry, `skip-worktree` entry, or gitlink mode `160000` reports `Dirty=true` conservatively. This avoids claiming nested submodule cleanliness that M1.1 does not verify. Otherwise detect tracked/untracked worktree changes with porcelain output.
8. Compute:

```text
sha256(
  "assurectl.git-change-set/v0\n" +
  canonical_repository_uri + "\n" +
  full_base_object_id + "\n" +
  full_head_object_id + "\n"
)
```

9. Return `domain.Subject` plus `Dirty` and `LocalOnly` advisory metadata.

## Repository URI normalization

M1.1 supports a deliberately small, deterministic identity surface:

- SCP-like SSH: `git@host:owner/repo.git` only;
- SSH URL: `ssh://git@host/...` only; an omitted username would inherit the operating-system login and is therefore identity-significant and rejected;
- `https://host/...` and `http://host/...` as identity inputs;
- canonical output is `host/path` with lowercase host and a trailing `.git` removed;
- ports are intentionally unsupported in this slice rather than silently discarded;
- credentials, passwords, query strings, fragments, percent-encoding, surrounding whitespace, empty repository paths, dot segments, traversal segments, and malformed hosts/paths are rejected;
- explicit caller identity uses the same canonicalization as origin identity;
- local fallback is `local://sha256/<hex-digest>` and sets `LocalOnly=true`.

The resolver does not contact the remote and does not infer hosting-provider trust.

## Git subprocess isolation

Every resolver Git subprocess is direct `exec.CommandContext` execution; there is no shell.

The subprocess boundary is intentionally narrower than the caller environment:

- repository/object/index/worktree redirection variables are removed;
- `GIT_CONFIG_GLOBAL` is pinned to `os.DevNull` and `GIT_CONFIG_NOSYSTEM=1` disables system configuration;
- `GIT_TERMINAL_PROMPT=0` prevents interactive credential prompts;
- `GIT_NO_LAZY_FETCH=1` prevents on-demand partial-clone fetches;
- `GIT_NO_REPLACE_OBJECTS=1` disables replacement refs;
- `GIT_OPTIONAL_LOCKS=0` keeps read-style inspection from optional index writes;
- `GIT_TRACE*`, `GIT_EXEC_PATH`, `GIT_NAMESPACE`, Git/SSH askpass and SSH command overrides are removed;
- `core.fsmonitor=false` is injected on each invocation so repository-local fsmonitor commands cannot execute during status/index inspection;
- worktree discovery is bound back to the caller's Git-directory identity before revision, origin, or dirty-state resolution proceeds.

Repository-local `remote.origin.url` remains deliberately readable because it is an identity input; arbitrary trust is never inferred from it.

## TDD sequence

### RED 1 — pure identity and digest contract

Tests were added before implementation for:

- HTTPS and SCP/SSH forms normalizing to the same identity;
- lowercase host / preserved repository path semantics;
- `.git` removal;
- rejection of credentials, query/fragment, dot/traversal, malformed or empty paths;
- deterministic `assurectl.git-change-set/v0` digest with an independently computed expected value;
- rejection of malformed object IDs.

### GREEN 1 — pure helpers

Implemented only the URI/OID/digest logic required by RED 1.

### RED 2 — temporary Git repository resolution

Integration tests were added using local temporary repositories for:

- exact base/head commit resolution;
- explicit URI precedence over origin;
- origin fallback;
- no-origin local advisory fallback without leaking the worktree path;
- clean versus dirty/untracked worktree state;
- non-repository and bare-repository failure;
- unresolved/invalid refs failing closed;
- repeatability for identical inputs.

### GREEN 2 — resolver

Implemented `Resolve` using fixed Git argument vectors without shell or network operations.

### RED 3 — initial review trust-boundary regressions

After Codex/CodeRabbit review, tests were added before fixes for:

- surrounding-whitespace repository identity collisions;
- identity-significant SSH username collisions;
- credential disclosure in rejected origin errors;
- inherited system/global Git configuration and terminal prompting;
- inherited Git trace/exec/SSH redirection;
- partial-clone lazy fetching;
- replacement-ref object masquerading;
- repository-controlled fsmonitor execution;
- `assume-unchanged` edits being hidden from dirty-state detection;
- significant trailing whitespace being stripped from the discovered worktree root.

The valid RED checkpoint reached `go test` with all intended review regressions failing before their production fixes.

### GREEN 3 — fail-closed Git trust boundary

The review regressions were fixed in small commits and re-run through the Go 1.26.x / 1.27.x CI matrix. Test-only process launches were also changed to `exec.CommandContext(t.Context(), ...)` per lint feedback.

### RED 4 — exact-head CodeRabbit identity and hidden-index findings

Commit `2f0dc3b244b902c3ac81593dc7ed10b35248bc04` added tests proving two exact-head findings:

- `ssh://host/path` must not collapse to the same identity as explicit `ssh://git@host/path`;
- a modified `skip-worktree` entry must not produce `Dirty=false`.

CI run `34282307380` failed at exactly those two tests after formatting and vet passed.

### GREEN 4 — explicit SSH identity and skip-worktree safety

Commit `6e274ab30983b34c7e4e8db25199ce78b0a380a0` fixed the SSH case; CI run `34282528253` then left only the skip-worktree regression failing. Commit `588f899924887488addc179309702b3eb11bb3d9` fixed `S`-tag handling, and CI run `34282754277` passed the full matrix.

### RED/GREEN 5 — caller checkout binding

Self-review reproduced a repository-local `core.worktree` redirect that could make `--show-toplevel` point at a different repository checkout. Test-only commit `6b94adf9764507ae08d18d823b7b6c691c20ab65` failed in CI run `34283606691` on that exact behavior. Commit `5fd85029ecb874f66a0ee4a08fab8a07fddb977c` binds caller/discovered Git-directory identity; CI run `34283831057` passed.

### RED/GREEN 6 — nested submodule claim boundary

Self-review also reproduced a false-clean nested state: a submodule can contain hidden worktree/index state that parent porcelain status does not establish. Test-only commit `148da7d5905f321ea94ae85aca18fd8fada1db1e` failed in CI run `34284087666` because a gitlink was still reported clean. Commit `15a80d945bae52fec8a8ec15e385d30acc70f119` treats gitlink mode `160000` as conservatively dirty; CI run `34284270829` passed the full matrix. Recursive submodule assurance remains explicitly out of scope.

## Verification

Required before merge:

```bash
mapfile -d '' files < <(find . -name '*.go' -type f -not -path './vendor/*' -print0)
test -z "$(gofmt -l "${files[@]}")"
go vet ./...
go test -race ./...
go build -trimpath -o /tmp/assurectl ./cmd/assurectl
```

Latest exact-head verification: commit `29c1587f678b8fb405aa53e1f74250d9d7f2e5ec`, GitHub Actions run `34284435745`, successful on Go 1.26.x and 1.27.x for formatting, vet, race tests, coverage, and CLI build.

GitHub Actions remains the compatibility authority for the committed Go 1.26.x / 1.27.x matrix.

## Exit criteria

This slice is complete only when:

- the resolver returns exact immutable Git subject data for local repositories;
- repository identity precedence is deterministic and tested;
- the discovered worktree cannot be redirected to a different Git-directory identity;
- dirty and local-only states are explicit rather than silently promoted;
- hidden index state and unverified gitlinks cannot produce a clean claim;
- malformed/ambiguous inputs fail closed without leaking raw credential-bearing origins;
- no remote fetch, repository-controlled fsmonitor command, inherited Git trace write, or shell execution path is introduced;
- exact-head CI is green;
- exact-head review findings are addressed and review threads are resolved.

Receipt semantics, recursive submodule assurance, and the `verify` CLI remain out of scope for this PR.
