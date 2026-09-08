package decision

import (
	"strings"
	"testing"

	"github.com/kefyusuf/assurectl/internal/domain"
)

func TestEvaluateRejectsMalformedEvidenceReferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		requirement domain.RequirementResult
		wantField   string
	}{
		{
			name: "non-canonical requirement id",
			requirement: domain.RequirementResult{
				RequirementID: "unit tests",
				EvidenceState: domain.EvidenceValid,
				EvidenceIDs:   []string{"ev-unit-tests"},
				Outcome:       domain.OutcomePassed,
				WaiverStatus:  domain.WaiverNotApplicable,
			},
			wantField: "requirement_id",
		},
		{
			name: "empty evidence id",
			requirement: domain.RequirementResult{
				RequirementID: "unit-tests",
				EvidenceState: domain.EvidenceValid,
				EvidenceIDs:   []string{""},
				Outcome:       domain.OutcomePassed,
				WaiverStatus:  domain.WaiverNotApplicable,
			},
			wantField: "evidence_ids",
		},
		{
			name: "path-like evidence id",
			requirement: domain.RequirementResult{
				RequirementID: "unit-tests",
				EvidenceState: domain.EvidenceValid,
				EvidenceIDs:   []string{"../evidence"},
				Outcome:       domain.OutcomePassed,
				WaiverStatus:  domain.WaiverNotApplicable,
			},
			wantField: "evidence_ids",
		},
		{
			name: "duplicate evidence id",
			requirement: domain.RequirementResult{
				RequirementID: "unit-tests",
				EvidenceState: domain.EvidenceValid,
				EvidenceIDs:   []string{"ev-unit-tests", "ev-unit-tests"},
				Outcome:       domain.OutcomePassed,
				WaiverStatus:  domain.WaiverNotApplicable,
			},
			wantField: "evidence_ids",
		},
	}

	want := domain.EvaluationResult{
		Verdict:  domain.VerdictIndeterminate,
		Decision: domain.DecisionBlocked,
	}
	for _, tt := range tests {
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
				t.Fatalf("Evaluate() = %#v, want %#v", got, want)
			}
		})
	}
}
