# M1 Evidence Ingestion and Intrinsic Validation Design

**Status:** Approved for implementation planning  
**Date:** 2026-09-29  
**Milestone:** M1 — Local advisory evaluation  
**Base commit:** `85587181f2f18d79c22d8920d823d937cb79634a`  
**Scope:** local evidence discovery, strict envelope ingestion, artifact integrity, and exact-subject facts only

## 1. Purpose

This slice introduces the evidence-ingestion subsystem required by M1 without starting requirement evaluation, producer-trust decisions, policy freshness evaluation, findings, `assurectl verify`, or receipt construction.

The subsystem answers only these questions:

1. Which local evidence-envelope candidates exist?
2. Is each candidate structurally and semantically well-formed?
3. Does its referenced local artifact resolve safely and match the declared SHA-256 digest?
4. Does its repository identity match the resolved software subject?
5. If the repository matches, is the envelope bound to the exact head revision or to another revision?
6. What deterministic digest identifies the validated typed envelope supplied to AssureCTL?

It does **not** decide whether the evidence satisfies a policy requirement.

## 2. Normative alignment

This design implements existing accepted direction rather than introducing a new protocol model.

Relevant invariants:

- ADR-0003: evidence for another head revision is stale relative to the current subject.
- ADR-0007: evidence payloads cannot establish their own authority or trust.
- Foundation INV-006: integrity is distinct from provenance.
- Foundation INV-007: freshness is first-class.
- Protocol v0: timestamp ingestion must perform real RFC3339 parsing and enforce `finished_at >= started_at`.
- Threat model v0: evidence path safety, digest validation, freshness, replay, and provenance are explicit trust-sensitive areas.
- The core evaluator performs no arbitrary network fetch.

No accepted ADR is superseded by this design. If implementation later requires a new trust, protocol, or schema semantic, that change must stop and use an ADR rather than being folded into this slice.

## 3. Scope

### 3.1 In scope

- discover local envelope files directly under `.assurectl/evidence/`;
- deterministic envelope ordering;
- strict JSON decoding using the existing `internal/strictjson` behavior;
- typed representation of `assurectl/evidence-envelope/v0`;
- schema-version and field-level semantic validation already required by the v0 schema/protocol;
- duplicate evidence-ID rejection across the discovered set;
- duplicate canonical artifact-path rejection across the discovered set;
- RFC3339 semantic parsing and invocation time ordering;
- safe local artifact-path validation and resolution;
- local artifact regular-file verification;
- streaming SHA-256 artifact verification;
- canonical typed-envelope SHA-256 digest;
- repository-identity comparison using the same canonicalization semantics as the Git subject resolver;
- explicit subject-binding fact: exact head versus another revision;
- externally assigned local-workspace source metadata marked untrusted/advisory;
- regression tests for security, determinism, and malformed input.

### 3.2 Explicitly out of scope

- requirement matching;
- deriving `MISSING`;
- final `domain.EvidenceState`;
- producer allowlists or producer trust;
- CI/OIDC provenance;
- `max_age_seconds` evaluation;
- wall-clock freshness evaluation;
- replay protection;
- finding generation;
- verification verdicts;
- completion decisions;
- waiver handling;
- `assurectl verify`;
- receipt construction or receipt ingestion;
- new JSON schemas or schema changes;
- YAML;
- network-backed artifact URIs;
- new third-party dependencies;
- arbitrary artifact-content interpretation based on media type.

### 3.3 Composition seam

The evidence subsystem receives an already resolved software subject; it does not execute Git or resolve refs itself.

The intended package seam is:

```go
func LoadLocal(root string, subject domain.Subject) ([]Loaded, error)
```

The exact return container may be refined during implementation if a smaller zero-value-safe type is required, but the dependency direction is fixed:

```text
gitsubject.Resolve(...)
        ↓
   domain.Subject
        ↓
evidence.LoadLocal(...)
```

The evidence package must not depend on `gitsubject.Resolution`, invoke Git, re-resolve base/head refs, or infer a subject from envelope contents. It uses only the canonical repository URI and exact head revision already present in the resolved `domain.Subject`.

## 4. Local layout and discovery

The local M1 convention is:

```text
.assurectl/
└── evidence/
    ├── unit-tests.json
    ├── security-scan.json
    └── artifacts/
        ├── unit-tests.json
        └── security-scan.json
```

The logical evidence root itself must resolve inside the supplied workspace root. A symlinked `.assurectl/evidence/` directory that escapes the workspace is rejected.

Only regular `*.json` files directly inside `.assurectl/evidence/` are envelope candidates.

Discovery is non-recursive. The `artifacts/` subtree is never scanned for envelopes.

Candidate paths are processed in deterministic canonical filename order.

If `.assurectl/evidence/` does not exist, ingestion succeeds with an empty evidence set. Absence becomes `MISSING` only later, when a policy requirement is evaluated.

No candidate envelope may be silently ignored because it is malformed, unreadable, or inconvenient to validate.

## 5. Typed evidence model

The new internal package is `internal/evidence`.

Its typed model mirrors the existing v0 evidence-envelope schema:

- `Envelope`;
- `Subject`;
- `Producer`;
- `Invocation`;
- `Outcome`;
- `Artifact`;
- `Loaded`;
- an internal subject-binding fact type.

The package must not introduce:

- `RequirementResult`;
- `Finding`;
- policy requirement types;
- a replacement for `domain.EvidenceState`;
- receipt types.

Observed outcome remains a separate fact and preserves the protocol values `PASSED`, `FAILED`, and `ERROR`.

No additional relationship between outcome and exit code is invented in this slice.

## 6. Strict envelope ingestion

Each candidate is decoded with the existing strict JSON rules:

- valid UTF-8;
- exactly one JSON value;
- duplicate object keys rejected recursively;
- unknown fields rejected;
- exact JSON field names required rather than case-insensitive aliases.

The typed validator additionally enforces the current v0 schema semantics, including:

- `schema_version == assurectl/evidence-envelope/v0`;
- canonical evidence-ID grammar;
- bounded string fields;
- 40- or 64-character lowercase hexadecimal Git object IDs where required;
- `sha256` digest algorithm;
- 64-character lowercase SHA-256 values;
- closed outcome values;
- required nested fields.

The implementation must not narrow the public schema by adding assumptions that are not already required by the accepted protocol/schema.

## 7. Timestamp semantics and determinism

The supplied timestamp strings are preserved as supplied typed input for envelope digesting.

Ingestion also parses `started_at` and `finished_at` with an RFC3339 parser and rejects impossible calendar values.

The required cross-field rule is:

```text
finished_at >= started_at
```

Equal timestamps are valid.

The loader must not call `time.Now()` to decide freshness.

Policy-relative freshness is deferred because it requires at least:

- explicit evaluation time;
- policy `max_age_seconds`;
- the loaded evidence invocation time.

Therefore identical filesystem/input state produces identical ingestion results regardless of the wall clock.

## 8. Subject binding

The resolved M1 Git subject already provides:

- canonical repository URI;
- base revision;
- head revision;
- change-set algorithm;
- change-set digest.

Evidence envelopes provide:

- repository URI;
- one revision.

Repository comparison must use the same canonical identity behavior as the Git subject resolver. That algorithm must not be copied into a second implementation.

The comparison seam must recognize both resolver identity forms:

- canonical remote repository identities produced from supported HTTPS/SSH/SCP inputs;
- the resolver-generated advisory fallback `local://sha256/<64 lowercase hex>` identity when no usable origin exists.

A narrow reusable seam may be exposed from `internal/gitsubject` for canonical repository **identity** handling. This must not broaden the resolver's explicit/origin URI acceptance rules: a caller still cannot choose a self-invented `local://` value as an explicit remote URI. It must not grow into a new identity framework or strategy hierarchy.

Subject outcomes are facts, not final evidence states:

### 8.1 Repository mismatch

If the evidence repository canonicalizes successfully but does not equal the resolved subject repository, ingestion fails closed.

Evidence for a different repository is not treated as merely old evidence for the current subject.

### 8.2 Other revision

If repository identity matches but:

```text
evidence.subject.revision != resolved.subject.head_revision
```

the envelope remains successfully ingested with an explicit "other revision" subject-binding fact.

This preserves the distinction needed later to derive `STALE` without pretending that ingestion alone has completed policy evaluation.

### 8.3 Exact revision

If repository identity and head revision both match, the subject-binding fact is exact.

Exact subject binding still does **not** mean final `EvidenceValid`; producer trust and policy freshness remain unresolved.

## 9. Artifact URI model

For local M1, `artifact.uri` is interpreted only as a portable slash-separated relative path rooted at `.assurectl/evidence/`.

A normal accepted example is:

```text
artifacts/unit-tests.json
```

The local loader rejects at least:

- absolute paths;
- `.` and `..` segments;
- traversal;
- repeated empty segments;
- backslashes;
- Windows drive syntax;
- UNC paths;
- `file:`, `http:`, `https:`, and other URI schemes;
- NUL/control-character ambiguity;
- unsafe Windows device/path aliases;
- trailing-space/trailing-dot aliases that are not portable.

Accepted artifact URIs are already canonical path spellings because unsafe normalization aliases are rejected rather than rewritten. Reusing the same canonical `artifact.uri` in more than one discovered evidence envelope is rejected set-wide as a duplicate normalized artifact path.

This slice does not implement a general URI resolver.

## 10. Artifact resolution and integrity

A referenced artifact must:

1. resolve from the evidence root;
2. remain within the evidence root after symlink resolution;
3. be a regular file;
4. be readable;
5. hash to exactly the declared SHA-256 digest.

Artifact hashing is streaming, conceptually:

```go
h := sha256.New()
io.Copy(h, file)
```

The artifact body is not loaded into memory solely to compute the digest.

No arbitrary artifact-size limit is introduced by this design. If resource limits become necessary, they require an explicit resource-policy decision rather than an undocumented protocol narrowing.

The declared `media_type` remains validated metadata. This slice does not parse or reinterpret artifact bytes based on that field.

## 11. Envelope digest

Envelope digest and artifact digest have different meanings.

### Artifact digest

```text
SHA256(actual artifact bytes)
```

### Envelope digest

```text
strictly decoded typed Envelope
        ↓
json.Marshal(typed envelope)
        ↓
SHA-256
```

This makes the envelope digest insensitive to JSON whitespace and object-key ordering while still identifying the typed input fields supplied to the evaluator.

The loader does not silently rewrite semantically equivalent timestamp representations or repository URI strings merely to make their digests equal. Parsed/canonical comparison values are validation facts; the typed supplied values remain the input being digested.

This normalized typed-envelope digest is retained so a later receipt-construction design can decide how evidence references bind to loaded envelopes. This slice does not establish new receipt semantics.

## 12. Trust metadata

A local workspace envelope cannot promote itself to trusted status.

Local evidence is externally annotated by the loader/integration as an advisory workspace input, consistent with ADR-0007.

The envelope schema contains no accepted self-authority field.

This slice records local source/trust metadata but does not perform producer-trust evaluation.

## 13. Failure model

### 13.1 Empty set

Missing `.assurectl/evidence/`:

```text
success → empty []Loaded
```

This is not `MISSING` yet.

### 13.2 Fatal ingestion error

The local set fails closed if an envelope candidate cannot be safely and deterministically ingested, including:

- malformed or ambiguous JSON;
- unsupported schema version;
- invalid typed fields;
- impossible or reversed timestamps;
- duplicate evidence IDs;
- duplicate canonical artifact paths;
- unsafe artifact URI;
- missing artifact;
- non-regular artifact;
- symlink escape;
- artifact digest mismatch;
- malformed repository identity;
- repository mismatch.

No partially accepted local evidence set is returned as though complete.

### 13.3 Successful non-exact subject fact

Same repository plus another revision is not a fatal ingestion error.

It is returned as successfully loaded evidence with a non-exact revision-binding fact so the later evaluation layer can derive stale evidence under the effective policy and evaluation context.

## 14. Package and file boundary

The intended implementation remains small:

```text
internal/evidence/
├── model.go
├── loader.go
├── artifact.go
├── subject.go
├── loader_test.go
├── artifact_test.go
├── subject_test.go
├── security_regression_test.go
└── determinism_test.go
```

Exact file count may shrink if a smaller implementation is clearer; the responsibilities must not expand.

Supporting changes are limited to narrow reuse seams:

- `internal/gitsubject`: expose existing repository-URI canonicalization without changing semantics;
- `internal/localinput`: optionally expose a safe regular-file opening primitive so containment/symlink logic is not duplicated, while preserving the existing 1 MiB behavior of configuration reads.

The following surfaces remain untouched unless a concrete implementation contradiction is found and brought back through design review:

- `cmd/assurectl/*`;
- `internal/decision/*`;
- `internal/domain/evidence.go`;
- public JSON schemas;
- receipt construction;
- policy/contract evaluation;
- waiver logic.

## 15. TDD acceptance matrix

Implementation must proceed RED → GREEN.

| Area | Required regression |
|---|---|
| Discovery | deterministic order, non-recursive discovery |
| Empty input | missing evidence directory returns empty set |
| JSON | unknown fields, duplicate keys, case aliases fail |
| Schema | unsupported schema version fails |
| Identity | malformed and duplicate evidence IDs fail |
| Timestamp | impossible RFC3339 date and reversed interval fail |
| Outcome | PASSED/FAILED/ERROR preserved independently |
| Artifact path | absolute/traversal/backslash/URI aliases and duplicate canonical artifact paths fail |
| Symlink | artifact/root escape fails |
| Artifact | missing/non-regular/digest mismatch fail |
| Repository | canonical repository mismatch fails |
| Revision | exact and explicit other-revision facts are distinguished |
| Envelope digest | typed canonical digest is deterministic |
| Trust | local workspace source remains externally untrusted/advisory |
| Repeatability | identical filesystem/input yields identical ordered results |

Tests in this slice must not pull requirement matching, producer trust, freshness age, findings, verdict, decision, or receipts into scope.

## 16. Security and review gates

Before this slice is considered complete:

- every behavior change has an observed RED test before production implementation;
- path and symlink regressions receive explicit adversarial tests;
- duplicate-ID and JSON ambiguity cases fail closed;
- no network access is introduced;
- no third-party dependency is introduced;
- exact-head CI passes the repository's required formatting, vet, race-test, and CLI-build checks;
- the final diff is reviewed against this scope document;
- any discovered need to change public protocol semantics stops implementation and returns to design/ADR review.

This slice does not claim race-free protection against a malicious concurrent filesystem mutation between path resolution, stat, open, and hashing. The current threat model already lists time-of-check/time-of-use protections as deferred work. The implementation must preserve existing fail-closed path/symlink checks without overstating that deferred guarantee.

## 17. Follow-on boundary

Only after this slice is merged should M1 proceed to a separate evaluation slice that combines:

```text
LoadedEvidence[]
+ policy requirements
+ externally resolved producer trust
+ explicit evaluation time
        ↓
normalized requirement evidence state / outcome
```

That later slice is where `MISSING`, `STALE`, `UNTRUSTED`, and final `VALID`/policy-relative evidence-state derivation belong.

Findings, advisory verify composition, and unsigned advisory receipt remain later steps.

## 18. Success criterion

This design is successful when AssureCTL can deterministically and fail-closed ingest local v0 evidence envelopes and their local artifacts, verify intrinsic integrity, and establish exact-versus-other-revision subject facts **without claiming that the evidence satisfies any requirement or is trusted**.
