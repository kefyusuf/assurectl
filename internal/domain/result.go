package domain

type VerificationVerdict string

const (
	VerdictPassed        VerificationVerdict = "PASSED"
	VerdictFailed        VerificationVerdict = "FAILED"
	VerdictIndeterminate VerificationVerdict = "INDETERMINATE"
)

func (v VerificationVerdict) Valid() bool {
	switch v {
	case VerdictPassed, VerdictFailed, VerdictIndeterminate:
		return true
	default:
		return false
	}
}

type CompletionDecision string

const (
	DecisionApproved         CompletionDecision = "APPROVED"
	DecisionRejected         CompletionDecision = "REJECTED"
	DecisionBlocked          CompletionDecision = "BLOCKED"
	DecisionAcceptedWithRisk CompletionDecision = "ACCEPTED_WITH_RISK"
)

func (v CompletionDecision) Valid() bool {
	switch v {
	case DecisionApproved, DecisionRejected, DecisionBlocked, DecisionAcceptedWithRisk:
		return true
	default:
		return false
	}
}

type RequirementResult struct {
	RequirementID string          `json:"requirement_id"`
	EvidenceState EvidenceState   `json:"evidence_state"`
	Outcome       ObservedOutcome `json:"outcome,omitempty"`
	Waivable      bool            `json:"waivable"`
	WaiverStatus  WaiverStatus    `json:"waiver_status"`
}

type EvaluationResult struct {
	Verdict  VerificationVerdict `json:"verification_verdict"`
	Decision CompletionDecision  `json:"completion_decision"`
}
