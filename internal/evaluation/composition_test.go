package evaluation

import (
	"testing"
	"time"

	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/evidence"
	"github.com/kefyusuf/assurectl/internal/policy"
)

func TestEvaluateComposition(t *testing.T) {
	t.Parallel()

	input := func(requirement policy.Requirement, entries ...evidence.Loaded) Input {
		return Input{
			Contract:    advisoryContract(referencedCriterion("req-a")),
			Policy:      advisoryPolicy(requirement),
			Evidence:    entries,
			EvaluatedAt: evaluationInstant,
		}
	}
	policyPassed := func(outcome domain.ObservedOutcome, entries ...evidence.Loaded) Input {
		requirement := policyRequirement("req-a", "test-result")
		for i := range entries {
			entries[i].Envelope.Outcome.Status = outcome
		}
		return input(requirement, entries...)
	}

	tests := []struct {
		name         string
		build        func() Input
		wantVerdict  domain.VerificationVerdict
		wantDecision domain.CompletionDecision
	}{
		{
			name:         "all passed approves",
			build:        func() Input { return policyPassed(domain.OutcomePassed, candidate("ev-a")) },
			wantVerdict:  domain.VerdictPassed,
			wantDecision: domain.DecisionApproved,
		},
		{
			name: "established failure rejects",
			build: func() Input {
				return policyPassed(domain.OutcomeFailed, candidate("ev-a"))
			},
			wantVerdict:  domain.VerdictFailed,
			wantDecision: domain.DecisionRejected,
		},
		{
			name:         "missing evidence blocks",
			build:        func() Input { return input(policyRequirement("req-a", "test-result")) },
			wantVerdict:  domain.VerdictIndeterminate,
			wantDecision: domain.DecisionBlocked,
		},
		{
			name: "stale evidence blocks",
			build: func() Input {
				return input(policyRequirement("req-a", "test-result"),
					evidenceEntry("ev-stale", "test-result", "local-user", evidence.SubjectBindingOtherRevision, evaluationInstant.Add(-time.Minute)),
				)
			},
			wantVerdict:  domain.VerdictIndeterminate,
			wantDecision: domain.DecisionBlocked,
		},
		{
			name: "untrusted producer blocks",
			build: func() Input {
				return input(policyRequirement("req-a", "test-result"),
					evidenceEntry("ev-untrusted", "test-result", "ci-runner", evidence.SubjectBindingExact, evaluationInstant.Add(-time.Minute)),
				)
			},
			wantVerdict:  domain.VerdictIndeterminate,
			wantDecision: domain.DecisionBlocked,
		},
		{
			name: "error outcome blocks",
			build: func() Input {
				return policyPassed(domain.OutcomeError, candidate("ev-a"))
			},
			wantVerdict:  domain.VerdictIndeterminate,
			wantDecision: domain.DecisionBlocked,
		},
		{
			name: "conflicting candidates block",
			build: func() Input {
				input := policyPassed(domain.OutcomePassed, candidate("ev-a"))
				input.Evidence = append(input.Evidence, conflictSibling("ev-b", domain.OutcomeFailed))
				return input
			},
			wantVerdict:  domain.VerdictIndeterminate,
			wantDecision: domain.DecisionBlocked,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := Evaluate(tt.build())
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if result.Verdict.Verdict != tt.wantVerdict || result.Verdict.Decision != tt.wantDecision {
				t.Fatalf("verdict/decision = (%q, %q), want (%q, %q)",
					result.Verdict.Verdict, result.Verdict.Decision, tt.wantVerdict, tt.wantDecision)
			}
			if result.Verdict.Decision == domain.DecisionAcceptedWithRisk {
				t.Fatalf("ACCEPTED_WITH_RISK must be unreachable in this slice")
			}
		})
	}
}

func candidate(id string) evidence.Loaded {
	entry := evidenceEntry(id, "test-result", "local-user", evidence.SubjectBindingExact, evaluationInstant.Add(-time.Minute))
	entry.Envelope.Outcome.Status = domain.OutcomePassed
	return entry
}

func conflictSibling(id string, outcome domain.ObservedOutcome) evidence.Loaded {
	entry := evidenceEntry(id, "test-result", "local-user", evidence.SubjectBindingExact, evaluationInstant.Add(-time.Minute))
	entry.Envelope.Outcome.Status = outcome
	return entry
}
