## Summary

<!-- What changed and why? -->

## Scope

- [ ] The change belongs to the active milestone.
- [ ] The PR does not introduce unrelated refactoring or product-scope expansion.

## Verification

- [ ] Behavior changes were developed test-first and the new tests were observed failing before implementation.
- [ ] `gofmt` reports no files.
- [ ] `go vet ./...` passes.
- [ ] `go test -race ./...` passes.
- [ ] `go build ./cmd/assurectl` passes.

## Assurance review

- [ ] Documentation does not claim correctness, security, bug freedom, or production readiness beyond available evidence.
- [ ] Evidence state, outcome, verdict, waiver, and completion decision remain distinct.
- [ ] No policy, test, trust requirement, or security check was weakened to make the change pass.
- [ ] Trust-boundary, protocol, schema, or public semantic changes have an ADR or explicitly state why none is required.
- [ ] No secrets, private evidence, or sensitive data are included.

## Known limitations

<!-- State anything intentionally deferred or not verified. -->
