# ADR-0003: Bind Decisions to Exact Repository Revisions

**Status:** Accepted  
**Date:** 2026-09-02

## Context

Evidence can be valid for one commit and irrelevant after another commit is added. Pull request numbers, branch names, and human-readable descriptions are mutable identifiers. Hashing a textual `git diff` introduces behavior dependent on formatting, rename detection, configuration, and Git version.

## Decision

A software-change subject identifies:

- canonical repository URI;
- full base Git object ID;
- full head Git object ID;
- versioned change-set algorithm;
- change-set digest.

The v0 digest is:

```text
sha256(
  "assurectl.git-change-set/v0\n" +
  canonical_repository_uri + "\n" +
  full_base_object_id + "\n" +
  full_head_object_id + "\n"
)
```

The algorithm identifier is part of the preimage. New commits make evidence for a different head revision stale. Authoritative evaluation requires an immutable committed source state; dirty worktrees are advisory only.

## Consequences

- Receipts can be compared against exact merge or release candidates.
- Change-set identity is independent of textual diff presentation.
- Repository normalization and object-format handling require conformance tests.
- A future digest algorithm must use a new identifier.

## Rejected Alternatives

- **Pull request number only:** mutable and provider-specific.
- **Branch name:** movable reference.
- **Textual diff hash:** not reliably canonical across environments.
- **Head commit only:** omits the trusted starting point of the evaluated change.
