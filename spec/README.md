# AssureCTL Protocol Artifacts

The files in `schemas/` and `examples/` are **unstable v0 drafts**. They make domain discussions concrete but are not yet a compatibility commitment.

## Versioning rules

- CLI versions and protocol/schema versions evolve independently.
- Schema identifiers are repository-controlled until a project domain is formally acquired and governed.
- A stable protocol version requires conformance fixtures, migration notes, canonicalization rules, and an explicit compatibility policy.
- Breaking changes after external consumers exist require a new schema identifier.
- Signed attestations are deferred; v0 examples carry no cryptographic or authoritative claim.

## Current artifacts

- `schemas/policy.v0.schema.json`
- `schemas/verification-contract.v0.schema.json`
- `schemas/evidence-envelope.v0.schema.json`
- `schemas/receipt.v0.schema.json`
- `examples/basic/`

The syntax tests confirm valid JSON and required root metadata. They are not a complete JSON Schema implementation and must not be described as full schema validation.
