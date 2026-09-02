# AssureCTL Foundation Design

**Status:** Proposed for implementation planning  
**Date:** 2026-09-02  
**Repository:** `github.com/kefyusuf/assurectl`  
**License direction:** Apache-2.0

## 1. Executive summary

AssureCTL is an open, deterministic, revision-bound assurance authority for software changes. It evaluates whether a specific software change is permitted to move to a completion state under an explicit verification contract, trusted policy, and machine-verifiable evidence.

AssureCTL is not a test runner, CI platform, coding agent, AI reviewer, or guarantee that code is bug-free. Evidence producers execute tests and checks. AssureCTL validates the provenance, subject binding, freshness, completeness, and outcomes of that evidence; evaluates a versioned policy; preserves findings and waivers without rewriting technical truth; and emits a machine-readable completion decision.

The first product surface is a local Go CLI. It produces advisory decisions for exact Git revisions. Later milestones add trusted CI evidence, a GitHub App that publishes a source-pinned required check, and in-toto/DSSE/Sigstore-aligned signed attestations. The long-term integration point is self-maintaining software: API, SDK, framework, dependency, security, and other automated migrations generate changes, while AssureCTL determines what the evidence permits.

## 2. Problem statement

Coding agents can modify repositories, run commands, write status files, and claim that work is complete. None of those actions independently establish that:

- the declared work was the work actually evaluated;
- the evaluated source state is the source state proposed for merge or release;
- required checks were executed by a trusted producer;
- evidence is current, complete, and untampered;
- the effective policy was not weakened by the same change under evaluation;
- a waiver was issued by an authorized risk owner;
- the final completion decision came from an authority independent of the worker.

Conventional CI answers whether configured jobs ran and what they reported. Supply-chain attestations primarily answer where an artifact came from and how it was built. Agent summaries answer what the agent says it did. The missing layer is a software-change-specific assurance authority that binds intent, policy, evidence, source revision, and completion semantics.

## 3. Product positioning

**Category:** software change assurance  
**Positioning:** open, revision-bound assurance authority for software changes  
**Tagline:** Agents propose changes. AssureCTL decides what the evidence permits.

The initial market is software delivery, not generic consequential agent workflows. The protocol may remain extensible, but v0 work units are limited to:

- pull requests;
- commits;
- release candidates;
- API and SDK migrations;
- framework and dependency upgrades;
- security remediations.

## 4. Goals

The first stable architecture must:

1. Bind every evaluation to an immutable repository subject and exact source revisions.
2. Keep evidence state, observed outcome, verification verdict, waiver, finding, and completion decision semantically separate.
3. Resolve effective policy from trusted sources and prevent a change from weakening the policy that evaluates itself.
4. Treat agent-authored assertions and workspace files as untrusted unless independently attested by an allowed producer.
5. Evaluate a declarative, versioned contract deterministically and fail closed when authority cannot be established.
6. Produce machine-readable receipts suitable for later in-toto/DSSE wrapping without inventing custom cryptography.
7. Remain independent of programming language, framework, coding agent, and CI provider.
8. Provide a narrow local vertical slice before adding hosted services, orchestration, or a web interface.

## 5. Non-goals

AssureCTL v0 will not:

- prove that software is correct, secure, bug-free, or production-ready;
- decide whether acceptance criteria themselves are sufficient or desirable;
- execute arbitrary build and test commands as a general CI runner;
- provide an agent orchestrator or multiplayer agent session system;
- use an LLM as the authoritative verdict source;
- provide a SaaS dashboard, billing, organization management, or long-term evidence storage;
- perform API/SDK/framework migration generation;
- introduce a custom signing algorithm, private PKI, or home-grown canonical-signature format;
- support arbitrary policy code, shell expressions, embedded scripts, or a general-purpose policy language in v0.

## 6. Core invariants

### INV-001 — Independent authority

The worker that proposes or produces a change must not be the authority that marks the change complete.

### INV-002 — Immutable subject binding

An authoritative decision must identify the repository, base revision, head revision, and normalized change-set digest it evaluated.

### INV-003 — Trusted intent

An authoritative decision requires a verification contract whose authority and digest are independently established. An agent may suggest acceptance criteria but may not silently replace the trusted contract it is being evaluated against.

### INV-004 — Trusted policy

A pull request or candidate head revision may not weaken the policy used to evaluate itself. Authoritative evaluation resolves project policy from a trusted base revision and, later, organization policy from a protected control plane.

### INV-005 — Evidence is not prose

An agent summary, status field, transcript, or self-authored `passed` file is not merge-grade evidence.

### INV-006 — Integrity is not provenance

A matching file hash proves byte integrity only. Trusted evidence also requires permitted producer identity, execution context, subject revision, invocation identity, timing, and outcome.

### INV-007 — Freshness is first-class

Evidence for a different subject revision or an expired validity window is stale. Stale evidence cannot be upgraded to valid by a waiver.

### INV-008 — Waivers preserve technical truth

A waiver is explicit risk acceptance. It never transforms failed, missing, stale, invalid, or untrusted evidence into passing evidence.

### INV-009 — Protocol invariants are non-waivable

Revision binding, trusted authority, policy integrity, receipt integrity, and required producer identity cannot be waived.

### INV-010 — Deterministic evaluation

For the same normalized subject, contract, effective policy, evidence envelopes, waiver set, and evaluator version, the evaluator must produce the same findings, verification verdict, and completion decision.

### INV-011 — Fail closed

Malformed inputs, unsupported schema versions, unresolved policy conflicts, unknown authority, ambiguous revisions, and internal evaluation errors cannot result in approval.

### INV-012 — Claim discipline

A completion receipt states only that the identified subject was evaluated under the identified contract and policy using the identified evidence. It does not assert universal correctness or safety.

## 7. Domain vocabulary

| Concept | Definition |
|---|---|
| `WorkUnit` | The declared unit of software change whose completion is being evaluated. |
| `Subject` | The immutable repository and revision identity evaluated by the authority. |
| `AcceptanceCriterion` | A required observable outcome associated with the work unit. |
| `VerificationRequirement` | The evidence type, trust, freshness, subject, and outcome conditions required to evaluate a criterion. |
| `VerificationContract` | The trusted, digest-bound set of acceptance criteria and verification requirements for a work unit. |
| `EvidenceEnvelope` | A machine-readable observation with subject, producer, invocation, outcome, artifact, and provenance metadata. |
| `Finding` | A normalized fact discovered during validation or evaluation. |
| `Waiver` | An authorized, scoped, expiring risk acceptance for an explicitly waivable requirement or finding. |
| `VerificationVerdict` | The aggregate technical result derived from requirements and evidence, without applying risk acceptance. |
| `CompletionDecision` | The lifecycle decision produced after policy and valid waivers are applied without changing the verification verdict. |
| `Receipt` | The machine-readable record of inputs, findings, verdict, waivers, decision, and verifier identity. |

A `WorkUnit` is not synonymous with a pull request. A pull request is one integration surface. The same domain model must support commits, releases, and migrations without changing the core semantics.

## 8. Subject and revision model

The minimum subject is:

```yaml
subject:
  repository_uri: github.com/acme/checkout
  base_revision: 2a5165d0f3f9...
  head_revision: 948ab31d0c42...
  change_set_digest: sha256:7f920d...
```

Rules:

- `repository_uri` is normalized to a provider-neutral repository identity.
- `base_revision` and `head_revision` are immutable full commit identifiers.
- `change_set_digest` is computed from a normalized representation of the base-to-head change set, not from a human-readable PR number.
- The receipt subject includes both revisions even when an adapter also records a PR or release identifier.
- Authoritative evaluation requires a clean, committed source state.
- A dirty workspace may be evaluated only as `ADVISORY`; it cannot produce `APPROVED` authority.
- New commits invalidate evidence whose subject does not match the new head revision.

The exact normalized change-set algorithm will be specified and conformance-tested before signed receipts become stable. No v0 security claim may depend on an undocumented Git diff representation.

## 9. Contract authority

A verification contract contains:

```yaml
schema_version: assurectl.dev/verification-contract/v0
work_unit:
  id: migration-184
  type: api_migration
  objective: Migrate checkout integration to the supported API contract.
authority:
  type: local-user
  identity: advisory
acceptance_criteria:
  - id: AC-001
    statement: Existing payment behavior remains covered by the required test suite.
    requirements:
      - unit-tests
      - integration-tests
```

Contract trust is separate from evidence trust. In local v0, a workspace contract is advisory. In a future authoritative GitHub flow, the contract must be pinned to an approved issue, protected metadata source, signed artifact, or other policy-approved authority. If the contract authority is not trusted, the engine may report findings but must return a blocked authoritative decision.

## 10. Evidence model

An evidence envelope has six concerns:

```yaml
schema_version: assurectl.dev/evidence-envelope/v0
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
  environment_digest: sha256:31fd...
  started_at: 2026-09-02T09:10:00Z
  finished_at: 2026-09-02T09:12:14Z
outcome:
  status: PASSED
  exit_code: 0
artifact:
  uri: artifacts/unit-tests.json
  digest: sha256:8aa3...
  media_type: application/json
```

### 10.1 Evidence state

Evidence validation produces one state:

- `VALID`: schema, integrity, subject, producer, invocation, and freshness requirements are satisfied;
- `INVALID`: evidence is malformed, internally inconsistent, or fails an integrity check;
- `MISSING`: required evidence is absent;
- `STALE`: evidence is otherwise usable but belongs to another revision or expired validity window;
- `UNTRUSTED`: integrity may be intact, but producer or execution provenance is not permitted by policy.

Evidence state is distinct from observed outcome. A trusted test report can be `VALID` evidence whose outcome is `FAILED`.

### 10.2 Evidence path safety

Evidence artifact resolution must reject absolute paths, traversal segments, escaping symlinks, duplicate normalized paths, unsafe platform aliases, and unsupported URI schemes. Core verification must not fetch arbitrary network resources in v0.

## 11. Policy model and precedence

Policy is declarative YAML validated by a versioned JSON Schema. The v0 evaluator supports a finite typed rule set; it does not execute policy-supplied code.

Effective policy layers are ordered as follows:

1. protocol baseline — built into the evaluator and non-waivable;
2. organization policy — protected external control plane in a later milestone;
3. project policy — loaded from the trusted base revision for authoritative evaluation;
4. assurance profile — referenced by trusted policy;
5. work-unit contract — may add requirements but may not weaken upstream constraints.

Deterministic merge rules:

- required verification requirements accumulate by set union;
- minimum producer trust may only increase;
- maximum evidence age may only decrease;
- a requirement may become non-waivable downstream but not become waivable;
- permitted producer identities may only narrow unless an upstream policy explicitly delegates extension;
- unknown or non-orderable conflicts produce a policy-resolution finding and a blocked decision.

Local advisory mode may load `.assurectl/policy.yaml` from the workspace, but the receipt must identify it as an untrusted workspace policy. Authoritative mode must record the trusted source revision and policy digest.

## 12. Findings, verdicts, waivers, and decisions

### 12.1 Findings

A finding contains a stable code, category, severity, affected requirement, relevant evidence identifiers, human-readable explanation, and deterministic disposition. Machine consumers must rely on stable codes rather than message text.

Initial finding categories include:

- subject mismatch;
- missing evidence;
- stale evidence;
- invalid evidence;
- untrusted producer;
- failed observed outcome;
- policy resolution failure;
- contract authority failure;
- invalid or expired waiver;
- unsupported schema;
- internal evaluation failure.

### 12.2 Verification verdict

The aggregate `VerificationVerdict` is computed before waivers:

- `FAILED` when at least one required verification has a definitive failed outcome;
- otherwise `INDETERMINATE` when at least one required verification cannot be established because evidence is missing, stale, invalid, untrusted, or authority is unresolved;
- otherwise `PASSED`.

A definitive failure takes precedence over indeterminate requirements so the receipt does not conceal known failure. All findings remain present.

### 12.3 Waivers

A waiver must identify:

- its stable identifier;
- the exact requirement or finding it covers;
- repository and work-unit scope;
- optional exact revision scope;
- reason and accepted risk;
- approver identity and authority;
- issuance and expiry times;
- integrity or signature metadata when authoritative waivers are introduced.

A waiver is applicable only when policy marks the target requirement waivable and the waiver is valid for the exact scope. Protocol invariants cannot be waiver targets.

### 12.4 Completion decision

The `CompletionDecision` is:

- `APPROVED`: verification passed and no blocking authority or policy finding exists;
- `REJECTED`: definitive failed requirements remain and cannot all be covered by valid, authorized waivers;
- `BLOCKED`: verification is indeterminate, authority is unresolved, policy or contract is invalid, or a protocol invariant cannot be established;
- `ACCEPTED_WITH_RISK`: verification failed only on explicitly waivable requirements, every such failure is covered by a valid authorized waiver, and all non-waivable requirements passed.

`ACCEPTED_WITH_RISK` does not change `VerificationVerdict: FAILED`.

## 13. Trust tiers

### Tier 0 — Local advisory

`assurectl verify` evaluates local inputs and produces an unsigned advisory receipt.

- authority: `ADVISORY`;
- workspace policy and contract are permitted but identified as untrusted sources;
- dirty workspaces may produce findings but never authoritative approval;
- suitable for developer and agent feedback;
- not merge-grade.

### Tier 1 — Trusted CI assertion

A CI adapter supplies producer identity and execution provenance.

- authority: `CI_ASSERTED`;
- merge suitability depends on protected workflow and policy configuration;
- receipt may be stored as a build artifact;
- not automatically equivalent to independent authority.

### Tier 2 — Independent repository authority

A GitHub App resolves trusted policy and contract, validates evidence, and publishes a required check whose expected source is pinned to that App.

- authority: `AUTHORITATIVE`;
- intended merge-grade integration;
- signing and durable storage are later managed capabilities.

## 14. Core architecture

```text
CLI / adapter
     |
     v
Subject Resolver ---- Contract Loader
     |                      |
     +----------+-----------+
                v
          Policy Resolver
                |
                v
       Evidence Loader
                |
                v
       Evidence Validator
                |
                v
      Verification Engine
                |
                v
          Waiver Engine
                |
                v
     Completion Decision Engine
                |
                v
          Receipt Builder
```

### 14.1 Components

- **CLI:** argument parsing, discovery, human and JSON output, exit-code mapping; no domain decisions.
- **Subject resolver:** discovers repository identity and exact Git state; computes or delegates normalized change-set identity.
- **Contract loader:** parses, schema-validates, normalizes, and records contract authority.
- **Policy resolver:** combines policy layers under monotonic restriction rules and emits the effective policy digest.
- **Evidence loader:** discovers local evidence envelopes and referenced artifacts without interpreting pass/fail semantics.
- **Evidence validator:** validates schema, paths, hashes, subject binding, producer trust, invocation metadata, and freshness.
- **Verification engine:** maps valid evidence and observations to requirements and produces findings plus technical verdict.
- **Waiver engine:** validates scope, authority, expiry, and policy permission; never mutates evidence or verdict.
- **Completion decision engine:** maps verdict, findings, and applicable waivers to the lifecycle decision.
- **Receipt builder:** emits a versioned record with normalized input digests, findings, verdict, waivers, decision, and verifier metadata.

Each component exposes typed inputs and outputs and must be testable without invoking the CLI.

## 15. CLI v0 vertical slice

The first executable behavior is:

```bash
assurectl verify
```

Default discovery:

```text
.assurectl/policy.yaml
.assurectl/work-unit.yaml
.assurectl/evidence/*.json
.assurectl/receipts/
```

The command also accepts explicit paths and `--format text|json`. It discovers the current Git repository, resolves a committed subject, validates the inputs, evaluates the decision, prints the result, and optionally writes an unsigned receipt.

Initial exit codes:

| Exit code | Meaning |
|---:|---|
| `0` | `APPROVED` |
| `1` | `REJECTED` |
| `2` | `BLOCKED` |
| `3` | `ACCEPTED_WITH_RISK` |
| `64` | CLI usage or configuration error before evaluation |
| `70` | unexpected internal error; fail closed |

Local approval remains advisory even when the completion decision field is `APPROVED`; authority is represented separately.

## 16. Receipt and attestation direction

The v0 receipt is unsigned JSON and makes no cryptographic authority claim. It includes:

- schema version;
- subject and work-unit identity;
- contract source and digest;
- effective policy sources and digest;
- normalized evidence references and digests;
- findings;
- verification verdict;
- applied and rejected waivers;
- completion decision;
- authority tier;
- evaluator name, semantic version, and protocol version;
- evaluation timestamp.

Signed receipts will use an in-toto Statement with an AssureCTL predicate, wrapped in DSSE and signed through a standard identity mechanism such as Sigstore/OIDC. The project will not define a custom signature primitive. Signature introduction requires a separate stable protocol specification and conformance fixtures.

## 17. Error handling

Expected domain failures are findings and decisions, not process crashes. Examples include missing evidence, stale evidence, unknown producer, and failed outcomes.

Configuration and usage errors stop before evaluation with exit code `64`. Unexpected internal failures return exit code `70`, emit no approval, and avoid writing a misleading receipt. JSON output sends machine data to stdout and diagnostics to stderr. Sensitive evidence contents are not included in receipts by default; receipts contain bounded metadata and digests.

## 18. Repository structure

After design approval, the foundation scaffold will use:

```text
.
├── cmd/assurectl/
├── internal/
│   ├── domain/
│   ├── subject/
│   ├── contract/
│   ├── policy/
│   ├── evidence/
│   ├── verification/
│   ├── waiver/
│   ├── decision/
│   └── receipt/
├── schemas/
├── spec/
├── examples/
├── docs/
│   ├── adr/
│   ├── architecture/
│   ├── protocol/
│   ├── threat-model/
│   └── superpowers/specs/
├── .github/workflows/
├── AGENTS.md
├── CONTRIBUTING.md
├── SECURITY.md
├── LICENSE
├── README.md
└── go.mod
```

The repository will avoid a public `pkg/` API until stable external consumers exist. Internal packages will enforce boundaries while the protocol evolves.

## 19. Testing strategy

Implementation follows test-driven development. The minimum test portfolio is:

1. table-driven domain tests for every evidence state, verdict, waiver, and decision transition;
2. policy merge tests proving downstream layers cannot weaken upstream constraints;
3. temporary-Git-repository integration tests for clean, dirty, base/head, and stale-revision scenarios;
4. adversarial fixtures for forged producer identity, self-rooted authority, path traversal, symlink escape, digest mismatch, duplicate identifiers, expired waiver, and unsupported schema;
5. golden JSON receipt tests with stable field ordering and normalized identifiers;
6. repeatability tests proving identical normalized inputs produce identical findings, verdict, and decision;
7. fuzz tests for parsers, path normalization, identifiers, and digest handling;
8. CLI integration tests for output channels and exit codes;
9. tests that require no network access for the core evaluator.

A feature is not complete because an agent reports success. Completion requires the planned test evidence and a review of the resulting diff.

## 20. Delivery milestones

### M0 — Foundation

Create project documents, ADRs, Go module, package skeleton, schema placeholders, examples, and minimal CI. No authoritative security claims.

### M1 — Local advisory vertical slice

Implement exact Git subject resolution, typed policy and contract parsing, local evidence validation, deterministic verdict/decision, and unsigned receipt output.

### M2 — Trusted CI evidence

Define CI producer envelopes, protected workflow guidance, reusable GitHub Action integration, and CI-asserted receipts.

### M3 — GitHub merge authority

Implement a GitHub App that resolves trusted-base policy and contract authority, validates evidence, and publishes a source-pinned required check.

### M4 — Signed attestations

Define the stable in-toto predicate, DSSE/Sigstore signing, trust-root policy, replay protection, issuer rotation, and conformance tests.

### M5 — Change-intelligence integration

Integrate automated API/SDK/framework/dependency migration producers while preserving the independent assurance boundary.

## 21. Open-source and commercial boundary

Apache-2.0 open-source scope:

- protocol and schemas;
- domain model;
- deterministic evaluator;
- offline verifier;
- CLI;
- reference GitHub Action;
- conformance fixtures.

Potential managed scope:

- hosted GitHub App;
- organization policy control plane;
- managed identity and signing;
- durable evidence retention and audit search;
- cross-repository analytics;
- change intelligence and migration orchestration;
- enterprise integrations and support.

The open evaluator must remain capable of independently validating the rules and receipts used by managed services.

## 22. Governance and change control

- Architectural decisions are recorded as ADRs and are superseded rather than silently rewritten.
- Protocol and schema identifiers are versioned independently from the CLI.
- Breaking protocol changes require migration notes and conformance fixtures.
- Security-sensitive behavior is documented in the threat model before being advertised as supported.
- After the initial design commit, implementation changes use topic branches and pull requests.
- Repository policy changes are evaluated under the previously trusted policy once authoritative enforcement exists.

## 23. Design acceptance criteria

This design is ready for implementation planning when the project owner confirms that it correctly captures:

- software-change-only v0 scope;
- open deterministic evaluation kernel;
- exact revision and change-set binding;
- trusted policy and contract authority;
- evidence provenance and freshness;
- separate technical verdict, waiver, and completion decision semantics;
- local advisory first, GitHub authority later;
- no custom cryptography and no correctness guarantee.

Implementation planning must not expand v0 into a CI platform, agent orchestrator, migration generator, SaaS dashboard, or generic agent-safety framework.