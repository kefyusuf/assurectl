# AssureCTL M0 Foundation Implementation Plan

**Status:** Superseded after implementation review  
**Original plan commit:** `af22fbc80ac91ba734d3d2b1dcc8f905cb3cf499`  
**Current implementation:** pull request `#1` and `2026-09-02-m0-foundation-execution-notes.md`

The original task-by-task plan is retained in Git history. It was superseded because implementation review accepted several security and maintenance refinements that made its literal instructions stale:

- GitHub Actions are pinned to immutable commit SHAs rather than mutable major tags.
- Go dependency caching is disabled while the zero-dependency module has no `go.sum`.
- ADR-0007 establishes that policy, contract, and evidence payloads cannot declare their own authority.
- Receipt policy provenance records every contributing policy source rather than a single source string.
- Required build commands write binaries outside the worktree.
- The accepted ADR count increased from six to seven.

For current M0 state and verification commands, use:

1. `docs/superpowers/specs/2026-09-02-assurectl-foundation-design.md`;
2. accepted ADRs in `docs/adr/`;
3. `docs/superpowers/plans/2026-09-02-m0-foundation-execution-notes.md`;
4. `CONTRIBUTING.md`;
5. the pull-request checks on PR #1.

Do not execute stale command snippets from the historical plan.
