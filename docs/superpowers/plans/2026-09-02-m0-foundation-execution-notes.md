# M0 Execution Notes

These notes record implementation-time refinements to the M0 plan.

## Implemented M0 behavior

- CI actions use immutable SHAs for `actions/checkout` v7.0.1 and `actions/setup-go` v7.0.0.
- Go dependency caching is disabled while the module has no `go.sum`; this avoids a cache setup failure in the zero-dependency M0 module.
- Formatting checks discover `*.go` files explicitly instead of passing directories to `gofmt`.
- ADR-0007 removes self-declared authority from policy and verification-contract payloads. Trust is resolved externally rather than established by those payloads.
- Local verification used the available Go 1.23.2 toolchain with a temporary local `go` directive override. GitHub Actions is the compatibility check for the committed Go 1.26.0 floor and the 1.26.x/1.27.x matrix.
- CI disables persisted checkout credentials and sets `GOTOOLCHAIN=local` to prevent unintended toolchain acquisition during verification.
- Decision validation fails closed when a normalized result claims a valid waiver for any state other than a waivable `VALID/FAILED` requirement.
- The decision kernel accepts normalized findings as a variadic input. A blocking finding yields `INDETERMINATE/BLOCKED` when requirements otherwise pass and preserves `FAILED/BLOCKED` when an established failure also exists; non-blocking findings do not change the result.
- Finding input validation rejects empty codes/messages and unknown categories/severities before decision evaluation.
- Optional finding requirement/evidence references use the shared canonical identifier grammar; duplicate evidence references within one finding fail closed.
- Normalized requirement results are keyed by canonical `requirement_id`; duplicate requirement identifiers within a single evaluation fail closed before verdict derivation.
- `VALID` normalized requirement results carry one or more canonical, unique `evidence_ids`; the decision kernel rejects a `VALID` result without evidence references and rejects malformed or duplicate evidence identifiers.
- Contributor-facing build commands write the CLI to `/tmp/assurectl`, and `/assurectl` is ignored defensively so verification does not dirty the repository.
- Zero-argument CLI commands reject trailing arguments with usage exit code `64`.
- Technical verdicts are derived from normalized requirement results and blocking findings: all `VALID/PASSED` with no blocker for `PASSED`; at least one `VALID/FAILED` for `FAILED`; otherwise at least one unavailable, errored, or blocking-finding condition for `INDETERMINATE`.
- Findings preserve a closed machine-readable category in addition to code and severity.

## Non-enforcing M0 receipt model

M0 emits no receipts and has no receipt-construction or receipt-ingestion acceptance path. The receipt bullets below describe **draft structural schema constraints and protocol shape only**. They are not implemented receipt protections, do not make generic JSON Schema validation an AssureCTL acceptance decision, and do not establish authoritative receipt trust. M1 receipt ingestion/construction must implement the corresponding semantic checks fail-closed before any receipt is accepted or emitted.

- Receipt v0 structurally separates supplied evidence references from requirement evaluation results so `MISSING` evidence does not require a fabricated digest or outcome.
- The draft receipt policy model records contributing layers as separate sources with digest, resolved trust status, authority basis, and revision binding when applicable; the effective policy has its own digest. M1+ must semantically validate those externally resolved provenance claims before relying on them.
- The draft model represents trusted-base policy sources with `revision_binding: SUBJECT_BASE` and without a free `source_revision`. M1+ must resolve that marker to the receipt subject's exact base object before semantic acceptance.
- The draft model represents a non-blocked authoritative policy as containing exactly one trusted `protocol_baseline` source with `authority_basis: BUILTIN`. M1+ must enforce baseline presence and source trust semantically.
- Draft receipt-schema conditionals mirror the deterministic decision kernel's intended output pairs, including `FAILED/BLOCKED` for mixed known-failure and indeterminate or blocking-finding states.
- Draft receipt requirement identifiers use the same canonical identifier pattern as verification contracts and reject whitespace-only values.
- Draft receipt evidence-reference IDs use the evidence-envelope canonical identifier grammar. If any requirement result is `VALID`, the schema structurally requires a non-empty top-level `evidence` set.
- JSON Schema Draft 2020-12 cannot enforce uniqueness of `requirement_results[].requirement_id` by object property. M1 semantic receipt validation must reject duplicate requirement IDs for **every** receipt, including `BLOCKED`, before acceptance or emission.
- Exact `requirement_result.evidence_ids[]` to top-level `evidence[].id` membership and digest matching is a fail-closed M1 receipt-builder/ingestion invariant because JSON Schema cannot compare identifiers across independent arrays.
- The draft model represents `APPROVED` only when every requirement result is established as `VALID/PASSED`; M1 semantic validation must derive and verify the same invariant rather than trusting the supplied decision field.
- The draft model represents `REJECTED` using only established `VALID` results and at least one failed requirement that is non-waivable or lacks a valid waiver; M1 semantic validation must preserve precedence for indeterminate results and blocking findings.
- The draft model represents `ACCEPTED_WITH_RISK` using only established `VALID` results, at least one failed result, a valid waiver status for every failed result, and at least one supplied waiver record resolved as valid. M1 must semantically validate waiver authority, scope, timing, and exact target matching.
- The draft model represents `BLOCKED` with at least one indeterminate requirement result or a finding marked `blocking: true`; M1 semantic validation must ensure an established failure alone does not select `BLOCKED`.
- Exact failed-requirement-to-waiver-record referential matching remains a semantic M1 receipt invariant because JSON Schema cannot compare identifiers across independent arrays.
- The draft model requires any receipt decision other than `BLOCKED` to contain only non-blocking findings; M1 semantic validation must enforce that relationship.
- The draft model represents non-`BLOCKED` `AUTHORITATIVE` receipts with trusted contract and effective-policy inputs. M0 does not establish or enforce that receipt authority.
- The draft model constrains sources contributing to a non-blocked authoritative policy to trusted, layer-compatible authority bases: built-in protocol baseline, protected organization control plane, trusted-base project policy, trusted-base or protected assurance profile, and approved contract source. M1+ must semantically resolve and validate those authorities.
- `ADVISORY_WORKSPACE`, `CANDIDATE_HEAD`, unresolved, untrusted, missing-baseline, and layer-incompatible policy provenance are modeled as ineligible for a non-blocked authoritative receipt; semantic enforcement is deferred to M1+.
- The draft receipt schema requires at least one requirement result, structurally preventing an empty evaluation from being represented as approval.
- Draft receipt waiver records preserve exact target, scope, accepted risk, approver identity and resolved authority, issue/expiry times, validity status, and a normalized-input digest. These fields are audit structure, not self-authenticating trust.
- Evidence and receipt timestamp schemas add lexical RFC3339 assertions; semantic parsing and ordering remain explicit M1 ingestion requirements.

## Verification history

- The evidence-binding TDD sequence used tests-only commits `e5ae469` and `185077c`, implementation commit `853ae4a`, and fixture-only correction `df95915`. Exact-head CI run `34151329570` passed formatting, vet, race tests, and CLI build on Go 1.26.x and Go 1.27.x.
- The normalized-identifier uniqueness TDD sequence used tests-only commit `0b4fc28`; CI run `34152522027` failed only on duplicate requirement IDs and malformed/duplicate finding references. Implementation commit `b132a06` closed those gaps, and CI run `34152752138` passed the full Go 1.26.x/1.27.x matrix before documentation-only follow-up.
- The original M0 plan is marked superseded because accepted implementation refinements made its literal action pins, cache settings, ADR count, and build commands stale.
