# AssureCTL Foundation Design

**Status:** Accepted for M0 implementation  
**Date:** 2026-09-02  
**Repository:** `github.com/kefyusuf/assurectl`  
**License:** Apache-2.0

## 1. Executive summary

AssureCTL is an open, deterministic, revision-bound assurance authority for software changes. It evaluates whether an exact software change is permitted to move to a completion state under an explicit verification contract, effective policy, and machine-verifiable evidence.

AssureCTL is not a test runner, CI platform, coding agent, AI reviewer, or guarantee that software is correct, secure, bug-free, or production-ready. Evidence producers execute checks. AssureCTL validates the subject binding, provenance, freshness, completeness, integrity, and observed outcomes of that evidence; preserves technical findings and risk acceptance as separate facts; and emits a machine-readable completion decision.

The first product surface is a local Go CLI with advisory authority. Later milestones add trusted CI evidence, a GitHub App that publishes a source-pinned required check, and in-toto/DSSE/Sigstore-aligned signed attestations. The long-term integration point is self-maintaining software: automated API, SDK, framework, dependency, and security migrations produce changes while AssureCTL determines what the evidence permits.

## 2. Normative precedence

This document defines the current foundation direction. When details conflict, use this order:

1. accepted ADRs in `docs/adr/`;
2. this consolidated foundation design;
3. active milestone implementation plans;
4. examples and unstable v0 schemas.

Historical amendments remain as decision history but do not need to be applied separately when their rules have been incorporated here. Accepted decisions are superseded through new ADRs rather than silently reinterpreted.

## 3. Product boundary

**Category:** software change assurance  
**Positioning:** open, revision-bound assurance authority for software changes  
**Tagline:** Agents propose changes. AssureCTL decides what the evidence permits.

Initial work-unit types are limited to software delivery:

- pull requests;
- commits;
- release candidates;
- API and SDK migrations;
- framework and dependency upgrades;
- security remediations.

AssureCTL v0 does not provide:

- a general CI runner or arbitrary command-execution service;
- an agent orchestrator or multiplayer agent environment;
- an LLM-based authoritative judge;
- a migration generator;
- a SaaS dashboard, billing system, or organization administration plane;
- custom cryptography, private PKI, or self-rooted certificates;
- arbitrary policy scripts, embedded shell expressions, or a general-purpose policy language.

## 4. Core invariants

### INV-001 — Independent authority

The worker that proposes or produces a change must not be the authority that marks it complete.

### INV-002 — Immutable subject binding

An authoritative decision identifies the canonical repository, exact base revision, exact head revision, and versioned change-set digest it evaluated.

### INV-003 — Trusted intent

An authoritative decision requires a verification contract whose source identity and digest are established outside the contract payload. An agent may suggest acceptance criteria but may not silently replace the trusted contract used to evaluate its work.

### INV-004 — Trusted policy

A candidate head revision may not weaken the policy used to evaluate itself. Authoritative project policy resolves from the trusted base revision and may be constrained further by protected organization policy.

### INV-005 — Evidence is not prose

An agent summary, transcript, status field, or self-authored `passed` file is not merge-grade evidence.

### INV-006 — Integrity is not provenance

A matching digest proves byte integrity only. Trusted evidence also requires externally resolved producer identity, execution context, subject revision, invocation identity, timing, and outcome.

### INV-007 — Freshness is first-class

Evidence for another subject revision or outside its permitted validity window is stale. A waiver cannot make stale evidence current.

### INV-008 — Waivers preserve technical truth

A waiver is explicit risk acceptance. It never changes evidence state, observed outcome, findings, or verification verdict.

### INV-009 — Protocol invariants are non-waivable

Subject binding, authority separation, trusted policy resolution, receipt integrity, and required producer identity cannot be waived.

### INV-010 — Deterministic evaluation

The same normalized subject, contract, effective policy, evidence envelopes, waiver set, and evaluator version produce the same findings, verdict, and completion decision.

### INV-011 — Fail closed

Malformed inputs, unsupported schema versions, ambiguous revisions, unresolved authority, policy conflicts, and internal evaluation errors cannot result in approval.

### INV-012 — Claim discipline

A receipt states only that an identified subject was evaluated under an identified contract and policy using identified evidence. It does not assert universal correctness or safety.

## 5. Domain vocabulary

| Concept | Definition |
|---|---|
| `WorkUnit` | The declared unit of software change whose completion is evaluated. |
| `Subject` | The immutable repository and revision identity under evaluation. |
| `AcceptanceCriterion` | A required observable outcome associated with the work unit. |
| `VerificationRequirement` | Conditions governing required evidence type, trust, freshness, subject, and outcome. |
| `VerificationContract` | The digest-bound acceptance criteria and verification requirements for a work unit. |
| `EvidenceEnvelope` | An observation with subject, producer, invocation, outcome, artifact, and provenance metadata. |
| `Finding` | A normalized fact discovered during validation or evaluation. |
| `Waiver` | Authorized, scoped, expiring risk acceptance for an explicitly waivable failure. |
| `VerificationVerdict` | Aggregate technical result before risk acceptance. |
| `CompletionDecision` | Lifecycle decision after policy and valid waivers are applied without rewriting the verdict. |
| `Receipt` | Machine-readable record of resolved inputs, findings, verdict, waivers, decision, and verifier identity. |

A `WorkUnit` is not synonymous with a pull request. A pull request is one integration surface; the same core model also applies to commits, releases, and migrations.

## 6. Subject and revision model

The minimum Git subject is:

```yaml
subject:
  repository_uri: github.com/acme/checkout
  base_revision: 2a5165d0f3f9...
  head_revision: 948ab31d0c42...
  change_set_algorithm: assurectl.git-change-set/v0
  change_set_digest: 7f920d...
```

Repository identity resolves in this order:

1. explicit repository URI supplied by the caller;
2. normalized `origin` remote URI;
3. local advisory identifier derived from the worktree root when no remote exists.

Common SSH and HTTPS forms for the same hosted repository normalize to one identity. Local-only identifiers are never eligible for authoritative mode.

For Git subjects, v0 computes:

```text
sha256(
  "assurectl.git-change-set/v0\n" +
  canonical_repository_uri + "\n" +
  full_base_object_id + "\n" +
  full_head_object_id + "\n"
)
```

The algorithm identifier is part of the preimage and receipt. This avoids dependence on textual diff formatting, rename detection, local Git configuration, or Git version. New commits make evidence for a different head revision stale. Dirty worktrees may be evaluated only with advisory authority.

## 7. Verification contract and contract authority

The verification contract payload contains software intent and requirements, not its own trust claim:

```yaml
schema_version: assurectl/verification-contract/v0
work_unit:
  id: migration-184
  type: api_migration
  objective: Migrate checkout integration to the supported API contract.
acceptance_criteria:
  - id: AC-001
    statement: Existing payment behavior remains covered by the required test suite.
    requirements:
      - unit-tests
      - integration-tests
```

The contract does **not** contain an `authority`, `trusted`, or equivalent self-promotion field. Its authority is resolved externally by the loader or integration from source context such as:

- an explicitly advisory local workspace;
- an approved issue or protected repository metadata source;
- a trusted base revision;
- a protected organization control plane;
- a signed artifact whose issuer is pinned by external policy.

Contract content trust is separate from evidence trust and overall receipt authority. If authoritative contract provenance cannot be established, the evaluator may produce advisory findings but authoritative completion is blocked. Receipts record the externally resolved contract source, digest, and trust status.

## 8. Policy model and policy authority

Policy payloads define typed constraints but do not declare themselves authoritative. Effective policy layers are:

1. protocol baseline — built into the evaluator and non-waivable;
2. organization policy — protected external control plane in a later milestone;
3. project policy — resolved from the trusted base revision for authoritative evaluation;
4. assurance profile — referenced by trusted policy;
5. work-unit contract — may add requirements but may not weaken upstream constraints.

Deterministic merge rules are monotonic:

- required verification requirements accumulate by set union;
- minimum producer trust may only increase;
- maximum evidence age may only decrease;
- a requirement may become non-waivable downstream but not become waivable;
- permitted producer identities may narrow unless upstream policy explicitly delegates extension;
- unknown or non-orderable conflicts produce a finding and block completion.

A policy-changing pull request is evaluated under the previously trusted policy. Its proposed policy becomes eligible only after acceptance into a trusted base. Local workspace policy is advisory because of its source context, not because it labels itself advisory.

## 9. Evidence model and evidence authority

An evidence envelope carries an observation and provenance metadata:

```yaml
schema_version: assurectl/evidence-envelope/v0
id: ev-unit-tests-01
type: test-result
subject:
  repository_uri: github.com/acme/checkout
  revision: 948ab31d0c42...
producer:
  type: github-actions
  identity: repo:acme/checkout:ref:refs/pull/184/merge
  workflow: .github/workflows/test.yml
  workflow_revision: 2a5165d0f3f9...
invocation:
  command_id: test.unit
  environment_digest:
    algorithm: sha256
    value: 31fd...
  started_at: 2026-09-02T09:10:00Z
  finished_at: 2026-09-02T09:12:14Z
outcome:
  status: PASSED
  exit_code: 0
artifact:
  uri: artifacts/unit-tests.json
  digest:
    algorithm: sha256
    value: 8aa3...
  media_type: application/json
```

Producer identity inside the envelope is a claim to validate, not a trust root. Trust is resolved from external execution and identity context.

Evidence validation produces one state:

- `VALID`: required schema, integrity, subject, producer, invocation, and freshness conditions are satisfied;
- `INVALID`: malformed, internally inconsistent, or fails integrity validation;
- `MISSING`: required evidence is absent;
- `STALE`: otherwise usable evidence belongs to another revision or validity window;
- `UNTRUSTED`: integrity may be intact but producer or execution provenance is not permitted.

Evidence state is distinct from observed outcome. `VALID` evidence may report `FAILED`; the initial observed outcomes are `PASSED`, `FAILED`, and `ERROR`.

Artifact resolution rejects absolute paths, traversal segments, escaping symlinks, duplicate normalized paths, unsafe platform aliases, and unsupported URI schemes. The core evaluator performs no arbitrary network fetch in v0.

## 10. Findings, verdicts, waivers, and decisions

A finding contains a stable code, category, severity, affected requirement, relevant evidence identifiers, bounded explanation, and deterministic disposition. Machine consumers depend on stable codes rather than message text.

The aggregate technical verdict is computed before waivers:

- `FAILED` when at least one required verification has an established failed outcome;
- otherwise `INDETERMINATE` when at least one required verification cannot be established because evidence is missing, stale, invalid, untrusted, errored, or authority is unresolved;
- otherwise `PASSED`.

A known failure remains visible even when another requirement is indeterminate. Indeterminate requirements still block completion.

A waiver identifies its exact target, repository and work-unit scope, optional revision scope, reason, accepted risk, approver identity and authority, issue and expiry times, and later integrity metadata. It is applicable only when policy marks the failure waivable and the waiver is valid for the exact scope.

Completion decision precedence is:

1. `BLOCKED` when any required verification is indeterminate, policy or contract is invalid, authority is unresolved, or a protocol invariant cannot be established;
2. `APPROVED` when every required verification is established and passed and no blocking finding exists;
3. `ACCEPTED_WITH_RISK` when every required verification is established, at least one failed, every failure is explicitly waivable, and every failure has a valid authorized waiver;
4. otherwise `REJECTED`.

`ACCEPTED_WITH_RISK` preserves `VerificationVerdict: FAILED`. A valid-waiver claim on a non-waivable requirement is malformed and fails closed.

## 11. Trust tiers

### Tier 0 — Local advisory

The local CLI evaluates workspace inputs and produces unsigned advisory results.

- authority: `ADVISORY`;
- suitable for developer and agent feedback;
- not merge-grade;
- dirty worktrees and local-only repository identities cannot produce authoritative approval.

### Tier 1 — CI asserted

A CI adapter supplies execution and producer context.

- authority: `CI_ASSERTED`;
- merge suitability depends on protected workflow, identity, and policy configuration;
- not automatically equivalent to an independent authority.

### Tier 2 — Independent repository authority

A GitHub App resolves trusted policy and contract sources, validates evidence, and publishes a required check whose expected source is pinned to that App.

- authority: `AUTHORITATIVE`;
- intended merge-grade integration;
- signing and durable evidence retention are later managed capabilities.

## 12. Receipt and attestation direction

The v0 receipt is unsigned JSON and makes no cryptographic authority claim. It records:

- subject and work-unit identity;
- externally resolved contract source, digest, and trust status;
- externally resolved effective policy sources, digest, and trust status;
- references and digests for evidence that was actually supplied;
- per-requirement evidence states and observed outcomes, including `MISSING` without fabricated evidence;
- findings;
- applied and rejected waivers;
- verification verdict;
- completion decision;
- authority tier;
- evaluator name, version, protocol version, and evaluation timestamp.

Future signed receipts use an in-toto Statement with an AssureCTL-specific predicate, wrapped in DSSE and signed through a standard identity mechanism such as Sigstore/OIDC. The project does not define a custom signature primitive or accept a key embedded by the worker as its own trust root.

## 13. Core architecture

```text
CLI / adapter
     |
     v
subject resolver ---- contract loader
     |                       |
     +-----------+-----------+
                 v
           policy resolver
                 |
                 v
          evidence loader
                 |
                 v
        evidence validator
                 |
                 v
       verification engine
                 |
                 v
          waiver engine
                 |
                 v
       completion decision
                 |
                 v
          receipt builder
```

Responsibilities remain separate:

- CLI: command routing, human/JSON output, and exit-code mapping; no domain decisions;
- subject resolver: repository identity, exact Git state, and versioned change-set digest;
- contract loader: parsing, normalization, digesting, and externally supplied source context;
- policy resolver: monotonic combination of protected layers and effective-policy digest;
- evidence loader: safe discovery of envelopes and referenced artifacts;
- evidence validator: schema, path, digest, subject, provenance, invocation, and freshness validation;
- verification engine: requirement results and findings;
- waiver engine: authorization, scope, expiry, and policy permission without mutating facts;
- completion decision engine: explicit precedence over normalized requirement results;
- receipt builder: deterministic machine-readable evaluation record.

The unstable implementation remains under `internal/`; a public Go API is deferred until external consumers and compatibility requirements exist.

## 14. M0 executable scope

M0 establishes:

- Go module and thin CLI shell with help/version behavior;
- closed domain values;
- deterministic completion-decision precedence;
- fail-closed malformed input handling;
- tests for waiver and indeterminate-state semantics;
- governance, threat model, ADRs, schema drafts, examples, and CI.

M0 intentionally does not implement `assurectl verify`, Git subject resolution, policy or contract loaders, evidence loading, receipt construction, GitHub merge authority, or signing. Those begin in M1 and later milestones.

Initial CLI shell behavior:

- help and version return success;
- unknown commands write diagnostics to stderr and return usage exit code `64`;
- no authoritative domain evaluation occurs.

## 15. Testing strategy

Implementation follows test-driven development. The planned portfolio includes:

1. table-driven domain tests for evidence state, observed outcome, verdict, waiver, and decision transitions;
2. policy merge tests proving downstream layers cannot weaken upstream constraints;
3. temporary Git repository tests for clean, dirty, base/head, and stale-revision cases;
4. adversarial fixtures for forged identity, self-rooted authority, path traversal, symlink escape, digest mismatch, duplicate identifiers, expired waivers, and unsupported schemas;
5. deterministic JSON receipt fixtures;
6. repeatability and race tests;
7. parser and normalization fuzz tests;
8. CLI output-channel and exit-code tests;
9. network-independent core evaluator tests.

A feature is not complete because an agent reports success. Its planned checks and resulting diff require fresh verification and review.

## 16. Delivery milestones

| Milestone | Scope |
|---|---|
| M0 | Foundation, domain semantics, decision table, governance, schemas, examples, CI |
| M1 | Exact Git subject resolution, typed local inputs, evidence validation, findings, and unsigned advisory receipt |
| M2 | Trusted CI evidence envelopes, protected workflow guidance, and reference GitHub Action |
| M3 | GitHub App, trusted-base policy resolution, source-pinned required check, and merge authority |
| M4 | Stable in-toto predicate, DSSE/Sigstore signing, replay protection, issuer lifecycle, and conformance tests |
| M5 | Integration with automated API, SDK, framework, dependency, and security migration producers |

## 17. Open-source and managed boundary

Apache-2.0 open-source scope includes the protocol, schemas, domain model, deterministic evaluator, offline verifier, CLI, reference GitHub Action, and conformance fixtures.

Potential managed scope includes the hosted GitHub App, organization policy control plane, managed identity/signing, durable evidence retention, audit search, cross-repository analytics, change intelligence, migration orchestration, enterprise integrations, and support.

Managed decisions must remain independently verifiable using the open protocol and evaluator.

## 18. Claim boundary

AssureCTL can state:

> This identified software-change subject was evaluated under these identified contract and policy inputs using these identified evidence inputs, producing this technical verdict and completion decision.

AssureCTL cannot state solely from that receipt:

- the software is universally correct or secure;
- every relevant test exists;
- the business requirement was complete or wise;
- the producer environment was uncompromised beyond the declared trust model;
- the software is production-ready.

That boundary is part of the protocol, not marketing language that may be relaxed later.