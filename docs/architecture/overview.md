# Architecture Overview

AssureCTL separates software-change inputs from the authority that issues a completion decision.

## Current M0 packages

| Package | Responsibility | Must not do |
|---|---|---|
| `cmd/assurectl` | Parse top-level commands, route output, map exit codes | Make domain decisions |
| `internal/buildinfo` | Expose linker-injected build metadata | Read repository or policy state |
| `internal/domain` | Define closed vocabulary and transport-neutral records | Perform I/O or policy evaluation |
| `internal/decision` | Validate normalized requirement results and findings, then apply deterministic verdict and completion-decision precedence | Load evidence, resolve finding causes, approve waivers, or mutate inputs |

## Planned evaluation flow

```text
CLI or adapter
     |
     v
subject resolver ---- contract loader
     |                       |
     +-----------+-----------+
                 v
           policy resolver
                 |
                 v
          evidence loader
                 |
                 v
        evidence validator
                 |
                 v
       verification engine
                 |
          +------+------+
          |             |
          v             v
 requirement results  findings
          |             |
          +------+------+
                 v
          waiver engine
                 |
                 v
       completion decision
                 |
                 v
          receipt builder
```

M0 implements only the closed domain vocabulary and the final decision-precedence kernel. The kernel accepts normalized requirement results and findings. A blocking finding yields `INDETERMINATE/BLOCKED` when no established failure exists and preserves `FAILED/BLOCKED` when a known failed requirement also exists. All I/O-heavy components remain deferred so their trust requirements can be specified before code is added.

## Dependency direction

- `cmd/assurectl` may depend on internal application packages.
- decision logic depends on `internal/domain` only.
- domain code depends on the Go standard library only.
- domain and decision packages do not depend on CLI, filesystems, Git, CI providers, network clients, or LLMs.

## Determinism

For normalized inputs, the decision engine performs no I/O, reads no clock, uses no randomness, mutates no input, and makes no network request. Evidence validation, finding generation, policy resolution, and waiver authorization normalize their outputs before the decision engine is called.
