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

func Evaluate(requirements []domain.RequirementResult) (domain.EvaluationResult, error) {
	if len(requirements) == 0 {
		return blockedIndeterminate, nil
	}

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

	verdict := domain.VerdictPassed
	if knownFailure {
		verdict = domain.VerdictFailed
	} else if indeterminate {
		verdict = domain.VerdictIndeterminate
	}

	if indeterminate {
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
