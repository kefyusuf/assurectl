# Migration Producer Integration Guide

This guide shows how automated migration tools — dependency upgraders, API migration codemods, framework upgraders, security remediation bots — integrate AssureCTL as their verification gate. It claims nothing the code does not implement.

## 1. The pattern

A migration tool proposes changes and produces evidence. AssureCTL evaluates that evidence against the policy from the base revision and emits a receipt. The receipt gates the migration's merge.

Three steps:

1. **Scaffold** the `.assurectl/` documents for your work-unit type (`assurectl init`);
2. **Produce evidence** — run your verification and write an evidence envelope with the result;
3. **Verify** — run `assurectl verify` to evaluate and emit a receipt.

## 2. Scaffold with `assurectl init`

```bash
assurectl init \
  --type dependency-upgrade \
  --requirement-id dep-upgrade \
  --evidence-type dependency-result \
  --output-dir .
```

This creates `.assurectl/contract.json` and `.assurectl/policy.json` with minimal, valid documents. Edit them for your needs — they are starting points, not authoritative. AssureCTL refuses to overwrite existing files.

## 3. Produce evidence

After your migration tool runs its verification, write an evidence envelope:

```json
{
  "schema_version": "assurectl/evidence-envelope/v0",
  "id": "ev-dep-upgrade-001",
  "type": "dependency-result",
  "subject": {
    "repository_uri": "github.com/acme/checkout",
    "revision": "<the exact head revision>"
  },
  "producer": { "type": "local-user", "identity": "renovate-bot" },
  "invocation": { "command_id": "dep.upgrade.verify", "started_at": "...", "finished_at": "..." },
  "outcome": { "status": "PASSED", "exit_code": 0 },
  "artifact": { "uri": "artifacts/upgrade-report.txt", "digest": { "algorithm": "sha256", "value": "<artifact sha256>" }, "media_type": "text/plain" }
}
```

Key rules:
- `type` must match the policy's `evidence_type`;
- `subject.revision` must be the exact head revision the evidence was produced on (anything else is `STALE`);
- `producer.type` must be in the policy's `allowed_producer_types`;
- the artifact digest must match the uploaded bytes (the evaluator re-hashes every artifact).

## 4. Verify

```bash
assurectl verify --base <base-ref> --head <head-ref>
```

Or in CI with the reference action and OIDC-asserted context:

```bash
assurectl verify --base <base-ref> --head <head-ref> --trusted-base --ci
```

The receipt's `completion_decision` is `APPROVED` when all requirements pass, `REJECTED` when any fails (waivable failures may become `ACCEPTED_WITH_RISK` with `--waivers`), and `BLOCKED` when evidence is missing, stale, untrusted, or inconsistent. Exit codes: 0 (approved/accepted-with-risk), 1 (rejected), 2 (blocked/error), 64 (usage).

## 5. Work-unit types

The closed vocabulary includes types for common migration scenarios: `dependency_upgrade`, `api_migration`, `sdk_migration`, `framework_upgrade`, `security_remediation`. Each type is just a label — the evaluation logic is identical. Choose the one that matches your migration scenario.

## 6. Trust tiers

- **Advisory** (no flags): local feedback, not merge-grade;
- **CI_ASSERTED** (`--ci`): a GitHub Actions runner asserted the context; subject cross-checks enforced;
- **AUTHORITATIVE** (`--trusted-base --ci`): the gate documents came from the base revision; the receipt's provenance markers resolve to the subject's base.

See `docs/guides/protected-workflows.md` for the required-check posture and its limits.

## 7. Known limits

- No migration generation, LLM judge, or agent orchestration (product boundary §3);
- no Sigstore keyless, transparency logs, or retention yet (managed scope, M4+);
- waivers require an explicit `--waivers` file (ADR-0008) — a repository cannot silently self-authorize.
