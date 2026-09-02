package decision

import (
	"fmt"
	"strings"

	"github.com/kefyusuf/assurectl/internal/domain"
)

var blockedIndeterminate = domain.EvaluationResult{
	Verdict:  domain.VerdictIndeterminate,
	Decision: domain.DecisionBlocked,
}

func Evaluate(requirements []domain.RequirementResult, findings ...domain.Finding) (domain.EvaluationResult, error) {
	knownFailure := false
	indeterminate := false

	for i, requirement := range requirements {
		if err := validateRequirement(i, requirement); err != nil {
			return blockedIndeterminate, err
		}

		if requirement.EvidenceState != domain.EvidenceValid {
			indeterminate = true
			continue
		}

		switch requirement.Outcome {
		case domain.OutcomeFailed:
			knownFailure = true
		case domain.OutcomeError:
			indeterminate = true
		}
	}

	hasBlockingFinding := false
	for i, finding := range findings {
		if err := validateFinding(i, finding); err != nil {
			return blockedIndeterminate, err
		}
		if finding.Blocking {
			hasBlockingFinding = true
		}
	}

	if len(requirements) == 0 {
		return blockedIndeterminate, nil
	}

	verdict := domain.VerdictPassed
	if knownFailure {
		verdict = domain.VerdictFailed
	} else if indeterminate || hasBlockingFinding {
		verdict = domain.VerdictIndeterminate
	}

	if indeterminate || hasBlockingFinding {
		return domain.EvaluationResult{
			Verdict:  verdict,
			Decision: domain.DecisionBlocked,
		}, nil
	}

	if verdict == domain.VerdictPassed {
		return domain.EvaluationResult{
			Verdict:  verdict,
			Decision: domain.DecisionApproved,
		}, nil
	}

	if everyFailureIsWaived(requirements) {
		return domain.EvaluationResult{
			Verdict:  verdict,
			Decision: domain.DecisionAcceptedWithRisk,
		}, nil
	}

	return domain.EvaluationResult{
		Verdict:  verdict,
		Decision: domain.DecisionRejected,
	}, nil
}

func validateRequirement(index int, requirement domain.RequirementResult) error {
	if strings.TrimSpace(requirement.RequirementID) == "" {
		return fmt.Errorf("requirement[%d].requirement_id: must not be empty", index)
	}
	if !requirement.EvidenceState.Valid() {
		return fmt.Errorf("requirement %q evidence_state: unsupported value %q", requirement.RequirementID, requirement.EvidenceState)
	}
	if requirement.Outcome != "" && !requirement.Outcome.Valid() {
		return fmt.Errorf("requirement %q outcome: unsupported value %q", requirement.RequirementID, requirement.Outcome)
	}
	if requirement.EvidenceState == domain.EvidenceValid && !requirement.Outcome.Valid() {
		return fmt.Errorf("requirement %q outcome: required for VALID evidence", requirement.RequirementID)
	}
	if !requirement.WaiverStatus.Valid() {
		return fmt.Errorf("requirement %q waiver_status: unsupported value %q", requirement.RequirementID, requirement.WaiverStatus)
	}
	if requirement.WaiverStatus == domain.WaiverValid &&
		(!requirement.Waivable || requirement.EvidenceState != domain.EvidenceValid || requirement.Outcome != domain.OutcomeFailed) {
		return fmt.Errorf("requirement %q waiver_status: VALID is permitted only for a waivable VALID/FAILED requirement", requirement.RequirementID)
	}
	return nil
}

func validateFinding(index int, finding domain.Finding) error {
	if finding.Code == "" {
		return fmt.Errorf("finding[%d].code: must not be empty", index)
	}
	if !finding.Category.Valid() {
		return fmt.Errorf("finding[%d].category: unsupported value %q", index, finding.Category)
	}
	if !finding.Severity.Valid() {
		return fmt.Errorf("finding[%d].severity: unsupported value %q", index, finding.Severity)
	}
	if finding.Message == "" {
		return fmt.Errorf("finding[%d].message: must not be empty", index)
	}
	return nil
}

func everyFailureIsWaived(requirements []domain.RequirementResult) bool {
	for _, requirement := range requirements {
		if requirement.EvidenceState != domain.EvidenceValid || requirement.Outcome != domain.OutcomeFailed {
			continue
		}
		if !requirement.Waivable || requirement.WaiverStatus != domain.WaiverValid {
			return false
		}
	}
	return true
}
