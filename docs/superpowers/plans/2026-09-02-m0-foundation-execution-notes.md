# M0 Execution Notes

These notes record implementation-time refinements to the M0 plan.

- CI actions use immutable SHAs for `actions/checkout` v7.0.1 and `actions/setup-go` v7.0.0.
- Go dependency caching is disabled while the module has no `go.sum`; this avoids a cache setup failure in the zero-dependency M0 module.
- Formatting checks discover `*.go` files explicitly instead of passing directories to `gofmt`.
- ADR-0007 removes self-declared authority from policy and verification-contract payloads. Trust is resolved externally and recorded by the verifier.
- Receipt v0 separates actual evidence references from requirement evaluation results so `MISSING` evidence does not require a fabricated digest or outcome.
- Local verification used the available Go 1.23.2 toolchain with a temporary local `go` directive override. GitHub Actions is the authoritative compatibility check for the committed Go 1.26.0 floor and the 1.26.x/1.27.x matrix.
- CI disables persisted checkout credentials and sets `GOTOOLCHAIN=local` to prevent unintended toolchain acquisition during verification.
- Decision validation fails closed when a normalized result claims a valid waiver for any state other than a waivable `VALID/FAILED` requirement.
- The decision kernel now accepts normalized findings as a variadic input. A blocking finding yields `INDETERMINATE/BLOCKED` when requirements otherwise pass and preserves `FAILED/BLOCKED` when an established failure also exists; non-blocking findings do not change the result.
- Finding input validation rejects empty codes/messages and unknown categories/severities before decision evaluation.
- Receipt policy provenance records every contributing layer as a separate source with digest, resolved trust status, authority basis, and revision binding when applicable; the effective policy has its own digest.
- Trusted-base policy sources use `revision_binding: SUBJECT_BASE` and cannot carry a free `source_revision`; this prevents a receipt from naming a trusted-base source at the candidate head or an unrelated revision while claiming subject-base authority.
- A non-blocked authoritative policy requires exactly one trusted `protocol_baseline` source with `authority_basis: BUILTIN`.
- Contributor-facing build commands write the CLI to `/tmp/assurectl`, and `/assurectl` is ignored defensively so verification does not dirty the repository.
- Zero-argument CLI commands reject trailing arguments with usage exit code `64`.
- Receipt schema conditionals mirror the deterministic decision kernel, including `FAILED/BLOCKED` for mixed known-failure and indeterminate or blocking-finding states.
- Technical verdicts are derived from requirement results and blocking findings: all `VALID/PASSED` with no blocker for `PASSED`; at least one `VALID/FAILED` for `FAILED`; otherwise at least one unavailable, errored, or blocking-finding condition for `INDETERMINATE`.
- Receipt requirement identifiers use the same canonical identifier pattern as verification contracts and reject whitespace-only values.
- An `APPROVED` receipt requires every requirement result to be established as `VALID/PASSED`.
- `REJECTED` requires only established `VALID` results and at least one failed requirement that is non-waivable or lacks a valid waiver; indeterminate results and blocking findings cannot be represented as rejection.
- `ACCEPTED_WITH_RISK` requires only established `VALID` results, at least one failed result, a valid waiver status for every failed result, and at least one supplied waiver record resolved as valid.
- `BLOCKED` requires at least one indeterminate requirement result or a finding marked `blocking: true`; an established failure alone must resolve to rejection or accepted risk.
- Exact failed-requirement-to-waiver-record referential matching remains a semantic receipt-builder invariant because JSON Schema cannot compare identifiers across independent arrays.
- Any receipt decision other than `BLOCKED` requires all findings to be non-blocking.
- A non-`BLOCKED` `AUTHORITATIVE` receipt requires trusted contract and effective-policy inputs.
- Every source contributing to a non-blocked authoritative policy must itself be trusted and use an authority basis compatible with its layer: built-in protocol baseline, protected organization control plane, trusted-base project policy, trusted-base or protected assurance profile, and approved contract source.
- `ADVISORY_WORKSPACE`, `CANDIDATE_HEAD`, unresolved, untrusted, missing-baseline, and layer-incompatible policy sources cannot contribute to a non-blocked authoritative receipt.
- A receipt requires at least one requirement result, preventing an empty evaluation from being represented as approval.
- Findings preserve a closed machine-readable category in addition to code and severity.
- Receipt waiver records preserve exact target, scope, accepted risk, approver identity and resolved authority, issue/expiry times, validity status, and a normalized-input digest.
- Evidence and receipt timestamp schemas add lexical RFC3339 assertions; semantic parsing and ordering remain explicit M1 ingestion requirements.
- The original M0 plan is marked superseded because accepted implementation refinements made its literal action pins, cache settings, ADR count, and build commands stale.
