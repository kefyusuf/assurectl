# Self-Hosting the AssureCTL App

This runbook walks through running your own instance of the reference GitHub App (`assurectl-app`) so that merge-gating checks are posted from outside your repositories (ADR-0010). It claims nothing the code does not implement: the App resolves the gate documents from the base revision, ingests CI-produced evidence, and posts a check whose conclusion derives from the receipt — nothing more.

## 1. Prerequisites

- A Go toolchain matching the repository's CI matrix;
- a GitHub App registration (step 2);
- repositories whose CI produces `assurectl-evidence` artifacts containing a `.assurectl/evidence/` bundle (see the reference action and this repository's own workflow), with the OIDC token relayed in `ci-token.txt`.

## 2. Register the App

1. GitHub → Settings → Developer settings → GitHub Apps → **New GitHub App**;
2. set the webhook URL to your `assurectl-app` endpoint (`https://your-host/webhook`) and a webhook secret;
3. grant **Repository permissions**: `checks: read & write`, `contents: read`, `actions: read`;
4. subscribe to **Pull request** events;
5. note the **App ID** and download the **private key** (PEM);
6. note the **client ID** if you want it as the OIDC audience.

## 3. Run the App

```bash
assurectl-app \
  --app-id 123456 \
  --private-key-path /secure/app.private-key.pem \
  --webhook-secret-path /secure/webhook-secret \
  --remote-template 'https://github.com/%s.git' \
  --oidc-audience '<your app client id>' \
  --listen ':8080'
```

- Without `--api-base-url` the process stays in the fail-closed skeleton mode (ports unwired): webhooks are signature-verified but evaluation fails closed. Supplying `--api-base-url` (default `https://api.github.com`) wires the real adapters.
- Secrets are read from files and never logged; worktree fetches use the configured `--remote-template` only — webhook payloads never name a remote.
- The private key never leaves the process; the App JWT lives for ten minutes and installation tokens are cached until near expiry.

## 4. Install and gate

1. Install the App on the target repositories;
2. in each repository's branch protection, make the `assurectl` check required — the check run is posted by your App installation, so a pull request cannot forge, edit, or delete it;
3. keep the repository's own workflow verification (the reference action) as developer feedback; the App's check is the merge authority.

## 5. What the check claims

- `AUTHORITATIVE`: the gate documents came from the pull request's base revision, the evidence bundle was bound to the head revision, and the producing workflow presented a valid GitHub Actions OIDC token whose `repository`/`sha` claims matched the resolved subject;
- `ADVISORY` (capped): any failure in that OIDC validation — the check text and receipt say so;
- the conclusion maps from the receipt's `completion_decision` exactly like the CLI exit codes; it never overrides the receipt.

## 6. Known limits

- No replay protection, freshness rules, signing, or evidence retention yet (M4 and managed scope);
- the App trusts the machine it runs on: host hardening and secret management are your operational responsibility;
- one App installation per organization is the tested shape; cross-org trust is untested.
