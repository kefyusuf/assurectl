# M4 Signed Receipts Design — In-toto Statements over DSSE with Ed25519

**Status:** Draft for review  
**Date:** 2026-11-07  
**Milestone:** M4 — Signing, replay protection, issuer lifecycle (slice 1 of the milestone)  
**Base commit:** `f30d3e5300975d62f68c420e0a35d742c38a0384`  
**Scope:** wrapping the v0 receipt as an in-toto statement, DSSE envelope production and offline verification with Ed25519 keys from a versioned trusted-root document — no Sigstore keyless, no transparency log, no issuer service in this slice

## 1. Purpose

M0's direction was explicit: "future signed receipts use an in-toto Statement with an AssureCTL-specific predicate, wrapped in DSSE and signed through a standard identity mechanism such as Sigstore/OIDC. The project does not define a custom signature primitive" (foundation §12). M4 slice 1 builds exactly that container layer: the receipt v0 becomes the predicate of an in-toto Statement (`https://assurectl.dev/attestations/receipt/v0`), the statement is wrapped and signed in a standard DSSE envelope with an Ed25519 key, and a new `assurectl verify-receipt` command verifies signed receipts offline against a versioned trusted-root document. Formats align with in-toto/DSSE so a Sigstore keyless signer slots in later without protocol change.

## 2. Why Ed25519 and why not Sigstore yet

- `crypto/ed25519` is in the standard library: no dependency, no custom primitive (ADR-0006's rule is against *custom cryptography*, not against standard signatures);
- Sigstore keyless signing needs OIDC, Fulcio certificates, and Rekor transparency-log access — network services that make the *core evaluator* network-dependent, exactly what the architecture forbids; they belong to the managed/hosted scope and later slices, and nothing in the container changes when they arrive;
- the trust anchor for this slice is a **trusted-root document** supplied by the caller (like `gitsubject`'s key allowlist and the receipt's provenance model): a signed receipt is worth exactly the root the verifier was configured with.

## 3. Statement and envelope

```json
Statement (in-toto v1):
  "_type": "https://in-toto.io/Statement/v1"
  "subject": [{ "name": "<canonical repository uri>", "digest": { "gitChangeSet": "<change-set digest>" } }]
  "predicateType": "https://assurectl.dev/attestations/receipt/v0"
  "predicate": { ...the complete receipt.v0 document, unchanged... }

DSSE envelope (v1):
  "payloadType": "https://assurectl.dev/attestations/receipt/v0"
  "payload": base64(UTF-8 JSON of the statement)
  "signatures": [{ "keyid": "<trusted-root key id>", "sig": base64(DSSE-PAE signature) }]
```

- Signing covers the DSSE PAE (`DSSEv1 <len(payloadType)> <payloadType> <len(payload)> <payload>`) — the standard pre-authentication encoding, byte-for-byte as specified by the DSSE spec; no custom canonicalization.
- The subject binds the statement to the exact change set, so a signed receipt is replay-proof *for a different change* by construction; `subject` digests use the in-toto gitTransport naming adapted to AssureCTL's change-set digest algorithm identifier.

## 4. Trusted root (v0)

A caller-supplied JSON document listing acceptable issuers and keys:

```json
{ "root_version": 1,
  "issuers": [{ "id": "<issuer id>", "keys": [{ "keyid": "<sha256 of public key>", "ed25519_public": "<base64>" }], "valid": {"from": "<RFC3339>", "until": "<RFC3339>"} }] }
```

- Key ids are the SHA-256 of the raw Ed25519 public key bytes (deterministic, no registry);
- verification: exactly one valid issuer key matching the envelope's `keyid` at the evaluation instant; expired/not-yet-valid issuers are rejected fail-closed;
- the trusted root itself is un-signed configuration in this slice — who distributes it and how it rotates is the issuer-lifecycle slice's subject.

## 5. Commands

- `assurectl sign-receipt --key <ed25519 PEM> --key-id <id> --in <receipt.json> --out <attestation.json>`: loads a receipt document, wraps and signs it (the signing side is an adapter; the App/CLI may use it).
- `assurectl verify-receipt --trusted-root <root.json> --in <attestation.json>`: verifies the DSSE signature against the trusted root, re-checks the predicate schema (receipt v0 semantics — the M1 builder checks re-run), and prints the embedded receipt; exit codes follow the decision mapping. Both commands fail closed; output on stdout only on success.

## 6. Replay protection in this slice

A signed receipt is bound to its subject's change-set digest; a verifier comparing the statement's subject digest against the change being reasoned about cannot replay an old verdict onto a new change. Time-based replay (an old receipt for the *same* unchanged change) is bounded by the receipt's `evaluated_at` and the verifier's own policy check — the full freshness window mechanism is the issuer-lifecycle slice, and this slice's verify-receipt accepts an optional `--max-age <duration>` check to make that explicit and testable now.

## 7. Non-scope

- Sigstore keyless (Fulcio/Rekor), transparency, issuer services, retention storage (managed scope / later slices);
- any change to receipt construction, evaluation, ingestion, or existing schemas — the receipt inside the predicate is byte-what-M1-built;
- new dependencies: DSSE, in-toto, and Ed25519 are implemented with the standard library only.

## 8. Testing strategy

Test-first, fully offline: PAE encoding vectors (byte-exact against the DSSE specification's worked example); statement shape fixtures; sign→verify round-trips; tamper table (each of payload, payloadType, signature mutated → rejected); trusted-root table (unknown keyid, expired issuer, not-yet-valid, multiple matching keys → fail closed); replay check (subject digest mismatch, `--max-age` exceeded); conformance fixtures committed under `examples/signed/` for downstream implementers. Both-toolchain gofmt before push.

## 9. Boundary to the next M4 slices

Slice 2: issuer lifecycle (signed trusted-root rotation, revocation lists, issuer identity for the hosted App). Slice 3: conformance suite for downstream implementers + retention metadata. Sigstore keyless as a signer variant slots into the same envelope with a managed issuer.
