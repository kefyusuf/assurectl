# AssureCTL

> Agents propose changes. AssureCTL decides what the evidence permits.

AssureCTL is an open, deterministic, revision-bound assurance authority for software changes. It is being designed to evaluate whether a specific change may move to a completion state under an explicit verification contract, trusted policy, and machine-verifiable evidence.

## Status

AssureCTL is in **M0 foundation development** and is not production-ready. The current codebase contains:

- a buildable Go CLI shell with help and version output;
- typed domain values for authority, evidence state, observed outcome, verdict, waiver status, and completion decision;
- an executable, deterministic decision-precedence table;
- initial architectural decisions, threat boundaries, and unstable v0 schema drafts.

The local `verify` workflow, Git subject resolution, evidence loading, policy parsing, receipt generation, GitHub merge authority, and signed attestations belong to later milestones.

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
go build ./cmd/assurectl
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

The full design is in [`docs/superpowers/specs/2026-09-02-assurectl-foundation-design.md`](docs/superpowers/specs/2026-09-02-assurectl-foundation-design.md).

## Roadmap

| Milestone | Scope |
|---|---|
| M0 | Foundation, domain semantics, decision table, governance, schemas, CI |
| M1 | Local advisory evaluation for exact Git revisions |
| M2 | Trusted CI evidence envelopes and reusable GitHub Action |
| M3 | GitHub App required-check and merge authority |
| M4 | in-toto/DSSE/Sigstore-aligned signed attestations |
| M5 | Self-maintaining API, SDK, framework, and dependency migration integrations |

## Contributing

Read [`CONTRIBUTING.md`](CONTRIBUTING.md), [`AGENTS.md`](AGENTS.md), the relevant ADRs, and the threat model before changing protocol or trust-sensitive behavior.

## License

Apache License 2.0. See [`LICENSE`](LICENSE).
