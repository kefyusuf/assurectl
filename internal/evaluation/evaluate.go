package evaluation

import (
	"fmt"
	"regexp"
	"time"

	"github.com/kefyusuf/assurectl/internal/contract"
	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/evidence"
	"github.com/kefyusuf/assurectl/internal/inputmeta"
	"github.com/kefyusuf/assurectl/internal/policy"
)

var (
	canonicalIdentifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	digestValuePattern  = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Input is the complete, closed set of facts one evaluation may consider.
// EvaluatedAt is supplied by the caller; this package never reads the wall
// clock so repeated evaluation of deeply equal input stays deeply equal.
type Input struct {
	Contract    contract.Loaded
	Policy      policy.Loaded
	Evidence    []evidence.Loaded
	EvaluatedAt time.Time
}

// Result carries normalized per-requirement facts, findings, and the
// verdict/decision pair produced by the deterministic kernel.
type Result struct {
	Requirements []domain.RequirementResult
	Findings     []domain.Finding
	Verdict      domain.EvaluationResult
}

// Evaluate is a pure function of Input: no clock, filesystem, network, or
// environment access, and no mutation of the inputs.
func Evaluate(input Input) (Result, error) {
	if err := validateInput(input); err != nil {
		return Result{}, fmt.Errorf("evaluate: %w", err)
	}

	requirements := make([]domain.RequirementResult, 0, len(input.Policy.Policy.Requirements))
	for _, requirement := range input.Policy.Policy.Requirements {
		requirements = append(requirements, domain.RequirementResult{
			RequirementID: requirement.ID,
			EvidenceState: domain.EvidenceMissing,
			Waivable:      requirement.Waivable,
			WaiverStatus:  domain.WaiverNotApplicable,
		})
	}
	return Result{Requirements: requirements}, nil
}

func validateInput(input Input) error {
	if input.EvaluatedAt.IsZero() {
		return fmt.Errorf("evaluated_at must be supplied")
	}
	if err := validateContractInput(input.Contract); err != nil {
		return err
	}
	if err := validatePolicyInput(input.Policy); err != nil {
		return err
	}

	seenRequirements := make(map[string]struct{}, len(input.Policy.Policy.Requirements))
	for i, requirement := range input.Policy.Policy.Requirements {
		if !canonicalIdentifier.MatchString(requirement.ID) {
			return fmt.Errorf("policy requirement[%d].id %q: must be a canonical identifier", i, requirement.ID)
		}
		if _, exists := seenRequirements[requirement.ID]; exists {
			return fmt.Errorf("policy requirement[%d].id: duplicate identifier %q", i, requirement.ID)
		}
		seenRequirements[requirement.ID] = struct{}{}
	}

	for i, criterion := range input.Contract.Contract.AcceptanceCriteria {
		for j, reference := range criterion.Requirements {
			if !canonicalIdentifier.MatchString(reference) {
				return fmt.Errorf(
					"contract acceptance criterion[%d] (%q).requirements[%d] %q: must be a canonical identifier",
					i, criterion.ID, j, reference,
				)
			}
		}
	}
	return nil
}

func validateContractInput(loaded contract.Loaded) error {
	if loaded.Contract.SchemaVersion != contract.SchemaVersion {
		return fmt.Errorf("contract schema_version %q is unsupported", loaded.Contract.SchemaVersion)
	}
	if err := validateDigest(loaded.Digest); err != nil {
		return fmt.Errorf("contract digest: %w", err)
	}
	if loaded.TrustStatus != inputmeta.TrustStatusUntrusted {
		return fmt.Errorf("contract trust status %q is not advisory workspace input", loaded.TrustStatus)
	}
	return nil
}

func validatePolicyInput(loaded policy.Loaded) error {
	if loaded.Policy.SchemaVersion != policy.SchemaVersion {
		return fmt.Errorf("policy schema_version %q is unsupported", loaded.Policy.SchemaVersion)
	}
	if err := validateDigest(loaded.Digest); err != nil {
		return fmt.Errorf("policy digest: %w", err)
	}
	if loaded.TrustStatus != inputmeta.TrustStatusUntrusted {
		return fmt.Errorf("policy trust status %q is not advisory workspace input", loaded.TrustStatus)
	}
	if loaded.AuthorityBasis != inputmeta.AuthorityBasisAdvisoryWorkspace {
		return fmt.Errorf("policy authority basis %q is not advisory workspace input", loaded.AuthorityBasis)
	}
	return nil
}

func validateDigest(digest inputmeta.Digest) error {
	if digest.Algorithm != inputmeta.DigestAlgorithmSHA256 {
		return fmt.Errorf("algorithm %q is unsupported", digest.Algorithm)
	}
	if !digestValuePattern.MatchString(digest.Value) {
		return fmt.Errorf("value %q is invalid", digest.Value)
	}
	return nil
}
