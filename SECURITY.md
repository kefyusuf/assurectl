# Security Policy

## Supported versions

AssureCTL is pre-release software. Only the current `main` branch receives security fixes. No released version is currently designated production-ready.

## Reporting a vulnerability

Do not publish undisclosed vulnerabilities, exploit details, credentials, private evidence, or sensitive repository data in a public issue.

Use GitHub private vulnerability reporting for this repository when that feature is available. If private reporting is unavailable, open a public issue containing only a request for a private reporting channel and no vulnerability details.

A useful private report includes:

- the affected commit or version;
- the relevant trust boundary or protocol component;
- reproduction steps with synthetic data;
- the impact on subject binding, policy integrity, producer identity, evidence integrity, waiver authorization, decision integrity, or receipt integrity;
- any known mitigations.

## Scope notes

The project does not treat an agent summary, status file, self-generated key, or self-authored evidence assertion as authoritative. Reports showing a path from an untrusted worker to an authoritative completion decision are especially important.
