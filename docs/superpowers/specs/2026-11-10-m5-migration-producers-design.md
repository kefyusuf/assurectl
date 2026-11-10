# M5 Migration Producer Integration Design

**Status:** Implemented in M5 migration-producers slice  
**Date:** 2026-11-10  
**Milestone:** M5 — Integration with automated migration producers  
**Base commit:** `66ec9e3e5a7d0b40b2cdbab7b34ee2a9b9042b3a`  
**Scope:** a producer integration guide, per-work-unit-type contract/policy examples, and an `assurectl init` scaffolding subcommand — no protocol, evaluator, or receipt changes

## 1. Purpose

The foundation design positions AssureCTL as "the assurance authority for software changes" — agents and migration tools propose changes, AssureCTL decides what the evidence permits. The core pipeline is complete through M4. M5 makes it usable by the tools that produce those changes: dependency upgraders, API migration codemods, framework upgraders, and security remediation bots.

The product boundary is explicit (§3): AssureCTL does not provide "a migration generator." It provides the verification gate. This slice builds the integration surface so that a migration producer can wire AssureCTL into its workflow without reading the full protocol specification.

## 2. What migration producers need

A migration tool (e.g. a dependency upgrade bot) that wants to use AssureCTL needs to:

1. know which `.assurectl/contract.json` and `.assurectl/policy.json` to write for its work-unit type;
2. know how to produce evidence envelopes its CI can verify;
3. call `assurectl verify` and interpret the receipt.

All three are possible today but require reading the full spec, writing JSON by hand, and understanding the trust model. M5 packages this knowledge.

## 3. Deliverables

### 3.1 Producer integration guide (`docs/guides/migration-producers.md`)

Per work-unit type (the six in the closed vocabulary beyond `commit` and `pull_request`):

- example contract and policy documents;
- evidence production pattern (which command to run, which artifact to attach);
- integration walkthrough (pre-merge hook, CI job, or App-posted check).

The guide claims nothing beyond implemented behavior and links to the existing protocol docs for details.

### 3.2 `assurectl init` subcommand

Scaffolds `.assurectl/contract.json` and `.assurectl/policy.json` for a given work-unit type:

```
assurectl init --type dependency-upgrade --requirement-id dep-upgrade --evidence-type dependency-result
```

- generates minimal, valid, minimal-policy documents (one requirement, non-waivable, one producer type);
- does not overwrite existing files (fail closed with a clear error);
- the caller edits the generated documents for their needs;
- flags: `--type`, `--requirement-id`, `--evidence-type`, `--output-dir` (default `.`).

This is a convenience: the generated documents are starting points, not authoritative. The design explicitly rejects making `init` the only way to create contracts — hand-authored documents remain first-class.

### 3.3 Example fixtures (`examples/migration/`)

One contract + policy + evidence envelope triple per migration work-unit type, with syntax and digest tests mirroring `examples/basic/` and `examples/ci/`.

## 4. Non-scope

- No changes to `internal/attestation`, `internal/evaluation`, `internal/receipt`, `internal/decision`, `internal/domain`, `internal/gitsubject`, `internal/verify`, `internal/appserver`, existing schemas, or `go.mod`;
- no migration generation, no LLM judge, no agent orchestration (explicit product-boundary exclusions);
- no Sigstore keyless, transparency, or retention (managed scope);
- no new dependency.

## 5. Testing strategy

Test-first for `init`: happy path per type (files created with valid schema), fail-closed on existing files, all generated documents pass `LoadLocal` validation. Example fixtures mirror existing test patterns. Integration: the generated documents work end-to-end with `assurectl verify` in a temporary Git repository.

## 6. Boundary

M5 completes the foundation §16 milestone roadmap. Post-M5 work is driven by adoption feedback, managed-scope demand, and protocol evolution — not by the original milestone table.
