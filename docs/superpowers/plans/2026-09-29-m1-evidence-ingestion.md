# M1 Evidence Ingestion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deterministically ingest local v0 evidence envelopes, verify intrinsic envelope/artifact integrity, and record exact-versus-other-revision subject facts without performing policy evaluation or assigning final evidence state.

**Architecture:** Add one `internal/evidence` package that consumes an already resolved `domain.Subject`. Reuse the existing strict JSON decoder, repository-URI canonicalization semantics, and workspace containment behavior; keep discovery, envelope validation, subject binding, and artifact verification as separate tested responsibilities, then compose them in `LoadLocal`.

**Tech Stack:** Go standard library only; existing `internal/strictjson`, `internal/inputmeta`, `internal/localinput`, `internal/gitsubject`, and `internal/domain`.

**Spec:** `docs/superpowers/specs/2026-09-29-m1-evidence-ingestion-design.md`

## Global Constraints

- M1 local advisory scope only.
- No new third-party dependency.
- No YAML.
- No network-backed artifact resolution or arbitrary network fetch.
- No `assurectl verify`, CLI behavior, receipt construction, findings, verdict, decision, waiver, requirement matching, or policy merge work.
- No final `domain.EvidenceState` assignment in this slice.
- No producer-trust or CI/OIDC provenance decision in this slice.
- No `time.Now()` freshness decision; wall-clock freshness remains a later explicit evaluation input.
- No public schema change.
- Evidence payloads cannot establish their own trust; local workspace evidence remains externally `UNTRUSTED` / `ADVISORY_WORKSPACE`.
- Repository mismatch fails ingestion; same repository plus another revision remains a successful non-exact subject fact.
- Artifact access is local, relative, workspace/evidence-root bounded, regular-file only, and SHA-256 verified.
- Existing contract/policy configuration reads retain their 1 MiB bound.
- Do not claim race-free TOCTOU protection; preserve current fail-closed path/symlink checks and the threat-model boundary.
- Every behavior change follows observed RED → minimal GREEN → focused review → commit.

## File Structure

### Create

- `internal/evidence/model.go` — typed v0 envelope, loaded result, and subject-binding fact.
- `internal/evidence/loader.go` — strict envelope decoding/validation, local discovery, set assembly, duplicate-ID rejection, and canonical envelope digest.
- `internal/evidence/subject.go` — comparison against an already resolved `domain.Subject`.
- `internal/evidence/artifact.go` — portable artifact-path validation, safe local open, streaming SHA-256 verification.
- `internal/evidence/loader_test.go` — schema/JSON/timestamp/numeric/discovery/set behavior.
- `internal/evidence/subject_test.go` — exact/other-revision/repository mismatch and malformed caller subject.
- `internal/evidence/artifact_test.go` — path aliases, regular-file requirements, digest validation.
- `internal/evidence/security_regression_test.go` — symlink escapes, self-asserted trust, fail-closed set behavior.
- `internal/evidence/determinism_test.go` — stable ordering and canonical typed-envelope digest.

### Modify

- `internal/gitsubject/identity.go` — expose one narrow canonical **repository identity** seam that preserves existing remote normalization and validates the resolver-generated `local://sha256/...` advisory identity without broadening explicit URI acceptance.
- `internal/gitsubject/identity_test.go` — pin exported seam to existing canonicalization behavior.
- `internal/localinput/read.go` — extract/open a reusable workspace-bounded regular-file primitive; keep `ReadWorkspaceFile` behavior and 1 MiB limit unchanged.
- Create `internal/localinput/read_test.go` if needed for the new primitive; do not move unrelated tests.
- `docs/superpowers/specs/2026-09-29-m1-evidence-ingestion-design.md` — status only after verified implementation if appropriate; no semantic rewrite during implementation.

## Review Focus

1. **Optional JSON fields explicitly present but empty/null** — absent `producer.workflow`, `producer.workflow_revision`, `invocation.environment_digest`, and `outcome.exit_code` are allowed where schema permits; explicitly invalid values must not collapse into “absent”. Task 2 pins this with presence-sensitive decoding tests.
2. **JSON Schema integer semantics for `outcome.exit_code`** — values such as `1e3`, `1000.0`, negative integers, and integers beyond int64 must behave as mathematical JSON integers rather than Go `int64` syntax. Task 2 pins this.
3. **Malformed or non-canonical caller `domain.Subject`** — `LoadLocal` must fail closed rather than trusting a manually constructed invalid subject merely because normal composition uses `gitsubject.Resolve`. Task 4 pins this.
4. **Filesystem aliases and cross-platform artifact names** — traversal, backslash, colon/scheme, Windows device aliases, and trailing-dot/space segments must not acquire platform-dependent meanings. Task 5 pins this.
5. **Evidence-root / artifact symlink escapes and partial-set acceptance** — an escaping root/artifact or one invalid member must fail the complete local ingestion call; valid siblings must not be returned as a misleading complete set. Tasks 3, 5, and 6 pin this.

---

### Task 1: Add Minimal Reuse Seams Without Changing Existing Semantics

**Files:**
- Modify: `internal/gitsubject/identity.go`
- Modify: `internal/gitsubject/identity_test.go`
- Modify: `internal/localinput/read.go`
- Create: `internal/localinput/read_test.go`

**Interfaces:**
- Produces: `func gitsubject.CanonicalizeRepositoryIdentity(raw string) (string, error)`
- Produces: `func localinput.OpenWorkspaceRegularFile(root, relativePath string) (*os.File, error)`
- Preserves: `func localinput.ReadWorkspaceFile(root, relativePath string) ([]byte, error)`, including `MaxWorkspaceInputBytes == 1 << 20`

- [ ] **Step 1: Write failing tests for the exported repository canonicalization seam**

In `internal/gitsubject/identity_test.go`, add a test that calls `CanonicalizeRepositoryIdentity` directly and proves at least:
- raw `https://github.com/acme/repo.git` → `github.com/acme/repo`;
- raw `git@github.com:acme/repo.git` → the same identity;
- already-canonical hosted `github.com/acme/repo` round-trips unchanged;
- canonical self-hosted `https://git.example.com/proj/repo` round-trips unchanged;
- canonical self-hosted `ssh://git@git.example.com/proj/repo` round-trips unchanged;
- resolver-produced canonical SCP identity `ssh+scp://git@git.example.com/proj/repo` round-trips unchanged;
- a valid resolver-generated `local://sha256/<64 lowercase hex>` identity is preserved exactly;
- malformed `local://` identities are rejected;
- malformed credential/percent-encoded remote inputs remain rejected exactly as the existing private URI helper rejects them.

Also retain/add a regression that `Resolve`'s explicit repository-URI path does **not** begin accepting caller-supplied `local://` values.

- [ ] **Step 2: Run the focused Git subject test and observe RED**

Run:

```bash
go test ./internal/gitsubject -run 'TestCanonicalizeRepositoryIdentity|TestCanonicalizeRepositoryURI'
```

Expected: FAIL to compile because `CanonicalizeRepositoryIdentity` does not exist.

- [ ] **Step 3: Expose the existing canonicalizer without changing its rules**

Add:

```go
func CanonicalizeRepositoryIdentity(raw string) (string, error)
```

The function must accept both raw supported repository forms and the canonical identities the resolver itself can emit. Handle canonical `local://sha256/<64 lowercase hex>`, allowlisted hosted `host/path`, and canonical self-hosted outputs including `ssh+scp://git@...`; delegate ordinary raw HTTP/HTTPS/SSH/SCP inputs to the existing private URI canonicalizer. Keep `Resolve`'s explicit/origin input path on the existing private URI rules so this comparison seam does not make `local://` or `ssh+scp://` newly user-selectable origin syntax. Do not introduce a new package, interface, provider registry, or alternate identity model.

- [ ] **Step 4: Run Git subject tests GREEN**

Run:

```bash
go test ./internal/gitsubject
```

Expected: PASS.

- [ ] **Step 5: Write failing tests for a reusable workspace-bounded regular-file opener**

Create `internal/localinput/read_test.go` with `TestOpenWorkspaceRegularFile` covering:
- normal regular file under root opens successfully;
- `../` relative escape rejects;
- symlink resolving outside root rejects where symlinks are supported;
- directory/non-regular target rejects.

Add a regression in the same file proving `ReadWorkspaceFile` still rejects a file larger than `MaxWorkspaceInputBytes`.

- [ ] **Step 6: Run the focused localinput test and observe RED**

Run:

```bash
go test ./internal/localinput -run 'TestOpenWorkspaceRegularFile|TestReadWorkspaceFileStillBoundsInput'
```

Expected: FAIL to compile because `OpenWorkspaceRegularFile` does not exist.

- [ ] **Step 7: Extract the minimal opener**

Implement:

```go
func OpenWorkspaceRegularFile(root, relativePath string) (*os.File, error)
```

using the existing root/path normalization, symlink-containment, and regular-file checks. Refactor `ReadWorkspaceFile` to call it, then keep its existing pre-read/stat and `io.LimitReader(..., MaxWorkspaceInputBytes+1)` bound.

Do not add an artifact-size limit to the opener.

- [ ] **Step 8: Run supporting-package tests GREEN**

Run:

```bash
go test ./internal/gitsubject ./internal/localinput ./internal/contract ./internal/policy
```

Expected: PASS with contract/policy behavior unchanged.

- [ ] **Step 9: Commit Task 1**

```bash
git add internal/gitsubject/identity.go internal/gitsubject/identity_test.go internal/localinput/read.go internal/localinput/read_test.go
git commit -m "refactor: expose evidence input safety seams"
```

---

### Task 2: Add Typed Envelope Decoding and Intrinsic Field Validation

**Files:**
- Create: `internal/evidence/model.go`
- Create: `internal/evidence/loader.go`
- Create: `internal/evidence/loader_test.go`

**Interfaces:**
- Consumes: `strictjson.Decode(data []byte, dst any) error`
- Consumes: `inputmeta.Digest`, `inputmeta.SHA256`, `domain.ObservedOutcome`
- Produces: `const SchemaVersion = "assurectl/evidence-envelope/v0"`
- Produces: typed `Envelope`, `Subject`, `Producer`, `Invocation`, `Outcome`, `Artifact`
- Produces internal: `func decodeEnvelope(data []byte) (decodedEnvelope, error)`
- Produces internal: `decodedEnvelope{Envelope Envelope, StartedAt time.Time, FinishedAt time.Time, Digest inputmeta.Digest}`

The normalized typed model uses pointers for optional values:
- `Producer.Workflow *string`
- `Producer.WorkflowRevision *string`
- `Invocation.EnvironmentDigest *inputmeta.Digest`
- `Outcome.ExitCode *big.Int`

However, pointers alone do **not** distinguish an omitted JSON field from an explicit `null`. Therefore strict decoding must first target private raw DTOs whose optional fields are `json.RawMessage`, then normalize them into the typed pointers only after presence/type validation. For optional nested objects such as `environment_digest`, run `strictjson.Decode` again on the raw object so duplicate/unknown nested keys remain fail-closed.

- [ ] **Step 1: Write RED tests for one valid envelope and canonical digest**

Add `TestDecodeEnvelopeReturnsTypedValueTimesAndCanonicalDigest` using the existing v0 example shape. Assert:
- schema/id/type values;
- parsed `StartedAt` and `FinishedAt`;
- `Outcome.Status == domain.OutcomePassed`;
- environment/artifact digests remain `sha256`;
- semantically identical JSON differing only in whitespace/object-key order produces the same envelope digest.

- [ ] **Step 2: Run the focused test and observe RED**

Run:

```bash
go test ./internal/evidence -run 'TestDecodeEnvelopeReturnsTypedValueTimesAndCanonicalDigest'
```

Expected: FAIL because the package/functions do not exist.

- [ ] **Step 3: Implement the typed model and strict decode skeleton**

Implement the exact v0 typed field structure plus private raw DTOs. Decode the top-level document through `strictjson.Decode` into the raw DTO, normalize presence-sensitive optional `json.RawMessage` fields into the typed model, validate `schema_version`, then compute envelope digest only after all intrinsic validation in this task succeeds.

Do not add filesystem, subject, policy, trust, or final evidence-state behavior.

- [ ] **Step 4: Run the valid-envelope test GREEN**

Run the same command. Expected: PASS.

- [ ] **Step 5: Add RED table tests for malformed/ambiguous envelope content**

Add `TestDecodeEnvelopeRejectsMalformedV0Input` covering:
- unsupported schema version;
- unknown/self-asserted trust field;
- duplicate JSON key;
- case-alias field such as `Schema_Version`;
- invalid evidence ID;
- empty or over-bound required strings;
- invalid 40/64-character object IDs;
- digest algorithm other than `sha256`;
- malformed digest hex;
- unknown outcome;
- impossible timestamp;
- `finished_at < started_at`;
- explicitly empty `producer.workflow`;
- explicit `null` for `producer.workflow`;
- explicitly empty `producer.workflow_revision`;
- explicit `null` for `producer.workflow_revision`;
- malformed optional environment digest;
- explicit `null` for `invocation.environment_digest`.

- [ ] **Step 6: Observe RED for semantic validation**

Run:

```bash
go test ./internal/evidence -run 'TestDecodeEnvelopeRejectsMalformedV0Input'
```

Expected: FAIL on the first not-yet-enforced semantic case, not on test-fixture construction.

- [ ] **Step 7: Implement minimal schema-aligned validators**

Implement only constraints already present in the current schema/protocol:
- identifier regex;
- rune-count bounds;
- object-ID regex;
- SHA-256 shape;
- closed outcome;
- RFC3339 lexical assertion plus `time.Parse(time.RFC3339Nano, ...)`;
- `finished_at >= started_at`;
- presence-sensitive optional validation.

Do not invent `PASSED => exit_code == 0` or any other cross-field rule absent from the protocol.

- [ ] **Step 8: Add RED tests for JSON Schema integer semantics of `exit_code`**

Add `TestDecodeEnvelopeNormalizesJSONSchemaIntegerExitCode` asserting:
- `1000`, `1000.0`, and `1e3` normalize to equal `big.Int(1000)`;
- negative integer values are accepted;
- an integer beyond int64 is accepted;
- `1.5` is rejected;
- explicit `null` is rejected if present;
- omission yields `nil`.

- [ ] **Step 9: Observe RED for exponent/large integer cases**

Run:

```bash
go test ./internal/evidence -run 'TestDecodeEnvelopeNormalizesJSONSchemaIntegerExitCode'
```

Expected: FAIL until mathematical-integer normalization is implemented.

- [ ] **Step 10: Implement presence-sensitive integer normalization**

Parse the raw JSON number through standard-library arbitrary precision (for example `big.Rat`), require `IsInt()`, and store the normalized signed value in `*big.Int`. Invalid JSON syntax is already rejected by strict decoding; do not add a numeric bound the schema does not define.

- [ ] **Step 11: Run Task 2 tests GREEN**

Run:

```bash
go test ./internal/evidence
```

Expected: PASS.

- [ ] **Step 12: Commit Task 2**

```bash
git add internal/evidence/model.go internal/evidence/loader.go internal/evidence/loader_test.go
git commit -m "feat: decode strict evidence envelopes"
```

---

### Task 3: Add Deterministic Local Evidence Discovery

**Files:**
- Modify: `internal/evidence/loader.go`
- Modify: `internal/evidence/loader_test.go`
- Create: `internal/evidence/security_regression_test.go`

**Interfaces:**
- Consumes: `localinput.ReadWorkspaceFile(root, relativePath string) ([]byte, error)`
- Produces internal: `func discoverLocal(root string) (localDiscovery, error)`
- Produces internal: `localDiscovery{EvidenceRoot string, Candidates []localCandidate}`
- Produces internal: `localCandidate{RelativePath string, Source string, Data []byte}`
- Fixed evidence root: `.assurectl/evidence`
- Source form: `workspace:.assurectl/evidence/<filename>`

- [ ] **Step 1: Write RED discovery tests**

Add tests proving:
- missing `.assurectl/evidence/` returns an empty candidate slice and nil error;
- only direct regular `*.json` files are candidates;
- nested `artifacts/*.json` and other nested JSON are not discovered;
- non-JSON direct files are ignored;
- candidates are returned in lexicographic canonical filename order;
- source strings use forward slashes and the fixed `workspace:.assurectl/evidence/` prefix.

- [ ] **Step 2: Run focused discovery tests and observe RED**

Run:

```bash
go test ./internal/evidence -run 'TestDiscoverLocal'
```

Expected: FAIL because `discoverLocal` does not exist.

- [ ] **Step 3: Implement non-recursive deterministic discovery**

Resolve the workspace root and logical evidence root fail-closed. Treat only a genuinely missing evidence directory as empty input. If the evidence root exists, require it to resolve inside the workspace and be a directory.

Use `os.ReadDir`, identify direct regular `*.json` candidates, sort by canonical filename, and read candidate bytes through `localinput.ReadWorkspaceFile`. Return the resolved, workspace-contained evidence-root path in `localDiscovery.EvidenceRoot`; this exact resolved root is the containment boundary later used for artifact opening.

Do not recursively walk.

- [ ] **Step 4: Add RED security regressions for the evidence root**

In `security_regression_test.go`, add:
- evidence root symlink resolving outside workspace → error;
- evidence root path exists as a regular file → error;
- unreadable/unsafe candidate that is selected as a regular `*.json` must fail the discovery/load path rather than be silently dropped.

Skip only OS-specific symlink/permission cases when the platform genuinely cannot create or enforce them; do not convert a supported failure into a skip.

- [ ] **Step 5: Run security discovery tests and observe RED**

Run:

```bash
go test ./internal/evidence -run 'TestDiscoverLocalRejects'
```

Expected: FAIL on the unimplemented containment/type rule.

- [ ] **Step 6: Implement the minimal root/candidate safety checks**

Keep the checks local to discovery unless an existing `localinput` seam exactly fits. Do not create a general filesystem framework.

- [ ] **Step 7: Run Task 3 tests GREEN**

Run:

```bash
go test ./internal/evidence
```

Expected: PASS.

- [ ] **Step 8: Commit Task 3**

```bash
git add internal/evidence/loader.go internal/evidence/loader_test.go internal/evidence/security_regression_test.go
git commit -m "feat: discover local evidence deterministically"
```

---

### Task 4: Add Exact Repository and Revision Binding Facts

**Files:**
- Create: `internal/evidence/subject.go`
- Create: `internal/evidence/subject_test.go`
- Modify: `internal/evidence/model.go`

**Interfaces:**
- Consumes: `gitsubject.CanonicalizeRepositoryIdentity(raw string) (string, error)`
- Consumes: `domain.Subject`
- Produces:
  - `type SubjectBinding string`
  - `const SubjectBindingExact SubjectBinding = "EXACT"`
  - `const SubjectBindingOtherRevision SubjectBinding = "OTHER_REVISION"`
- Produces internal: `func validateResolvedSubject(resolved domain.Subject) error`
- Produces internal: `func bindSubject(evidenceSubject Subject, resolved domain.Subject) (SubjectBinding, error)`

- [ ] **Step 1: Write RED binding tests**

Add `TestBindSubject` cases:
- evidence HTTPS URI and resolved canonical GitHub identity normalize to the same repository + same head → `EXACT`;
- matching valid `local://sha256/...` evidence/resolved identities + same head → `EXACT`;
- same repository + different valid revision → `OTHER_REVISION`;
- different repository → error;
- malformed evidence repository URI → error.

- [ ] **Step 2: Add Review Focus RED tests for malformed caller subject**

Add `TestValidateResolvedSubject` cases where `domain.Subject` is manually constructed with:
- malformed/non-canonical repository URI;
- empty repository URI;
- invalid head revision.

Expected: fail closed before any envelope binding is attempted. Also keep a `bindSubject` regression proving it calls the same validation rather than bypassing it.

For repository URI, canonicalize the caller value and require the canonicalized value to equal the supplied resolved value; the evidence package must not silently “repair” a supposedly resolved subject.

- [ ] **Step 3: Run subject tests and observe RED**

Run:

```bash
go test ./internal/evidence -run 'TestBindSubject'
```

Expected: FAIL because binding types/functions do not exist.

- [ ] **Step 4: Implement subject binding only**

Implement `validateResolvedSubject` plus repository canonical comparison and revision comparison. Reuse the same 40/64 lowercase object-ID grammar used by envelope validation for the resolved head. `bindSubject` must call `validateResolvedSubject`.

Do not inspect base revision, change-set digest, policy, producer, or wall clock in this helper.

- [ ] **Step 5: Run subject tests GREEN**

Run:

```bash
go test ./internal/evidence -run 'TestBindSubject'
```

Expected: PASS.

- [ ] **Step 6: Commit Task 4**

```bash
git add internal/evidence/model.go internal/evidence/subject.go internal/evidence/subject_test.go
git commit -m "feat: bind evidence to resolved subjects"
```

---

### Task 5: Add Portable Artifact Resolution and Streaming Digest Verification

**Files:**
- Create: `internal/evidence/artifact.go`
- Create: `internal/evidence/artifact_test.go`
- Modify: `internal/evidence/security_regression_test.go`

**Interfaces:**
- Consumes: `localinput.OpenWorkspaceRegularFile(root, relativePath string) (*os.File, error)`
- Consumes: `inputmeta.Digest`
- Produces internal: `func verifyArtifact(evidenceRoot, uri string, expected inputmeta.Digest) error`
- Produces internal: `func validatePortableArtifactURI(uri string) error`
- Artifact filesystem root: the resolved `localDiscovery.EvidenceRoot`, never the broader workspace root

- [ ] **Step 1: Write RED lexical path tests**

Add `TestValidatePortableArtifactURI` with one accepted path:

```text
artifacts/unit-tests.json
```

and rejection cases for:
- `/artifact.json`;
- `../artifact.json`;
- `artifacts/../artifact.json`;
- `./artifacts/x.json`;
- `artifacts//x.json`;
- `artifacts\x.json`;
- `C:\\x.json`;
- UNC-style input;
- `file:///x.json`;
- `https://example.com/x`;
- segments containing `:`;
- control characters;
- trailing dot/space segments;
- case-insensitive Windows device aliases such as `CON`, `NUL.txt`, `COM1.log`, and `LPT9`.

- [ ] **Step 2: Run lexical path tests and observe RED**

Run:

```bash
go test ./internal/evidence -run 'TestValidatePortableArtifactURI'
```

Expected: FAIL because the validator does not exist.

- [ ] **Step 3: Implement portable path validation**

Validate the slash-separated logical string before converting it to an OS path. Do not use URL fetching and do not normalize unsafe input into acceptance.

- [ ] **Step 4: Add RED artifact integrity tests**

Add `TestVerifyArtifact` covering:
- regular local artifact with matching SHA-256 → success;
- missing artifact → error;
- directory/non-regular target → error;
- digest mismatch → error;
- unsupported digest algorithm → error.

Add a test using a multi-megabyte artifact to prove verification is not subject to `MaxWorkspaceInputBytes`. The test need not measure memory; it pins the absence of the configuration-file size cap.

- [ ] **Step 5: Add RED symlink escape regression**

Where symlinks are supported:
- `.assurectl/evidence/artifacts/link.json` → file outside the evidence root must error, including a target that is still inside the workspace but elsewhere under that workspace.

Also test a symlink that remains inside the evidence root if the spec/platform behavior permits it; it may succeed only when final resolved target remains inside the root and is regular.

- [ ] **Step 6: Run artifact tests and observe RED**

Run:

```bash
go test ./internal/evidence -run 'TestVerifyArtifact|TestArtifactSymlink'
```

Expected: FAIL until artifact verification exists.

- [ ] **Step 7: Implement streaming SHA-256 verification**

After URI validation, call `OpenWorkspaceRegularFile(evidenceRoot, uri)` so symlink/path containment is enforced against the resolved evidence root itself, not merely against the broader workspace. Hash with `sha256.New()` + `io.Copy` and compare lowercase hex digest exactly to the validated expected digest.

Do not parse artifact contents by media type.

- [ ] **Step 8: Run Task 5 tests GREEN**

Run:

```bash
go test ./internal/evidence ./internal/localinput
```

Expected: PASS.

- [ ] **Step 9: Commit Task 5**

```bash
git add internal/evidence/artifact.go internal/evidence/artifact_test.go internal/evidence/security_regression_test.go
git commit -m "feat: verify local evidence artifacts"
```

---

### Task 6: Compose `LoadLocal`, Set-Wide Fail-Closed Semantics, and Determinism

**Files:**
- Modify: `internal/evidence/model.go`
- Modify: `internal/evidence/loader.go`
- Modify: `internal/evidence/loader_test.go`
- Modify: `internal/evidence/security_regression_test.go`
- Create: `internal/evidence/determinism_test.go`

**Interfaces:**
- Consumes: Tasks 2–5 helpers.
- Produces:

```go
func LoadLocal(root string, subject domain.Subject) ([]Loaded, error)
```

- Produces `Loaded` with exactly:
  - `Envelope Envelope`
  - `Source string`
  - `Digest inputmeta.Digest`
  - `TrustStatus inputmeta.TrustStatus`
  - `AuthorityBasis inputmeta.AuthorityBasis`
  - `StartedAt time.Time`
  - `FinishedAt time.Time`
  - `SubjectBinding SubjectBinding`

No final evidence state field is added.

- [ ] **Step 1: Write RED happy-path integration test**

Add `TestLoadLocalReturnsTypedAdvisoryEvidence` that creates:
- two direct envelope JSON files in reverse creation order;
- two referenced artifacts with correct digests;
- one exact-head envelope and one same-repository other-revision envelope.

Assert:
- two results in filename order;
- source strings are deterministic;
- both `TrustStatus == inputmeta.TrustStatusUntrusted`;
- both `AuthorityBasis == inputmeta.AuthorityBasisAdvisoryWorkspace`;
- one binding is `EXACT`, one is `OTHER_REVISION`;
- outcomes are preserved independently of binding;
- parsed times are present;
- envelope digests are populated.

- [ ] **Step 2: Run the integration test and observe RED**

Run:

```bash
go test ./internal/evidence -run 'TestLoadLocalReturnsTypedAdvisoryEvidence'
```

Expected: FAIL because `LoadLocal` / `Loaded` composition is incomplete.

- [ ] **Step 3: Implement minimal `LoadLocal` composition**

Order:
1. call `validateResolvedSubject(subject)` even when the evidence directory is absent/empty;
2. discover candidates;
3. strict-decode each envelope;
4. reject duplicate evidence IDs and duplicate canonical artifact URIs set-wide;
5. bind subject;
6. verify the referenced artifact against `localDiscovery.EvidenceRoot`;
7. construct `Loaded`;
8. return only after the complete set succeeds.

Do not return partial results with a non-nil error.

- [ ] **Step 4: Add RED duplicate-ID and partial-set regressions**

Add:
- two valid files with same envelope `id` → error naming duplicate ID;
- two otherwise valid envelopes referencing the same canonical `artifact.uri` → error naming duplicate artifact path;
- first file valid + later artifact digest mismatch → `LoadLocal` returns error and no accepted slice;
- repository mismatch in any member → complete call fails;
- malformed member among valid siblings → complete call fails;
- malformed caller `domain.Subject` + missing evidence directory → error rather than a silently successful empty set.

- [ ] **Step 5: Run fail-closed set tests and observe RED**

Run:

```bash
go test ./internal/evidence -run 'TestLoadLocalRejectsDuplicateEvidenceID|TestLoadLocalFailsClosedForInvalidMember'
```

Expected: FAIL until set-wide validation is complete.

- [ ] **Step 6: Implement set-wide uniqueness and all-or-error return**

Use one map keyed by canonical evidence ID and one map keyed by the already-canonical accepted `artifact.uri`. Keep deterministic candidate order; do not reorder by ID or artifact path after discovery unless the spec is deliberately changed.

- [ ] **Step 7: Write determinism tests**

Create `determinism_test.go` with:
- same typed envelope represented with different JSON whitespace/object-key order → same envelope digest;
- changed supplied timestamp spelling representing the same instant → different envelope digest while parsed instants compare equal;
- changed supplied repository URI spelling that canonicalizes to same repository → different envelope digest while subject binding remains exact;
- repeated loads of identical filesystem state → deeply equal ordered `[]Loaded`.

This pins the distinction between normalized typed-input digest and comparison facts.

- [ ] **Step 8: Run determinism tests GREEN**

Run:

```bash
go test ./internal/evidence -run 'TestLoadLocalDeterministic|TestEnvelopeDigest'
```

Expected: PASS.

- [ ] **Step 9: Run the complete evidence package with race detection**

Run:

```bash
go test -race ./internal/evidence
```

Expected: PASS.

- [ ] **Step 10: Commit Task 6**

```bash
git add internal/evidence
git commit -m "feat: load intrinsic local evidence"
```

---

### Task 7: Full Verification, Scope Audit, and Documentation Alignment

**Files:**
- Modify only if needed: `docs/superpowers/specs/2026-09-29-m1-evidence-ingestion-design.md`
- Modify this plan only for factual execution evidence if the project convention requires it.
- No new product behavior.

**Interfaces:**
- Verifies: the completed evidence-ingestion slice only.
- Produces: exact-head verification evidence suitable for the PR description.

- [ ] **Step 1: Run focused package tests**

```bash
go test ./internal/gitsubject ./internal/localinput ./internal/evidence ./internal/contract ./internal/policy
```

Expected: PASS.

- [ ] **Step 2: Run required repository verification**

```bash
mapfile -d '' files < <(find . -name '*.go' -type f -not -path './vendor/*' -print0)
test -z "$(gofmt -l "${files[@]}")"
go vet ./...
go test -race ./...
go build -trimpath -o /tmp/assurectl ./cmd/assurectl
```

Expected: every command exits 0.

- [ ] **Step 3: Audit the diff against explicit scope exclusions**

Run:

```bash
git diff --name-only main...HEAD
git diff --stat main...HEAD
```

Confirm no behavior changes under:
- `cmd/assurectl/*`;
- `internal/decision/*`;
- `internal/domain/evidence.go`;
- `schemas/*.json`;
- receipt/waiver/policy-evaluation code.

Confirm no new module dependency was introduced.

- [ ] **Step 4: Re-read the final diff for trust-boundary overclaims**

Specifically verify:
- no self-asserted evidence trust field is consumed;
- no `time.Now()` freshness check exists;
- no repository mismatch is converted to `OTHER_REVISION`;
- no exact binding is called `VALID`;
- no partial result slice is returned on ingestion error;
- no arbitrary network or shell execution exists;
- comments/docs do not claim TOCTOU-race-free protection.

- [ ] **Step 5: Commit documentation-only status/evidence update if needed**

If implementation exactly matches the approved design, update the design status from `Approved for implementation planning` to `Implemented in M1 evidence-ingestion slice` only after all verification above is green.

Use a docs-only commit, for example:

```bash
git add docs/superpowers/specs/2026-09-29-m1-evidence-ingestion-design.md docs/superpowers/plans/2026-09-29-m1-evidence-ingestion.md
git commit -m "docs: record M1 evidence ingestion verification"
```

Do not make production fixes in this step; any discovered behavior defect returns to the owning task with RED → GREEN evidence.

- [ ] **Step 6: Push and require exact-head CI**

The exact branch head must pass the repository's Go 1.26.x / Go 1.27.x GitHub Actions matrix before merge readiness is claimed.

Record:
- exact head SHA;
- workflow run number/ID;
- formatting result;
- vet result;
- race-test result;
- CLI-build result.

- [ ] **Step 7: Final review gate**

Review the complete PR against:
- this plan;
- the evidence-ingestion design spec;
- ADR-0003;
- ADR-0007;
- `docs/threat-model/v0.md`.

Any requested change that adds policy matching, producer trust, freshness-age calculation, findings, `verify`, or receipts is a new slice unless it fixes a concrete bug in the approved ingestion contract.

## Exit Criteria

This implementation plan is complete only when the resulting PR demonstrates all of the following:

- local evidence discovery is deterministic and non-recursive;
- missing evidence directory returns an empty set, not `MISSING`;
- malformed/ambiguous v0 envelope input fails closed;
- timestamps are lexically and semantically valid and ordered;
- optional fields preserve absence versus invalid explicit presence;
- `exit_code` follows JSON Schema mathematical-integer semantics without an invented range;
- local artifact paths are portable-relative and cannot escape the evidence/workspace root through covered path/symlink cases;
- artifact SHA-256 is verified via streaming without inheriting the 1 MiB configuration read cap;
- repository mismatch fails;
- another revision is represented as a fact distinct from final `STALE`;
- exact subject binding is not mislabeled as final `VALID`;
- envelope digest is deterministic over the typed supplied input;
- duplicate evidence IDs fail the complete local set;
- one invalid member prevents a misleading partially accepted set;
- duplicate canonical artifact URIs across the local set fail closed;
- local evidence source metadata remains externally untrusted/advisory;
- no network, new dependency, schema change, CLI evaluation, requirement evaluation, final evidence state, finding, verdict, decision, waiver, or receipt behavior entered the slice;
- exact-head full CI is green before merge readiness is claimed.
