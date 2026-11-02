package receipt

import (
	"testing"

	"github.com/kefyusuf/assurectl/internal/domain"
)

func TestBuildRejectsTamperedVerdict(t *testing.T) {
	t.Parallel()

	input := buildInput()
	input.Evaluation.Verdict.Verdict = domain.VerdictFailed
	if _, err := Build(input); err == nil {
		t.Fatalf("Build() accepted a tampered verdict")
	}
}

func TestBuildRejectsTamperedDecision(t *testing.T) {
	t.Parallel()

	input := buildInput()
	input.Evaluation.Verdict.Decision = domain.DecisionAcceptedWithRisk
	if _, err := Build(input); err == nil {
		t.Fatalf("Build() accepted a tampered decision")
	}
}

func TestBuildRejectsTamperedDecisionBlockingFinding(t *testing.T) {
	t.Parallel()

	input := buildInput()
	input.Evaluation.Findings = append(input.Evaluation.Findings, domain.Finding{
		Code:     "EVIDENCE_MISSING",
		Category: domain.FindingCategoryEvidence,
		Severity: domain.SeverityError,
		Message:  "missing",
		Blocking: true,
	})
	if _, err := Build(input); err == nil {
		t.Fatalf("Build() accepted a decision ignoring a blocking finding")
	}
}
