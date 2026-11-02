package receipt

import (
	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/inputmeta"
)

const (
	// SchemaVersion is the v0 receipt protocol identifier.
	SchemaVersion = "assurectl/receipt/v0"
	// ProtocolVersion is the protocol generation recorded by the verifier.
	ProtocolVersion = "v0"
	// VerifierName identifies this evaluator in receipts.
	VerifierName = "assurectl"

	// LayerProject is the workspace project policy layer.
	LayerProject = "project"
	// LayerProtocolBaseline is the built-in non-waivable protocol baseline.
	LayerProtocolBaseline = "protocol_baseline"
	// BuiltinProtocolBaselineSource marks the compiled-in baseline policy.
	BuiltinProtocolBaselineSource = "builtin:assurectl/protocol-baseline/v0"
)

// Receipt is the unsigned v0 advisory record. Every array is deterministic:
// evidence is sorted by id, requirement results follow policy order, findings
// keep the evaluation's order, and waivers are always empty in M1.
type Receipt struct {
	SchemaVersion       string                     `json:"schema_version"`
	Authority           domain.AuthorityTier       `json:"authority"`
	Subject             domain.Subject             `json:"subject"`
	WorkUnit            domain.WorkUnit            `json:"work_unit"`
	Contract            DigestedInput              `json:"contract"`
	Policy              ResolvedPolicy             `json:"policy"`
	Evidence            []EvidenceReference        `json:"evidence"`
	RequirementResults  []domain.RequirementResult `json:"requirement_results"`
	Findings            []domain.Finding           `json:"findings"`
	Waivers             []WaiverRecord             `json:"waivers"`
	VerificationVerdict domain.VerificationVerdict `json:"verification_verdict"`
	CompletionDecision  domain.CompletionDecision  `json:"completion_decision"`
	Verifier            Verifier                   `json:"verifier"`
	EvaluatedAt         string                     `json:"evaluated_at"`
}

// DigestedInput records an externally resolved input by source and digest.
type DigestedInput struct {
	Source      string                `json:"source"`
	Digest      inputmeta.Digest      `json:"digest"`
	TrustStatus inputmeta.TrustStatus `json:"trust_status"`
}

// ResolvedPolicy records the effective policy and its contributing layers.
type ResolvedPolicy struct {
	Sources         []PolicySource        `json:"sources"`
	EffectiveDigest inputmeta.Digest      `json:"effective_digest"`
	TrustStatus     inputmeta.TrustStatus `json:"trust_status"`
}

// PolicySource records one contributing policy layer. Revision binding stays
// unset for the built-in baseline: it binds to no base revision.
type PolicySource struct {
	Layer           string                   `json:"layer"`
	Source          string                   `json:"source"`
	Digest          inputmeta.Digest         `json:"digest"`
	TrustStatus     inputmeta.TrustStatus    `json:"trust_status"`
	AuthorityBasis  inputmeta.AuthorityBasis `json:"authority_basis"`
	RevisionBinding string                   `json:"revision_binding,omitempty"`
	SourceRevision  string                   `json:"source_revision,omitempty"`
}

// EvidenceReference points at one supplied evidence envelope.
type EvidenceReference struct {
	ID     string           `json:"id"`
	Digest inputmeta.Digest `json:"digest"`
}

// Verifier identifies the evaluator that produced the receipt.
type Verifier struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	ProtocolVersion string `json:"protocol_version"`
}
