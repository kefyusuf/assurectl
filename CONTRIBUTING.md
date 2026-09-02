# Contributing to AssureCTL

AssureCTL is trust-sensitive infrastructure. Small semantic changes can alter merge and release decisions, so contributions must preserve explicit boundaries and leave reviewable evidence.

## Before changing code

1. Read the foundation design, relevant ADRs, and `docs/threat-model/v0.md`.
2. Confirm the change belongs to the current milestone.
3. For behavior changes, write a failing test and observe the expected failure before implementation.
4. Use a topic branch and submit a pull request.

## Required local checks

Run all of the following before requesting review:

```bash
files=$(find . -name '*.go' -type f -not -path './vendor/*')
test -z "$(gofmt -l $files)"
go vet ./...
go test -race ./...
go build ./cmd/assurectl
```

## Architectural changes

Add or supersede an ADR when a change affects:

- protocol semantics or stable terminology;
- trust boundaries or authority tiers;
- subject, policy, contract, evidence, waiver, verdict, decision, or receipt models;
- schema compatibility;
- public package or CLI behavior;
- signing or attestation direction.

Accepted ADRs are not silently rewritten. Mark the previous ADR as superseded and link the replacement.

## Dependency policy

M0 uses only the Go standard library. A new dependency requires written justification covering necessity, maintenance, license, supply-chain risk, and why a small internal implementation is not appropriate.

## Review discipline

A passing test suite is necessary but not sufficient. Reviewers also check scope, claims, decision-table semantics, failure behavior, policy weakening, and trust-boundary changes.
