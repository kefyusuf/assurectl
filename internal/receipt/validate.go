package receipt

import (
	"fmt"
	"time"

	"github.com/kefyusuf/assurectl/internal/decision"
	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/inputmeta"
)

// validateBuildInput rejects malformed or trust-overclaiming build inputs
// before any receipt structure exists.
func validateBuildInput(input BuildInput) error {
	if input.Authority != domain.AuthorityAdvisory {
		return fmt.Errorf("authority %q is not advisory: M1 produces advisory receipts only", input.Authority)
	}
	if input.EvaluatedAt.IsZero() {
		return fmt.Errorf("evaluated_at must be supplied")
	}
	seenEvidence := make(map[string]inputmeta.Digest, len(input.Evidence))
	for _, entry := range input.Evidence {
		id := entry.Envelope.ID
		if previous, exists := seenEvidence[id]; exists && previous != entry.Digest {
			return fmt.Errorf("evidence id %q is supplied twice with different digests", id)
		}
		seenEvidence[id] = entry.Digest
	}
	return nil
}

// validateReceipt enforces the semantic invariants the M0 execution notes
// deferred, for every receipt including BLOCKED ones: unique requirement
// identifiers, exact evidence membership and digest binding, waiver target
// and applicability rules, non-blocking findings only outside BLOCKED, a
// complete verifier block, and a parseable RFC3339 evaluation instant.
func validateReceipt(receipt Receipt) error {
	if err := validateRequirementUniqueness(receipt.RequirementResults); err != nil {
		return err
	}
	if err := validateDecisionDerivation(receipt); err != nil {
		return err
	}
	evidenceDigests := make(map[string]inputmeta.Digest, len(receipt.Evidence))
	for _, reference := range receipt.Evidence {
		if _, duplicate := evidenceDigests[reference.ID]; duplicate {
			return fmt.Errorf("evidence %q is referenced twice", reference.ID)
		}
		evidenceDigests[reference.ID] = reference.Digest
	}
	if err := validateEvidenceBinding(receipt.RequirementResults, evidenceDigests); err != nil {
		return err
	}
	if err := validateWaiverRecords(receipt.Waivers, receipt.RequirementResults, receipt.Subject.RepositoryURI, receipt.WorkUnit.ID); err != nil {
		return err
	}
	if receipt.CompletionDecision != domain.DecisionBlocked {
		for _, finding := range receipt.Findings {
			if finding.Blocking {
				return fmt.Errorf("decision %q carries blocking finding %q", receipt.CompletionDecision, finding.Code)
			}
		}
	}
	if receipt.Verifier.Name != VerifierName || receipt.Verifier.Version == "" || receipt.Verifier.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("verifier block is incomplete: %#v", receipt.Verifier)
	}
	if _, err := time.Parse(time.RFC3339, receipt.EvaluatedAt); err != nil {
		return fmt.Errorf("evaluated_at %q is not a valid RFC3339 timestamp", receipt.EvaluatedAt)
	}
	return nil
}

func validateRequirementUniqueness(results []domain.RequirementResult) error {
	seen := make(map[string]struct{}, len(results))
	for i, result := range results {
		if _, exists := seen[result.RequirementID]; exists {
			return fmt.Errorf("requirement_results[%d]: duplicate requirement_id %q", i, result.RequirementID)
		}
		seen[result.RequirementID] = struct{}{}
	}
	return nil
}

// validateDecisionDerivation re-runs the deterministic kernel over the
// receipt's own requirement results and findings and fails on any mismatch
// with the pair destined for the receipt: the recorded decision is derived,
// never supplied.
func validateDecisionDerivation(receipt Receipt) error {
	derived, err := decision.Evaluate(receipt.RequirementResults, receipt.Findings...)
	if err != nil {
		return fmt.Errorf("decision re-derivation: %w", err)
	}
	recorded := domain.EvaluationResult{
		Verdict:  receipt.VerificationVerdict,
		Decision: receipt.CompletionDecision,
	}
	if derived != recorded {
		return fmt.Errorf("receipt records (%q, %q) but the kernel derives (%q, %q)",
			recorded.Verdict, recorded.Decision, derived.Verdict, derived.Decision)
	}
	return nil
}

func validateEvidenceBinding(results []domain.RequirementResult, digests map[string]inputmeta.Digest) error {
	for _, result := range results {
		if len(result.EvidenceIDs) > 0 {
			for _, id := range result.EvidenceIDs {
				if _, exists := digests[id]; !exists {
					return fmt.Errorf("requirement %q references evidence %q that is not supplied", result.RequirementID, id)
				}
			}
		}
		if result.EvidenceState == domain.EvidenceValid && len(result.EvidenceIDs) == 0 {
			return fmt.Errorf("requirement %q is VALID without evidence references", result.RequirementID)
		}
	}
	return nil
}

func validateWaiverRecords(records []WaiverRecord, results []domain.RequirementResult, repositoryURI, workUnitID string) error {
	requirements := make(map[string]domain.RequirementResult, len(results))
	for _, result := range results {
		requirements[result.RequirementID] = result
	}
	for _, record := range records {
		if record.Target.Kind != "REQUIREMENT" {
			return fmt.Errorf("waiver %q targets unsupported kind %q", record.ID, record.Target.Kind)
		}
		result, exists := requirements[record.Target.ID]
		if !exists {
			return fmt.Errorf("waiver %q targets unknown requirement %q", record.ID, record.Target.ID)
		}
		if result.EvidenceState != domain.EvidenceValid || result.Outcome != domain.OutcomeFailed {
			return fmt.Errorf("waiver %q targets requirement %q which is not an established failure", record.ID, result.RequirementID)
		}
		if record.Status == domain.WaiverValid {
			if !result.Waivable {
				return fmt.Errorf("waiver %q claims VALID on non-waivable requirement %q", record.ID, result.RequirementID)
			}
			if record.Scope.RepositoryURI != repositoryURI || record.Scope.WorkUnitID != workUnitID {
				return fmt.Errorf("waiver %q scope does not match the receipt subject and work unit", record.ID)
			}
			if _, err := time.Parse(time.RFC3339, record.IssuedAt); err != nil {
				return fmt.Errorf("waiver %q issued_at %q is not RFC3339", record.ID, record.IssuedAt)
			}
			if _, err := time.Parse(time.RFC3339, record.ExpiresAt); err != nil {
				return fmt.Errorf("waiver %q expires_at %q is not RFC3339", record.ID, record.ExpiresAt)
			}
		}
	}
	return nil
}
