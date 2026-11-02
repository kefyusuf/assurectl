package evaluation

import (
	"math/big"
	"testing"
	"time"

	"github.com/kefyusuf/assurectl/internal/contract"
	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/evidence"
	"github.com/kefyusuf/assurectl/internal/policy"
)

func TestEvidenceFindingsMissing(t *testing.T) {
	t.Parallel()

	input := Input{
		Contract:    advisoryContract(referencedCriterion("req-a")),
		Policy:      advisoryPolicy(policyRequirement("req-a", "test-result")),
		EvaluatedAt: evaluationInstant,
	}
	result, err := Evaluate(input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	assertFindings(t, result.Findings, findingExpectation{
		Code:          "EVIDENCE_MISSING",
		Category:      domain.FindingCategoryEvidence,
		Severity:      domain.SeverityError,
		Blocking:      true,
		RequirementID: "req-a",
	})
}

func TestEvidenceFindingsFacts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		requirement policy.Requirement
		entries     []evidence.Loaded
		want        findingExpectation
	}{
		{
			name:        "stale revision",
			requirement: policyRequirement("req-a", "test-result"),
			entries: []evidence.Loaded{
				evidenceEntry("ev-stale", "test-result", "local-user", evidence.SubjectBindingOtherRevision, evaluationInstant.Add(-time.Minute)),
			},
			want: evidenceFinding("EVIDENCE_STALE_REVISION", "req-a"),
		},
		{
			name:        "stale future",
			requirement: policyRequirement("req-a", "test-result"),
			entries: []evidence.Loaded{
				evidenceEntry("ev-future", "test-result", "local-user", evidence.SubjectBindingExact, evaluationInstant.Add(time.Minute)),
			},
			want: evidenceFinding("EVIDENCE_STALE_FUTURE", "req-a"),
		},
		{
			name: "stale age",
			requirement: policy.Requirement{
				ID:                   "req-a",
				EvidenceType:         "test-result",
				AllowedProducerTypes: []string{"local-user"},
				MaxAgeSeconds:        big.NewInt(60),
			},
			entries: []evidence.Loaded{
				evidenceEntry("ev-aged", "test-result", "local-user", evidence.SubjectBindingExact, evaluationInstant.Add(-2*time.Minute)),
			},
			want: evidenceFinding("EVIDENCE_STALE_AGE", "req-a"),
		},
		{
			name:        "untrusted producer",
			requirement: policyRequirement("req-a", "test-result"),
			entries: []evidence.Loaded{
				evidenceEntry("ev-untrusted", "test-result", "ci-runner", evidence.SubjectBindingExact, evaluationInstant.Add(-time.Minute)),
			},
			want: evidenceFinding("EVIDENCE_UNTRUSTED_PRODUCER", "req-a"),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			input := Input{
				Contract:    advisoryContract(referencedCriterion("req-a")),
				Policy:      advisoryPolicy(tt.requirement),
				Evidence:    tt.entries,
				EvaluatedAt: evaluationInstant,
			}
			result, err := Evaluate(input)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}

			// The requirement must be indeterminate-driving, and the
			// matching evidence finding must be present with the category,
			// severity, and the contributing envelope ID.
			assertFindings(t, result.Findings, tt.want)
		})
	}
}

func evidenceFinding(code, requirementID string) findingExpectation {
	return findingExpectation{
		Code:          code,
		Category:      domain.FindingCategoryEvidence,
		Severity:      domain.SeverityError,
		Blocking:      true,
		RequirementID: requirementID,
	}
}

func referencedCriterion(requirementID string) contract.AcceptanceCriterion {
	return contract.AcceptanceCriterion{
		ID:           "crit-a",
		Statement:    "the change is verified",
		Requirements: []string{requirementID},
	}
}
