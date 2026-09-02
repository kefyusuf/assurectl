# M0 Execution Notes

These notes record implementation-time refinements to the M0 plan.

- CI actions use immutable SHAs for `actions/checkout` v7.0.1 and `actions/setup-go` v7.0.0.
- Go dependency caching is disabled while the module has no `go.sum`; this avoids a cache setup failure in the zero-dependency M0 module.
- Formatting checks discover `*.go` files explicitly instead of passing directories to `gofmt`.
- ADR-0007 removes self-declared authority from policy and verification-contract payloads. Trust is resolved externally and recorded by the verifier.
- Receipt v0 separates actual evidence references from requirement evaluation results so `MISSING` evidence does not require a fabricated digest or outcome.
- Local verification used the available Go 1.23.2 toolchain with a temporary local `go` directive override. GitHub Actions is the authoritative compatibility check for the committed Go 1.26.0 floor and the 1.26.x/1.27.x matrix.
- CI disables persisted checkout credentials and sets `GOTOOLCHAIN=local` to prevent unintended toolchain acquisition during verification.
- Decision validation fails closed when a normalized result claims a valid waiver for a non-waivable requirement.
- Receipt policy provenance records every contributing layer as a separate source with digest, resolved trust status, authority basis, and source revision when applicable; the effective policy has its own digest.
- Contributor-facing build commands write the CLI to `/tmp/assurectl`, and `/assurectl` is ignored defensively so verification does not dirty the repository.
- Zero-argument CLI commands reject trailing arguments with usage exit code `64`.
- Receipt schema conditionals constrain verdict/decision combinations to the deterministic decision kernel.
- Evidence timestamp schemas add lexical RFC3339 assertions; semantic parsing and ordering remain an explicit M1 ingestion requirement.
- The original M0 plan is marked superseded because accepted implementation refinements made its literal action pins, cache settings, ADR count, and build commands stale.
