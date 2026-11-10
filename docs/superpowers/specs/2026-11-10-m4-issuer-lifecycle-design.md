# M4 Issuer Lifecycle Design — Signed Root Rotation and Revocation

**Status:** Implemented in M4 issuer-lifecycle slice  
**Date:** 2026-11-10  
**Milestone:** M4 — Signing, replay protection, issuer lifecycle (slice 2 of the milestone)  
**Base commit:** `23eb7c8d3082b7eef07932710fa01f1c2668c605`  
**Scope:** signed trusted-root documents, root key rotation, issuer/key revocation, and version monotonicity — a self-contained, offline-tested extension of the attestation package; no hosting, no transparency log, no network

## 1. Purpose

M4 slice 1 left the trusted root as un-signed caller configuration: "who distributes it and how it rotates is the issuer-lifecycle slice's subject." This slice closes exactly that gap. The problem it solves is the classic trust-anchor rotation problem: a verifier pins one root key out-of-band (initial trust), and every later change to the set of accepted issuers and keys must be **authorized by the keys the verifier already trusted**, versioned so old roots cannot be replayed, and fail-closed when the chain is broken.

## 2. Design: a minimal, TUF-shaped signed root

Full TUF is deliberately out of scope (metadata file graph, delegate roles, snapshot timestamps). The protocol here is the minimal subset that keeps the fail-closed guarantees:

```json
SignedRoot (v1):
  "root_version": 2
  "issuers": [ ...same TrustedIssuer shape as slice 1... ]
  "signatures": [ { "keyid": "<root key id>", "sig": "<base64 over PAE>" } ]
```

- The root **payload** is the slice-1 trusted-root document exactly (`root_version`, `issuers` with validity windows) — PAE covers it byte-exact, so the slice-1 parsing and validity-window logic is reused unchanged.
- **Initial trust is a pin.** The verifier is configured with the *initial* root key id out-of-band (one string, like the reference action pins a commit SHA). A signed root is accepted only if at least one valid signature comes from a key the verifier already trusts.
- **Version monotonicity.** The verifier tracks the highest root version it has accepted (in process, plus an optional `--min-root-version` flag for callers persisting state). A signed root with version ≤ the accepted version is rejected — old roots (including ones with still-valid signatures) cannot roll back revocations.
- **Rotation needs old + new.** A root version that *changes* the set of root keys must carry valid signatures from **both** a key trusted in the currently-accepted version and a key present in the new version (the standard "step" transition). A root signed only by a brand-new key is rejected: new keys cannot introduce themselves.
- **Revocation is a newer version.** Dropping an issuer or key from the issuers list, signed per the rules above, revokes it; receipts signed by a revoked key after the revocation root's acceptance fail verification (the root resolution runs against the newest accepted root).
- **Validity windows** (slice 1's per-issuer `valid.from/until`) continue to apply unchanged at the evaluation instant.

## 3. Commands and integration

- `assurectl sign-root --key <PEM> --key-id <id> --in <root-payload.json> --out <signed-root.json>`: signs a root payload with a root key (multiple signatures are produced by running the command per key and merging — the envelope format already carries a signature list).
- `assurectl verify-receipt` gains `--trusted-root-signed <file> --root-keyid <pinned-id>` as an alternative to `--trusted-root <unsigned file>`: parses the signed root, applies the acceptance rules (pin, monotonicity, transition, windows), then verifies receipts exactly as before. Both flags together are a usage error (exactly one root source).
- A `--min-root-version <n>` flag feeds the monotonicity check for callers that persist state.

## 4. Non-scope

- Transparency logs, timestamping, delegation roles (TUF ecosystem features beyond the minimal subset);
- online revocation (CRL/OCSP-style) — revocation here is versioned-root-based, deliberate and offline;
- changes to receipt construction, evaluation, the App core, or existing schemas — the root payload is the slice-1 document shape;
- new dependencies; all crypto stays standard-library Ed25519/SHA-256.

## 5. Testing strategy

Test-first, fully offline: signed-root parse/verify table (valid, bad signature, version rollback, new-key-only self-introduction, missing old-key signature on rotation, revoked key rejected for receipts, validity-window interplay, multiple-signature merge); `verify-receipt` integration (signed root accepted, unsigned root still accepted, both root flags rejected, monotonicity via `--min-root-version`); determinism of signatures over identical payloads; conformance fixtures under `examples/signed/` extended with a rotation chain (v1 → v2 rotation → v3 revocation) regenerable with `FIXTURE_UPDATE=1`. Both-toolchain gofmt before push.

## 6. Boundary to the next slices

M4 slice 3: conformance-suite expansion (downstream implementer vectors for the whole signing surface) and retention metadata. Sigstore keyless and transparency remain managed-scope later work; nothing in this root protocol changes when they arrive.
