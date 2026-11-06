package appserver

import (
	"context"
)

// PullRequestEvent is the closed view of the GitHub webhook fields the App
// consumes.
type PullRequestEvent struct {
	InstallationID int64
	Repository     string // "owner/name"
	Action         string
	PRNumber       int64
	BaseSHA        string
	HeadSHA        string
}

// WorktreeSource materializes a local worktree for a repository at a
// revision. The real adapter performs server-side Git operations.
type WorktreeSource interface {
	Worktree(ctx context.Context, installation int64, repository, revision string) (string, error)
}

// EvidenceSource materializes an evidence bundle laid out as
// <bundle>/.assurectl/evidence/... and returns the CI OIDC token that
// accompanies it (possibly empty).
type EvidenceSource interface {
	EvidenceBundle(ctx context.Context, installation int64, repository string, prNumber int64, headSHA string) (bundle string, ciToken string, err error)
}

// CheckPoster posts the check run for a pull request. headSHA is required
// by the GitHub check-runs API and travels with the port, not the payload
// protocol.
type CheckPoster interface {
	PostCheck(ctx context.Context, installation int64, repository string, prNumber int64, headSHA string, payload CheckPayload) error
}

// ExpectedClaims names what a validated CI token must bind to.
type ExpectedClaims struct {
	Repository string // canonical repository identity
	SHA        string // head revision
}

// ValidatedClaims carries the OIDC claims the App relies on after checking
// their bindings.
type ValidatedClaims struct {
	Repository  string
	WorkflowRef string
	WorkflowSHA string
	SHA         string
}

// TokenValidator validates the CI OIDC token accompanying an evidence
// bundle. The real adapter fetches JWKS over the network; tests inject
// keys.
type TokenValidator interface {
	Validate(ctx context.Context, token string, expect ExpectedClaims) (ValidatedClaims, error)
}

// CheckPayload is the typed form of the check-payload.v0 protocol.
type CheckPayload struct {
	Name             string           `json:"name"`
	Conclusion       string           `json:"conclusion"`
	Title            string           `json:"title"`
	Summary          string           `json:"summary"`
	ReceiptReference ReceiptReference `json:"receipt_reference"`
}

// ReceiptReference points back at the receipt the check summarizes.
type ReceiptReference struct {
	ChangeSetDigest string `json:"change_set_digest"`
	ReceiptSHA256   string `json:"receipt_sha256"`
	Authority       string `json:"authority"`
}

// Outcome carries what one evaluation produced for the caller.
type Outcome struct {
	Payload CheckPayload
}
