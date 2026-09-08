package decision

import (
	"strings"
	"testing"

	"github.com/kefyusuf/assurectl/internal/domain"
)

func TestEvaluateRejectsDuplicateRequirementIDs(t *testing.T) {
	t.Parallel()

	requirements := []domain.RequirementResult{
		{
			RequirementID: "unit-tests",
			EvidenceState: domain.EvidenceValid,
			EvidenceIDs:   []string{"ev-unit-tests-a"},
			Outcome:       domain.OutcomePassed,
			WaiverStatus:  domain.WaiverNotApplicable,
		},
		{
			RequirementID: "unit-tests",
			EvidenceState: domain.EvidenceValid,
			EvidenceIDs:   []string{"ev-unit-tests-b"},
			Outcome:       domain.OutcomePassed,
			WaiverStatus:  domain.WaiverNotApplicable,
		},
	}

	got, err := Evaluate(requirements)
	if err == nil {
		t.Fatal("Evaluate() error = nil, want duplicate requirement_id error")
	}
	if !strings.Contains(err.Error(), "requirement_id") {
		t.Fatalf("error = %q, want requirement_id", err)
	}
	want := domain.EvaluationResult{Verdict: domain.VerdictIndeterminate, Decision: domain.DecisionBlocked}
	if got != want {
		t.Fatalf("Evaluate() = %#v, want %#v", got, want)
	}
}

func TestEvaluateRejectsMalformedFindingReferences(t *testing.T) {
	t.Parallel()

	requirement := domain.RequirementResult{
		RequirementID: "unit-tests",
		EvidenceState: domain.EvidenceValid,
		EvidenceIDs:   []string{"ev-unit-tests"},
		Outcome:       domain.OutcomePassed,
		WaiverStatus:  domain.WaiverNotApplicable,
	}

	base := domain.Finding{
		Code:     "ASR-INFO-001",
		Category: domain.FindingCategoryVerification,
		Severity: domain.SeverityInfo,
		Message:  "informational finding",
		Blocking: false,
	}

	tests := []struct {
		name      string
		finding   domain.Finding
		wantField string
	}{
		{
			name: "non-canonical requirement reference",
			finding: func() domain.Finding {
				got := base
				got.RequirementID = "unit tests"
				return got
			}(),
			wantField: "requirement_id",
		},
		{
			name: "path-like evidence reference",
			finding: func() domain.Finding {
				got := base
				got.EvidenceIDs = []string{"../evidence"}
				return got
			}(),
			wantField: "evidence_ids",
		},
		{
			name: "duplicate evidence reference",
			finding: func() domain.Finding {
				got := base
				got.EvidenceIDs = []string{"ev-unit-tests", "ev-unit-tests"}
				return got
			}(),
			wantField: "evidence_ids",
		},
	}

	want := domain.EvaluationResult{Verdict: domain.VerdictIndeterminate, Decision: domain.DecisionBlocked}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Evaluate([]domain.RequirementResult{requirement}, tt.finding)
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
