package decision

import (
	"reflect"
	"strings"
	"testing"

	"github.com/kefyusuf/assurectl/internal/domain"
)

func TestEvaluateDecisionTable(t *testing.T) {
	t.Parallel()

	passed := requirement("unit-tests", domain.EvidenceValid, domain.OutcomePassed, false, domain.WaiverNotApplicable)
	failed := requirement("integration-tests", domain.EvidenceValid, domain.OutcomeFailed, false, domain.WaiverNotApplicable)
	waivedFailure := requirement("integration-tests", domain.EvidenceValid, domain.OutcomeFailed, true, domain.WaiverValid)

	tests := []struct {
		name         string
		requirements []domain.RequirementResult
		want         domain.EvaluationResult
	}{
		{"no requirements", nil, evaluation(domain.VerdictIndeterminate, domain.DecisionBlocked)},
		{"all valid and passed", []domain.RequirementResult{passed}, evaluation(domain.VerdictPassed, domain.DecisionApproved)},
		{"valid definitive failure", []domain.RequirementResult{failed}, evaluation(domain.VerdictFailed, domain.DecisionRejected)},
		{"waived definitive failure", []domain.RequirementResult{waivedFailure}, evaluation(domain.VerdictFailed, domain.DecisionAcceptedWithRisk)},
		{"waivable failure without waiver", []domain.RequirementResult{requirement("integration-tests", domain.EvidenceValid, domain.OutcomeFailed, true, domain.WaiverNotApplicable)}, evaluation(domain.VerdictFailed, domain.DecisionRejected)},
		{"missing evidence", []domain.RequirementResult{requirement("unit-tests", domain.EvidenceMissing, "", false, domain.WaiverNotApplicable)}, evaluation(domain.VerdictIndeterminate, domain.DecisionBlocked)},
		{"stale evidence", []domain.RequirementResult{requirement("unit-tests", domain.EvidenceStale, domain.OutcomePassed, false, domain.WaiverNotApplicable)}, evaluation(domain.VerdictIndeterminate, domain.DecisionBlocked)},
		{"invalid evidence", []domain.RequirementResult{requirement("unit-tests", domain.EvidenceInvalid, "", false, domain.WaiverNotApplicable)}, evaluation(domain.VerdictIndeterminate, domain.DecisionBlocked)},
		{"untrusted evidence", []domain.RequirementResult{requirement("unit-tests", domain.EvidenceUntrusted, domain.OutcomePassed, false, domain.WaiverNotApplicable)}, evaluation(domain.VerdictIndeterminate, domain.DecisionBlocked)},
		{"known failure plus missing evidence", []domain.RequirementResult{failed, requirement("security-scan", domain.EvidenceMissing, "", false, domain.WaiverNotApplicable)}, evaluation(domain.VerdictFailed, domain.DecisionBlocked)},
		{"errored check", []domain.RequirementResult{requirement("unit-tests", domain.EvidenceValid, domain.OutcomeError, false, domain.WaiverNotApplicable)}, evaluation(domain.VerdictIndeterminate, domain.DecisionBlocked)},
		{"mixed passed and waived failures", []domain.RequirementResult{passed, waivedFailure}, evaluation(domain.VerdictFailed, domain.DecisionAcceptedWithRisk)},
		{"mixed waived and non-waived failures", []domain.RequirementResult{waivedFailure, requirement("security-scan", domain.EvidenceValid, domain.OutcomeFailed, false, domain.WaiverNotApplicable)}, evaluation(domain.VerdictFailed, domain.DecisionRejected)},
	}

	for _, tt := range tests {
		t := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Evaluate(tt.requirements)
			if err != nil {
				t.Fatalf("Evaluate() unexpected error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("Evaluate() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestEvaluateRejectsMalformedInputsFailClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		requirement domain.RequirementResult
		wantField   string
	}{
		{"missing requirement id", requirement("", domain.EvidenceValid, domain.OutcomePassed, false, domain.WaiverNotApplicable), "requirement_id"},
		{"invalid evidence state", requirement("unit-tests", domain.EvidenceState("UNKNOWN"), "", false, domain.WaiverNotApplicable), "evidence_state"},
		{"invalid outcome", requirement("unit-tests", domain.EvidenceValid, domain.ObservedOutcome("UNKNOWN"), false, domain.WaiverNotApplicable), "outcome"},
		{"invalid waiver status", requirement("unit-tests", domain.EvidenceValid, domain.OutcomePassed, false, domain.WaiverStatus("UNKNOWN")), "waiver_status"},
		{"valid waiver on non-waivable requirement", requirement("unit-tests", domain.EvidenceValid, domain.OutcomeFailed, false, domain.WaiverValid), "waiver_status"},
		{"valid waiver on passing requirement", requirement("unit-tests", domain.EvidenceValid, domain.OutcomePassed, true, domain.WaiverValid), "waiver_status"},
		{"valid waiver on indeterminate requirement", requirement("unit-tests", domain.EvidenceMissing, "", true, domain.WaiverValid), "waiver_status"},
	}

	want := evaluation(domain.VerdictIndeterminate, domain.DecisionBlocked)
	for _, tt := range tests {
		t := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Evaluate([]domain.RequirementResult{tt.requirement})
			if err == nil {
				t.Fatal("Evaluate() error = nil, want non-nil")
			}
			if !strings.Contains(err.Error(), tt.wantField) {
				t.Fatalf("error = %q, want field %q", err, tt.wantField)
			}
			if got != want {
				t.Fatalf("fail-closed result = %#v, want %#v", got, want)
			}
		})
	}
}

func TestEvaluateDoesNotMutateInput(t *testing.T) {
	t.Parallel()

	requirements := []domain.RequirementResult{
		requirement("unit-tests", domain.EvidenceValid, domain.OutcomeFailed, true, domain.WaiverValid),
	}
	before := append([]domain.RequirementResult(nil), requirements...)

	if _, err := Evaluate(requirements); err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if !reflect.DeepEqual(requirements, before) {
		t.Fatalf("Evaluate() mutated input: got %#v, want %#v", requirements, before)
	}
}

func TestEvaluateIsRepeatable(t *testing.T) {
	t.Parallel()

	requirements := []domain.RequirementResult{
		requirement("unit-tests", domain.EvidenceValid, domain.OutcomePassed, false, domain.WaiverNotApplicable),
		requirement("integration-tests", domain.EvidenceValid, domain.OutcomeFailed, true, domain.WaiverValid),
	}
	first, err := Evaluate(requirements)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	for i := 0; i < 100; i++ {
		got, err := Evaluate(requirements)
		if err != nil {
			t.Fatalf("Evaluate() iteration %d error = %v", i, err)
		}
		if got != first {
			t.Fatalf("Evaluate() iteration %d = %#v, want %#v", i, got, first)
		}
	}
}

func requirement(id string, state domain.EvidenceState, outcome domain.ObservedOutcome, waivable bool, waiver domain.WaiverStatus) domain.RequirementResult {
	return domain.RequirementResult{
		RequirementID: id,
		EvidenceState: state,
		Outcome:       outcome,
		Waivable:      waivable,
		WaiverStatus:  waiver,
	}
}

func evaluation(verdict domain.VerificationVerdict, decision domain.CompletionDecision) domain.EvaluationResult {
	return domain.EvaluationResult{Verdict: verdict, Decision: decision}
}
