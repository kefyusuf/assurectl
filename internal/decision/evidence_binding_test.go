package decision

import (
	"strings"
	"testing"

	"github.com/kefyusuf/assurectl/internal/domain"
)

func TestEvaluateRejectsValidResultWithoutEvidenceReferences(t *testing.T) {
	t.Parallel()

	got, err := Evaluate([]domain.RequirementResult{
		{
			RequirementID: "unit-tests",
			EvidenceState: domain.EvidenceValid,
			Outcome:       domain.OutcomePassed,
			WaiverStatus:  domain.WaiverNotApplicable,
		},
	})

	if err == nil {
		t.Fatal("Evaluate() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "evidence_ids") {
		t.Fatalf("error = %q, want evidence_ids diagnostic", err)
	}
	want := domain.EvaluationResult{
		Verdict:  domain.VerdictIndeterminate,
		Decision: domain.DecisionBlocked,
	}
	if got != want {
		t.Fatalf("Evaluate() = %#v, want %#v", got, want)
	}
}
