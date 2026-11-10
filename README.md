# AssureCTL

> Agents propose changes. AssureCTL decides what the evidence permits.

AssureCTL is an open, deterministic, revision-bound assurance authority for software changes. It is being designed to evaluate whether a specific change may move to a completion state under an explicit verification contract, trusted policy, and machine-verifiable evidence.

## Status

**M0 through M5 of the foundation roadmap are complete.** The current codebase contains:

- a deterministic evaluator producing receipts from policy, contract, and machine-verifiable evidence;
- `assurectl verify` for local advisory, CI-asserted, and authoritative (trusted-base) evaluation;
- Git subject resolution with fail-closed ambiguity rejection;
- evidence ingestion with artifact digest verification, stale/untrusted/missing detection, and set-wide fail-closed semantics;
- a waiver engine with explicit opt-in inputs (ADR-0008);
- signed receipts as in-toto statements over DSSE with Ed25519 and trusted-root rotation/revocation;
- a reference GitHub Action and a self-hostable App (`assurectl-app`) posting merge-gating checks from outside the repository;
- `assurectl init` for migration producer scaffolding;
- a dogfooding gate: this repository's own PRs are verified blocking under the base's gate.

Not production-ready. Sigstore keyless signing, transparency logs, retention storage, and organization policy control planes are managed-scope items not yet implemented.

## Why this exists

A coding agent can write code, run commands, produce a status file, and say that the work is complete. Those actions do not independently establish that:

- the evidence belongs to the exact revision proposed for merge or release;
- required checks ran under an allowed producer identity;
- the policy was not weakened by the same change it evaluates;
- missing, stale, invalid, or untrusted evidence was handled correctly;
- a waiver came from an authorized risk owner;
- the final decision was issued independently of the worker.

AssureCTL keeps those concerns separate and evaluates them deterministically.

```text
software change
      |
      v
verification contract ---- trusted policy
      |                         |
      +------------+------------+
                   v
             evidence inputs
                   |
                   v
          deterministic evaluator
                   |
        +----------+----------+
        v                     v
verification verdict   completion decision
                   |
                   v
        advisory or authoritative receipt
```

## Claim boundary

AssureCTL does **not** claim that evaluated software is correct, secure, bug-free, complete in every possible sense, or production-ready. A future receipt will state only that an identified subject was evaluated under an identified contract and policy using identified evidence.

## Build and test

Prerequisite: Go 1.26 or newer.

```bash
go test ./...
go vet ./...
go build -trimpath -o /tmp/assurectl ./cmd/assurectl
go run ./cmd/assurectl version
```

## Repository map

```text
cmd/assurectl/          CLI entry point
internal/domain/        closed domain vocabulary
internal/decision/      deterministic decision precedence
internal/buildinfo/     build metadata
schemas/                unstable v0 JSON Schema drafts
examples/               protocol examples
spec/                   protocol status and compatibility notes
docs/adr/               architectural decision records
docs/architecture/      concise component boundaries
docs/threat-model/      supported and deferred protections
```

The consolidated design is in [`docs/superpowers/specs/2026-09-02-assurectl-foundation-design.md`](docs/superpowers/specs/2026-09-02-assurectl-foundation-design.md). Accepted ADRs take precedence over conflicting details; historical amendments are retained for traceability.

## Roadmap

| Milestone | Scope | Status |
|---|---|---|
| M0 | Foundation, domain semantics, decision table, governance, schemas, CI | ✅ |
| M1 | Local advisory evaluation for exact Git revisions | ✅ |
| M2 | Trusted CI evidence envelopes, waiver engine, and reusable GitHub Action | ✅ |
| M3 | Trusted-base resolution, GitHub App required-check, merge authority, OSS app skeleton and adapters | ✅ |
| M4 | In-toto/DSSE signed attestations with Ed25519, trusted-root rotation and revocation | ✅ |
| M5 | Migration producer integration (`assurectl init`, examples, guide) | ✅ |

## Contributing

Read [`CONTRIBUTING.md`](CONTRIBUTING.md), [`AGENTS.md`](AGENTS.md), the relevant ADRs, and the threat model before changing protocol or trust-sensitive behavior.

## License

Apache License 2.0. See [`LICENSE`](LICENSE).
