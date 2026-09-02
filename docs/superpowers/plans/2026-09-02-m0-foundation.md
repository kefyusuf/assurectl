# AssureCTL M0 Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish a reviewable, test-backed AssureCTL repository foundation with the initial domain vocabulary, deterministic verdict/decision precedence, a buildable CLI shell, protocol placeholders, project governance files, and CI.

**Architecture:** M0 keeps all unstable implementation packages under `internal/`, uses the Go standard library only, and makes the decision rules executable before adding YAML parsing, Git subject resolution, evidence loading, or receipt generation. The CLI remains a thin shell; domain types and decision logic are independently testable. Protocol examples and ADRs document the trust boundaries without making authoritative security claims.

**Tech Stack:** Go 1.26 minimum, Go standard library, GitHub Actions, Markdown, JSON Schema Draft 2020-12.

**Spec:** `docs/superpowers/specs/2026-09-02-assurectl-foundation-design.md`

## Global Constraints

- Product scope is software-change assurance; M0 must not expand into a CI platform, agent orchestrator, migration generator, SaaS dashboard, or generic agent-safety framework.
- The evaluator is deterministic and fail-closed.
- Evidence state, observed outcome, verification verdict, finding, waiver, and completion decision remain separate concepts.
- A waiver never converts failed, missing, stale, invalid, or untrusted evidence into passing evidence.
- Protocol invariants are non-waivable.
- The worker proposing a change is not the authority that marks it complete.
- Local M0 output is advisory only and makes no cryptographic authority claim.
- No custom signing primitive, network fetch, embedded policy code, or arbitrary shell execution is introduced.
- Public packages are deferred; implementation remains under `internal/` until external interfaces stabilize.
- M0 uses no third-party Go dependency.
- Minimum Go version is `1.26.0`; CI verifies Go `1.26.x` and `1.27.x`.

## Planned File Map

```text
.
├── .github/
│   ├── pull_request_template.md
│   └── workflows/ci.yml
├── cmd/assurectl/main.go
├── internal/
│   ├── buildinfo/buildinfo.go
│   ├── decision/evaluate.go
│   ├── decision/evaluate_test.go
│   └── domain/
│       ├── authority.go
│       ├── authority_test.go
│       ├── evidence.go
│       ├── evidence_test.go
│       ├── finding.go
│       ├── result.go
│       ├── result_test.go
│       ├── subject.go
│       ├── waiver.go
│       └── workunit.go
├── schemas/
│   ├── evidence-envelope.v0.schema.json
│   ├── policy.v0.schema.json
│   ├── receipt.v0.schema.json
│   └── verification-contract.v0.schema.json
├── spec/README.md
├── examples/basic/
│   ├── evidence/unit-tests.json
│   ├── policy.json
│   └── work-unit.json
├── docs/
│   ├── adr/
│   │   ├── 0001-completion-assurance-authority.md
│   │   ├── 0002-separate-verdict-waiver-and-decision.md
│   │   ├── 0003-revision-bound-subjects.md
│   │   ├── 0004-trusted-base-policy-resolution.md
│   │   ├── 0005-open-deterministic-kernel.md
│   │   └── 0006-standard-attestation-direction.md
│   ├── architecture/overview.md
│   ├── protocol/v0.md
│   └── threat-model/v0.md
├── .gitignore
├── AGENTS.md
├── CONTRIBUTING.md
├── LICENSE
├── README.md
├── SECURITY.md
└── go.mod
```

---

### Task 1: Establish module, build metadata, CLI shell, and CI

**Files:**
- Create: `go.mod`
- Create: `.gitignore`
- Create: `internal/buildinfo/buildinfo.go`
- Create: `cmd/assurectl/main.go`
- Create: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: none.
- Produces: `buildinfo.Version`, `buildinfo.Commit`, `buildinfo.Date`, and a buildable `assurectl` command supporting `version` and help output.

- [ ] **Step 1: Write the failing build-info test**

Create `internal/buildinfo/buildinfo_test.go`:

```go
package buildinfo

import "testing"

func TestSummary(t *testing.T) {
	t.Parallel()

	Version = "v0.0.0-test"
	Commit = "abc123"
	Date = "2026-09-02T00:00:00Z"

	got := Summary()
	want := "assurectl v0.0.0-test (commit abc123, built 2026-09-02T00:00:00Z)"
	if got != want {
		t.Fatalf("Summary() = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run the focused test and confirm RED**

Run:

```bash
go test ./internal/buildinfo -run TestSummary -count=1
```

Expected: compilation failure because `Version`, `Commit`, `Date`, and `Summary` do not exist.

- [ ] **Step 3: Implement minimal build metadata**

Create `internal/buildinfo/buildinfo.go`:

```go
package buildinfo

import "fmt"

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

func Summary() string {
	return fmt.Sprintf("assurectl %s (commit %s, built %s)", Version, Commit, Date)
}
```

- [ ] **Step 4: Add the module and CLI shell**

Create `go.mod`:

```go
module github.com/kefyusuf/assurectl

go 1.26.0
```

Create `cmd/assurectl/main.go` with these exact behaviors:

- no arguments or `help`, `-h`, `--help`: print usage to stdout and exit `0`;
- `version`, `-v`, `--version`: print `buildinfo.Summary()` to stdout and exit `0`;
- any other command: print `unknown command: <value>` and usage to stderr, then exit `64`;
- no domain evaluation is performed in M0.

Use a testable `run(args []string, stdout, stderr io.Writer) int` function and keep `main()` limited to `os.Exit(run(...))`.

- [ ] **Step 5: Add CLI behavior tests**

Create `cmd/assurectl/main_test.go` covering:

```go
func TestRunHelp(t *testing.T)
func TestRunVersion(t *testing.T)
func TestRunUnknownCommand(t *testing.T)
```

Assertions:

- help contains `Usage: assurectl <command>` and returns `0`;
- version contains `assurectl` and returns `0`;
- unknown command returns `64`, writes nothing to stdout, and includes the command in stderr.

- [ ] **Step 6: Run focused tests and confirm GREEN**

Run:

```bash
go test ./internal/buildinfo ./cmd/assurectl -count=1
```

Expected: both packages pass.

- [ ] **Step 7: Add CI and ignore rules**

Create `.gitignore` with:

```text
/bin/
/dist/
/coverage.out
*.test
.assurectl/receipts/
.DS_Store
```

Create `.github/workflows/ci.yml` with:

- events: pull requests and pushes to `main`;
- least-privilege `contents: read` permission;
- matrix Go `1.26.x` and `1.27.x`;
- `actions/checkout@v4`;
- `actions/setup-go@v5` with dependency cache enabled;
- `gofmt` check using `test -z "$(gofmt -l .)"`;
- `go vet ./...`;
- `go test -race -coverprofile=coverage.out ./...`;
- `go build ./cmd/assurectl`.

- [ ] **Step 8: Run the complete Task 1 verification**

Run:

```bash
gofmt -w cmd internal
test -z "$(gofmt -l cmd internal)"
go vet ./...
go test -race ./...
go build ./cmd/assurectl
```

Expected: every command exits `0`.

- [ ] **Step 9: Commit**

```bash
git add go.mod .gitignore .github/workflows/ci.yml cmd/assurectl internal/buildinfo
git commit -m "chore: establish Go CLI and CI baseline"
```

---

### Task 2: Define the initial domain vocabulary as typed values

**Files:**
- Create: `internal/domain/authority.go`
- Create: `internal/domain/authority_test.go`
- Create: `internal/domain/evidence.go`
- Create: `internal/domain/evidence_test.go`
- Create: `internal/domain/finding.go`
- Create: `internal/domain/result.go`
- Create: `internal/domain/result_test.go`
- Create: `internal/domain/subject.go`
- Create: `internal/domain/waiver.go`
- Create: `internal/domain/workunit.go`

**Interfaces:**
- Consumes: standard-library types only.
- Produces: closed string enums with `Valid() bool`, plus focused data structures used by the decision engine.

- [ ] **Step 1: Write failing enum validation tests**

Create table-driven tests for these exact values:

```go
AuthorityTier: ADVISORY, CI_ASSERTED, AUTHORITATIVE
EvidenceState: VALID, INVALID, MISSING, STALE, UNTRUSTED
ObservedOutcome: PASSED, FAILED, ERROR
VerificationVerdict: PASSED, FAILED, INDETERMINATE
CompletionDecision: APPROVED, REJECTED, BLOCKED, ACCEPTED_WITH_RISK
FindingSeverity: INFO, WARNING, ERROR
WorkUnitType: pull_request, commit, release_candidate, api_migration,
              sdk_migration, framework_upgrade, dependency_upgrade,
              security_remediation
```

Each table must assert that listed values are valid and an unknown value is invalid.

- [ ] **Step 2: Run the domain tests and confirm RED**

Run:

```bash
go test ./internal/domain -count=1
```

Expected: compilation failure because domain types are undefined.

- [ ] **Step 3: Implement the closed enums**

Create one focused source file per concern. Each string type exposes:

```go
func (v SomeType) Valid() bool
```

Use explicit `switch` statements. Do not accept empty strings, aliases, or case-insensitive input.

- [ ] **Step 4: Define immutable-input domain records**

Add these structures without behavior beyond data representation:

```go
type Subject struct {
	RepositoryURI  string `json:"repository_uri"`
	BaseRevision  string `json:"base_revision"`
	HeadRevision  string `json:"head_revision"`
	ChangeSetAlgo string `json:"change_set_algorithm"`
	ChangeSetHash string `json:"change_set_digest"`
}

type WorkUnit struct {
	ID        string       `json:"id"`
	Type      WorkUnitType `json:"type"`
	Objective string       `json:"objective"`
}

type Finding struct {
	Code          string          `json:"code"`
	Severity      FindingSeverity `json:"severity"`
	RequirementID string          `json:"requirement_id,omitempty"`
	EvidenceIDs   []string        `json:"evidence_ids,omitempty"`
	Message       string          `json:"message"`
	Blocking      bool            `json:"blocking"`
}

type WaiverStatus string

const (
	WaiverNotApplicable WaiverStatus = "NOT_APPLICABLE"
	WaiverValid         WaiverStatus = "VALID"
	WaiverInvalid       WaiverStatus = "INVALID"
)

type RequirementResult struct {
	RequirementID string          `json:"requirement_id"`
	EvidenceState EvidenceState   `json:"evidence_state"`
	Outcome       ObservedOutcome `json:"outcome,omitempty"`
	Waivable      bool            `json:"waivable"`
	WaiverStatus  WaiverStatus    `json:"waiver_status"`
}

type EvaluationResult struct {
	Verdict  VerificationVerdict `json:"verification_verdict"`
	Decision CompletionDecision  `json:"completion_decision"`
}
```

`WaiverStatus.Valid()` accepts only the three declared values.

- [ ] **Step 5: Add zero-value and JSON round-trip tests**

Add tests proving:

- empty enum values are invalid;
- a `RequirementResult` with declared values survives JSON marshal/unmarshal unchanged;
- `Subject` serializes the algorithm separately from the digest;
- no custom JSON unmarshalling silently coerces unknown values in M0.

- [ ] **Step 6: Run complete domain verification**

Run:

```bash
gofmt -w internal/domain
go test ./internal/domain -count=1
go vet ./internal/domain
```

Expected: all commands exit `0`.

- [ ] **Step 7: Commit**

```bash
git add internal/domain
git commit -m "feat: define assurance domain primitives"
```

---

### Task 3: Make verdict and completion-decision precedence executable

**Files:**
- Create: `internal/decision/evaluate.go`
- Create: `internal/decision/evaluate_test.go`

**Interfaces:**
- Consumes: `[]domain.RequirementResult`.
- Produces: `domain.EvaluationResult` and a validation error for malformed enum inputs.
- Public internal signature:

```go
func Evaluate(requirements []domain.RequirementResult) (domain.EvaluationResult, error)
```

- [ ] **Step 1: Write the decision-table tests first**

Create a table covering at least these cases:

| Case | Requirements | Verdict | Decision |
|---|---|---|---|
| no requirements | empty | `INDETERMINATE` | `BLOCKED` |
| all valid and passed | all `VALID/PASSED` | `PASSED` | `APPROVED` |
| valid definitive failure | `VALID/FAILED`, not waivable | `FAILED` | `REJECTED` |
| waived definitive failure | `VALID/FAILED`, waivable, waiver valid | `FAILED` | `ACCEPTED_WITH_RISK` |
| waivable failure without waiver | `VALID/FAILED`, waiver absent | `FAILED` | `REJECTED` |
| missing evidence | `MISSING` | `INDETERMINATE` | `BLOCKED` |
| stale evidence | `STALE` | `INDETERMINATE` | `BLOCKED` |
| invalid evidence | `INVALID` | `INDETERMINATE` | `BLOCKED` |
| untrusted evidence | `UNTRUSTED` | `INDETERMINATE` | `BLOCKED` |
| known failure plus missing evidence | one `VALID/FAILED`, one `MISSING` | `FAILED` | `BLOCKED` |
| errored check | `VALID/ERROR` | `INDETERMINATE` | `BLOCKED` |
| mixed passed and waived failures | established inputs only | `FAILED` | `ACCEPTED_WITH_RISK` |
| mixed waived and non-waived failures | established inputs only | `FAILED` | `REJECTED` |

Also test malformed enum input returns a non-nil error and never returns `APPROVED`.

- [ ] **Step 2: Run the focused tests and confirm RED**

Run:

```bash
go test ./internal/decision -count=1
```

Expected: compilation failure because `Evaluate` does not exist.

- [ ] **Step 3: Implement verdict calculation**

Rules:

1. Empty requirements produce `INDETERMINATE`.
2. Any established `VALID/FAILED` requirement makes the technical verdict `FAILED`.
3. Otherwise any non-`VALID` evidence state, `VALID/ERROR`, or missing outcome makes the verdict `INDETERMINATE`.
4. Otherwise the verdict is `PASSED`.

Reject invalid enum values with an error containing the requirement ID and field name.

- [ ] **Step 4: Implement completion-decision precedence**

Apply this exact order:

1. Any indeterminate requirement yields `BLOCKED`, including when the aggregate technical verdict is `FAILED` because a separate requirement has a known failure.
2. `PASSED` yields `APPROVED`.
3. `FAILED` yields `ACCEPTED_WITH_RISK` only when every failed requirement is waivable and has `WaiverValid`.
4. Remaining `FAILED` cases yield `REJECTED`.

Never mutate requirement inputs or rewrite the verdict because of a waiver.

- [ ] **Step 5: Run tests and confirm GREEN**

Run:

```bash
gofmt -w internal/decision
go test ./internal/decision -count=1
go test ./internal/... -count=1
```

Expected: all tests pass.

- [ ] **Step 6: Add deterministic repeatability test**

Call `Evaluate` 100 times with the same input and assert every result is exactly equal to the first. Run with the race detector:

```bash
go test -race ./internal/decision -run TestEvaluateIsRepeatable -count=1
```

Expected: pass with no race report.

- [ ] **Step 7: Commit**

```bash
git add internal/decision
git commit -m "feat: implement deterministic completion decisions"
```

---

### Task 4: Add public repository governance and claim-discipline documents

**Files:**
- Create: `README.md`
- Create: `LICENSE`
- Create: `SECURITY.md`
- Create: `CONTRIBUTING.md`
- Create: `AGENTS.md`
- Create: `.github/pull_request_template.md`
- Create: `docs/architecture/overview.md`
- Create: `docs/threat-model/v0.md`

**Interfaces:**
- Consumes: the foundation specification and executable terminology from Tasks 2–3.
- Produces: contributor-facing boundaries and review requirements.

- [ ] **Step 1: Write the README**

Include only currently supported claims:

- positioning and tagline;
- why agent prose and self-authored status are not authoritative;
- explicit non-guarantees: no proof of correctness, security, bug-freedom, or production readiness;
- current status: M0 foundation, not production-ready;
- repository layout;
- build and test commands;
- roadmap M0–M5;
- link to the foundation design and threat model.

Do not advertise `assurectl verify` as implemented during M0.

- [ ] **Step 2: Add Apache-2.0 and security reporting**

- Use the unmodified Apache License 2.0 text in `LICENSE`.
- `SECURITY.md` states that public GitHub issues must not contain undisclosed vulnerabilities, identifies the currently supported branch as `main`, and asks reporters to use GitHub private vulnerability reporting when enabled.
- Do not invent a security email address.

- [ ] **Step 3: Add contributor and agent rules**

`CONTRIBUTING.md` requires:

- topic branches and pull requests after the foundation commit;
- tests before implementation for behavior changes;
- `gofmt`, `go vet ./...`, `go test -race ./...`, and `go build ./cmd/assurectl` before review;
- ADRs for protocol, trust-boundary, schema, and public semantic changes;
- no new dependency without written justification.

`AGENTS.md` requires:

- read the foundation spec and relevant ADRs before editing;
- do not claim completion without fresh command output;
- do not weaken policy, tests, or security checks to make a change pass;
- keep LLM output advisory and domain decisions deterministic;
- preserve evidence/verdict/waiver/decision separation;
- never add secrets or private evidence contents;
- stop and surface ambiguity rather than making a security-sensitive assumption.

- [ ] **Step 4: Add architecture and threat-model summaries**

`docs/architecture/overview.md` documents package boundaries and data flow without duplicating the full spec.

`docs/threat-model/v0.md` includes:

- assets: decision integrity, subject binding, policy integrity, contract integrity, producer identity, evidence integrity;
- adversaries: change-producing agent, malicious contributor, compromised workflow, forged evidence producer;
- M0 protections: typed domain, deterministic decision table, fail-closed malformed input, no network fetch, no authority claim;
- deferred protections: trusted GitHub App, policy resolution from base, OIDC identity, signing, replay defense, revocation;
- explicit non-goals and residual risk.

- [ ] **Step 5: Add PR checklist**

The template includes checkboxes for scope, tests, format/vet/build, claim discipline, ADR/schema impact, trust-boundary impact, and no policy/test weakening.

- [ ] **Step 6: Verify document claims against code**

Run:

```bash
grep -R "assurectl verify" README.md docs --exclude='2026-09-02-assurectl-foundation-design.md'
grep -R "production-ready\|bug-free\|guarantees security" README.md docs SECURITY.md
```

Expected:

- any `assurectl verify` occurrence is clearly roadmap/future language;
- no document claims production readiness, bug freedom, or guaranteed security.

Then run:

```bash
go test ./...
```

Expected: pass.

- [ ] **Step 7: Commit**

```bash
git add README.md LICENSE SECURITY.md CONTRIBUTING.md AGENTS.md .github/pull_request_template.md docs/architecture docs/threat-model
git commit -m "docs: establish project governance and threat boundaries"
```

---

### Task 5: Record the locked architectural decisions as ADRs

**Files:**
- Create: `docs/adr/0001-completion-assurance-authority.md`
- Create: `docs/adr/0002-separate-verdict-waiver-and-decision.md`
- Create: `docs/adr/0003-revision-bound-subjects.md`
- Create: `docs/adr/0004-trusted-base-policy-resolution.md`
- Create: `docs/adr/0005-open-deterministic-kernel.md`
- Create: `docs/adr/0006-standard-attestation-direction.md`
- Create: `docs/adr/README.md`

**Interfaces:**
- Consumes: foundation specification sections 6–16.
- Produces: stable decision history referenced by future code and protocol work.

- [ ] **Step 1: Create the ADR template and index**

`docs/adr/README.md` defines statuses `Proposed`, `Accepted`, `Deprecated`, and `Superseded`; filenames use zero-padded sequence numbers; accepted decisions are superseded rather than silently rewritten.

- [ ] **Step 2: Write ADRs 0001–0003**

Each ADR contains Context, Decision, Consequences, Rejected Alternatives, and Status `Accepted`.

- `0001`: independent completion assurance authority, not CI runner or agent.
- `0002`: separate evidence state, observed outcome, verification verdict, finding, waiver, and completion decision.
- `0003`: repository/base/head/versioned change-set subject binding; M0 algorithm hashes the canonical repository/base/head tuple rather than textual diff output.

- [ ] **Step 3: Write ADRs 0004–0006**

- `0004`: authoritative project policy resolves from trusted base; a candidate change cannot evaluate itself under its proposed weaker policy.
- `0005`: protocol, schemas, evaluator, verifier, CLI, and reference integrations remain open; deterministic core uses no LLM verdict.
- `0006`: unsigned advisory receipts first; future signed attestations align with in-toto Statement, DSSE, and Sigstore/OIDC rather than custom cryptography.

- [ ] **Step 4: Cross-check ADR consistency**

Run:

```bash
grep -R "Status: Accepted" docs/adr/000*.md | wc -l
grep -R "Context\|Decision\|Consequences\|Rejected Alternatives" docs/adr/000*.md
```

Expected: six accepted ADRs and every ADR contains all required sections.

- [ ] **Step 5: Commit**

```bash
git add docs/adr
git commit -m "docs: record core assurance architecture decisions"
```

---

### Task 6: Add versioned protocol placeholders and self-consistent examples

**Files:**
- Create: `schemas/policy.v0.schema.json`
- Create: `schemas/verification-contract.v0.schema.json`
- Create: `schemas/evidence-envelope.v0.schema.json`
- Create: `schemas/receipt.v0.schema.json`
- Create: `spec/README.md`
- Create: `docs/protocol/v0.md`
- Create: `examples/basic/policy.json`
- Create: `examples/basic/work-unit.json`
- Create: `examples/basic/evidence/unit-tests.json`

**Interfaces:**
- Consumes: exact enum values and JSON field names from `internal/domain`.
- Produces: non-stable `v0` protocol artifacts for review; no parser implementation or stable compatibility promise.

- [ ] **Step 1: Create strict schema skeletons**

All schemas use:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/kefyusuf/assurectl/schemas/<name>.v0.schema.json",
  "type": "object",
  "additionalProperties": false
}
```

Requirements:

- schema identifiers remain repository-controlled;
- enum strings exactly match Task 2;
- digests are represented as algorithm plus value, not an ambiguous free-form hash where the model requires algorithm agility;
- receipt schema includes `authority`, `verification_verdict`, and `completion_decision` as separate fields;
- waiver arrays do not alter evidence result fields;
- descriptions label the schema as unstable v0.

- [ ] **Step 2: Add self-consistent basic examples**

Use placeholder full-length hexadecimal Git object IDs and SHA-256 digests that satisfy schema patterns. The example represents:

- one `commit` work unit;
- one required `unit-tests` verification;
- local advisory authority;
- one evidence envelope with state-independent observed outcome `PASSED`;
- no claim that the evidence is merge-grade.

- [ ] **Step 3: Document protocol status**

`spec/README.md` states:

- v0 is unstable and not a compatibility commitment;
- CLI and protocol versions evolve independently;
- changes require schema fixtures and migration notes once consumers exist;
- signed attestation format is deferred.

`docs/protocol/v0.md` maps domain concepts to schema files and repeats the claim boundary.

- [ ] **Step 4: Add schema syntax tests using the standard library**

Create `schemas/schema_syntax_test.go` in package `schemas_test`. The test:

- locates all `*.schema.json` files;
- parses each with `encoding/json`;
- fails on duplicate-free valid JSON syntax assumptions only;
- asserts each root has `$schema`, `$id`, `type: object`, and `additionalProperties: false`.

Create `examples/basic/examples_syntax_test.go` that parses every JSON example and asserts its `schema_version` is non-empty. These tests do not pretend to be full JSON Schema validation.

- [ ] **Step 5: Run protocol artifact verification**

Run:

```bash
go test ./schemas ./examples/basic -count=1
go test ./... -count=1
```

Expected: all JSON files parse and all tests pass.

- [ ] **Step 6: Commit**

```bash
git add schemas spec docs/protocol examples/basic
git commit -m "docs: add v0 protocol schemas and examples"
```

---

### Task 7: Perform the M0 release-candidate verification and open the PR

**Files:**
- Modify only files required to fix verification failures.
- No scope expansion.

**Interfaces:**
- Consumes: all M0 outputs.
- Produces: one reviewable pull request from `feat/m0-foundation` to `main`.

- [ ] **Step 1: Run formatting and generated-artifact checks**

```bash
gofmt -w cmd internal schemas examples
test -z "$(gofmt -l cmd internal schemas examples)"
```

Expected: exit `0` and no listed files.

- [ ] **Step 2: Run full static and dynamic verification**

```bash
go vet ./...
go test -race -coverprofile=coverage.out ./...
go build ./cmd/assurectl
```

Expected: every command exits `0`; no race report.

- [ ] **Step 3: Exercise the CLI shell**

```bash
go run ./cmd/assurectl --help
go run ./cmd/assurectl version
set +e
go run ./cmd/assurectl unknown >/tmp/assurectl.stdout 2>/tmp/assurectl.stderr
status=$?
set -e
test "$status" -ne 0
test ! -s /tmp/assurectl.stdout
grep -q "unknown command: unknown" /tmp/assurectl.stderr
```

Expected: help and version succeed; unknown command produces only stderr and a non-zero process status. Note that `go run` wraps the program's exit status, so unit tests remain the authoritative assertion for exact exit code `64`.

- [ ] **Step 4: Review the branch diff against the spec**

Checklist:

- no `assurectl verify` implementation was smuggled into M0;
- no third-party dependency exists;
- no authoritative or cryptographic claim exists;
- no policy execution, command runner, network fetch, or LLM verdict exists;
- every enum and decision transition is tested;
- documentation matches current behavior;
- all new public terminology has an ADR or protocol entry.

- [ ] **Step 5: Commit verification-only fixes, when required**

```bash
git add -A
git commit -m "test: close M0 verification gaps"
```

Skip this commit when the verification run requires no changes.

- [ ] **Step 6: Open the pull request**

Title:

```text
feat: establish AssureCTL M0 foundation
```

Body must include:

- product boundary and non-goals;
- domain primitives and decision precedence implemented;
- governance, ADR, schema, and example artifacts added;
- exact commands run and their results;
- known limitation: M0 has no local `verify` evaluator, Git subject resolver, evidence loader, or signed receipt;
- next milestone: M1 local advisory vertical slice.
