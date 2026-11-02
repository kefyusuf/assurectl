package receipt

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/kefyusuf/assurectl/internal/buildinfo"
	"github.com/kefyusuf/assurectl/internal/contract"
	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/evaluation"
	"github.com/kefyusuf/assurectl/internal/evidence"
	"github.com/kefyusuf/assurectl/internal/inputmeta"
	"github.com/kefyusuf/assurectl/internal/policy"
	"github.com/kefyusuf/assurectl/internal/waiver"
)

// BuildInput is the closed set of externally resolved facts one receipt may
// record. Nothing inside it is consulted as a trust root.
type BuildInput struct {
	Authority   domain.AuthorityTier
	Subject     domain.Subject
	Contract    contract.Loaded
	Policy      policy.Loaded
	Evidence    []evidence.Loaded
	Evaluation  evaluation.Result
	EvaluatedAt time.Time
}

// Build maps the evaluated facts onto the v0 advisory receipt. It is
// fail-closed: every semantic check must pass before a receipt is returned.
func Build(input BuildInput) (Receipt, error) {
	if err := validateBuildInput(input); err != nil {
		return Receipt{}, fmt.Errorf("build receipt: %w", err)
	}

	built := Receipt{
		SchemaVersion: SchemaVersion,
		Authority:     input.Authority,
		Subject:       input.Subject,
		WorkUnit:      input.Contract.Contract.WorkUnit,
		Contract: DigestedInput{
			Source:      input.Contract.Source,
			Digest:      input.Contract.Digest,
			TrustStatus: input.Contract.TrustStatus,
		},
		Policy: ResolvedPolicy{
			Sources: []PolicySource{
				{
					Layer:          LayerProtocolBaseline,
					Source:         BuiltinProtocolBaselineSource,
					Digest:         builtinDigest(),
					TrustStatus:    inputmeta.TrustStatusUntrusted,
					AuthorityBasis: inputmeta.AuthorityBasisAdvisoryWorkspace,
				},
				{
					Layer:          LayerProject,
					Source:         input.Policy.Source,
					Digest:         input.Policy.Digest,
					TrustStatus:    input.Policy.TrustStatus,
					AuthorityBasis: input.Policy.AuthorityBasis,
				},
			},
			EffectiveDigest: input.Policy.Digest,
			TrustStatus:     input.Policy.TrustStatus,
		},
		Evidence:            evidenceReferences(input.Evidence),
		RequirementResults:  copyRequirements(input.Evaluation.Requirements),
		Findings:            copyFindings(input.Evaluation.Findings),
		Waivers:             convertWaivers(input.Evaluation.Waivers),
		VerificationVerdict: input.Evaluation.Verdict.Verdict,
		CompletionDecision:  input.Evaluation.Verdict.Decision,
		Verifier: Verifier{
			Name:            VerifierName,
			Version:         buildinfo.Version,
			ProtocolVersion: ProtocolVersion,
		},
		EvaluatedAt: input.EvaluatedAt.UTC().Format(time.RFC3339),
	}
	if err := validateReceipt(built); err != nil {
		return Receipt{}, fmt.Errorf("build receipt: %w", err)
	}
	return built, nil
}

func evidenceReferences(entries []evidence.Loaded) []EvidenceReference {
	references := make([]EvidenceReference, 0, len(entries))
	for _, entry := range entries {
		references = append(references, EvidenceReference{
			ID:     entry.Envelope.ID,
			Digest: entry.Digest,
		})
	}
	sort.SliceStable(references, func(i, j int) bool {
		return references[i].ID < references[j].ID
	})
	return references
}

func copyRequirements(requirements []domain.RequirementResult) []domain.RequirementResult {
	copied := make([]domain.RequirementResult, 0, len(requirements))
	for _, requirement := range requirements {
		ids := make([]string, len(requirement.EvidenceIDs))
		copy(ids, requirement.EvidenceIDs)
		requirement.EvidenceIDs = ids
		copied = append(copied, requirement)
	}
	return copied
}

func copyFindings(findings []domain.Finding) []domain.Finding {
	copied := make([]domain.Finding, 0, len(findings))
	for _, finding := range findings {
		ids := make([]string, len(finding.EvidenceIDs))
		copy(ids, finding.EvidenceIDs)
		finding.EvidenceIDs = ids
		copied = append(copied, finding)
	}
	return copied
}

// convertWaivers maps the evaluation's resolved waiver records onto the
// receipt's waiver_record structure, field for field.
func convertWaivers(records []waiver.Record) []WaiverRecord {
	converted := make([]WaiverRecord, 0, len(records))
	for _, record := range records {
		converted = append(converted, WaiverRecord{
			ID:     record.ID,
			Target: WaiverTarget{Kind: record.Target.Kind, ID: record.Target.ID},
			Scope: WaiverScope{
				RepositoryURI: record.Scope.RepositoryURI,
				WorkUnitID:    record.Scope.WorkUnitID,
				HeadRevision:  record.Scope.HeadRevision,
			},
			Reason:       record.Reason,
			AcceptedRisk: record.AcceptedRisk,
			Approver:     WaiverApprover{Identity: record.Approver.Identity, Authority: record.Approver.Authority},
			IssuedAt:     record.IssuedAt,
			ExpiresAt:    record.ExpiresAt,
			Status:       record.Status,
			Digest:       record.Digest,
		})
	}
	return converted
}

func builtinDigest() inputmeta.Digest {
	// The protocol baseline is compiled in; its recorded digest identifies
	// the built-in definition rather than a policy file.
	return inputmeta.SHA256([]byte(BuiltinProtocolBaselineSource))
}

// JSON renders the receipt as deterministic compact JSON.
func (r Receipt) JSON() ([]byte, error) {
	return json.Marshal(r)
}

// Digest returns the SHA-256 of the exact emitted receipt bytes.
func (r Receipt) Digest() (inputmeta.Digest, error) {
	data, err := r.JSON()
	if err != nil {
		return inputmeta.Digest{}, err
	}
	return inputmeta.SHA256(data), nil
}
