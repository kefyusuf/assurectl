package evaluation

import (
	"testing"
	"time"

	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/evidence"
	"github.com/kefyusuf/assurectl/internal/policy"
)

func TestEvaluateNormalize(t *testing.T) {
	t.Parallel()

	input := Input{
		Contract: advisoryContract(),
		Policy: advisoryPolicy(
			policyRequirement("req-a", "test-result"),
			policy.Requirement{
				ID:                   "req-b",
				EvidenceType:         "lint-result",
				Waivable:             true,
				AllowedProducerTypes: []string{"local-user"},
			},
		),
		Evidence: []evidence.Loaded{
			evidenceEntry("ev-a", "test-result", "local-user", evidence.SubjectBindingExact, evaluationInstant.Add(-time.Minute)),
			evidenceEntry("ev-z", "lint-result", "ci-runner", evidence.SubjectBindingOtherRevision, evaluationInstant.Add(-time.Minute)),
		},
		EvaluatedAt: evaluationInstant,
	}

	result, err := Evaluate(input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if len(result.Requirements) != 2 {
		t.Fatalf("Requirements = %d entries, want 2", len(result.Requirements))
	}

	a := result.Requirements[0]
	if a.RequirementID != "req-a" || a.EvidenceState != domain.EvidenceValid || a.Outcome != domain.OutcomePassed {
		t.Fatalf("req-a = (id=%q state=%q outcome=%q), want (VALID PASSED)", a.RequirementID, a.EvidenceState, a.Outcome)
	}
	if len(a.EvidenceIDs) != 1 || a.EvidenceIDs[0] != "ev-a" {
		t.Fatalf("req-a EvidenceIDs = %#v, want [ev-a]", a.EvidenceIDs)
	}

	b := result.Requirements[1]
	if b.RequirementID != "req-b" || b.EvidenceState != domain.EvidenceStale || b.Outcome != "" {
		t.Fatalf("req-b = (id=%q state=%q outcome=%q), want (STALE no outcome)", b.RequirementID, b.EvidenceState, b.Outcome)
	}
	if len(b.EvidenceIDs) != 1 || b.EvidenceIDs[0] != "ev-z" {
		t.Fatalf("req-b EvidenceIDs = %#v, want [ev-z]", b.EvidenceIDs)
	}
	if !b.Waivable || b.WaiverStatus != domain.WaiverNotApplicable {
		t.Fatalf("req-b waiver fields = (%t, %q), want (true, NOT_APPLICABLE)", b.Waivable, b.WaiverStatus)
	}
}

func TestEvaluateNormalizeAggregation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		outcomes    []domain.ObservedOutcome
		wantState   domain.EvidenceState
		wantOutcome domain.ObservedOutcome
	}{
		{name: "all passed", outcomes: []domain.ObservedOutcome{domain.OutcomePassed, domain.OutcomePassed}, wantState: domain.EvidenceValid, wantOutcome: domain.OutcomePassed},
		{name: "any error dominates", outcomes: []domain.ObservedOutcome{domain.OutcomePassed, domain.OutcomeError}, wantState: domain.EvidenceValid, wantOutcome: domain.OutcomeError},
		{name: "error dominates established failure", outcomes: []domain.ObservedOutcome{domain.OutcomeFailed, domain.OutcomeError}, wantState: domain.EvidenceValid, wantOutcome: domain.OutcomeError},
		{name: "consistent failures", outcomes: []domain.ObservedOutcome{domain.OutcomeFailed, domain.OutcomeFailed}, wantState: domain.EvidenceValid, wantOutcome: domain.OutcomeFailed},
		{
			name:        "conflicting pass and fail collapse to invalid",
			outcomes:    []domain.ObservedOutcome{domain.OutcomeFailed, domain.OutcomePassed, domain.OutcomeFailed},
			wantState:   domain.EvidenceInvalid,
			wantOutcome: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			evidenceSet := make([]evidence.Loaded, 0, len(tt.outcomes))
			for i, outcome := range tt.outcomes {
				entry := evidenceEntry(
					"ev-"+string(rune('a'+i)),
					"test-result",
					"local-user",
					evidence.SubjectBindingExact,
					evaluationInstant.Add(-time.Minute),
				)
				entry.Envelope.Outcome.Status = outcome
				evidenceSet = append(evidenceSet, entry)
			}

			input := Input{
				Contract:    advisoryContract(),
				Policy:      advisoryPolicy(policyRequirement("req-a", "test-result")),
				Evidence:    evidenceSet,
				EvaluatedAt: evaluationInstant,
			}
			result, err := Evaluate(input)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			requirement := result.Requirements[0]
			if requirement.EvidenceState != tt.wantState || requirement.Outcome != tt.wantOutcome {
				t.Fatalf("state/outcome = (%q, %q), want (%q, %q)", requirement.EvidenceState, requirement.Outcome, tt.wantState, tt.wantOutcome)
			}
		})
	}
}

func TestEvaluateNormalizeStaleOverUntrusted(t *testing.T) {
	t.Parallel()

	input := Input{
		Contract: advisoryContract(),
		Policy:   advisoryPolicy(policyRequirement("req-a", "test-result")),
		Evidence: []evidence.Loaded{
			evidenceEntry("ev-stale", "test-result", "local-user", evidence.SubjectBindingOtherRevision, evaluationInstant.Add(-time.Minute)),
			evidenceEntry("ev-untrusted", "test-result", "ci-runner", evidence.SubjectBindingExact, evaluationInstant.Add(-time.Minute)),
		},
		EvaluatedAt: evaluationInstant,
	}

	result, err := Evaluate(input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	requirement := result.Requirements[0]
	if requirement.EvidenceState != domain.EvidenceStale {
		t.Fatalf("EvidenceState = %q, want STALE over UNTRUSTED", requirement.EvidenceState)
	}
	if len(requirement.EvidenceIDs) != 1 || requirement.EvidenceIDs[0] != "ev-stale" {
		t.Fatalf("EvidenceIDs = %#v, want only the stale envelope", requirement.EvidenceIDs)
	}
}

func TestEvaluateNormalizeUntrustedOnly(t *testing.T) {
	t.Parallel()

	input := Input{
		Contract: advisoryContract(),
		Policy:   advisoryPolicy(policyRequirement("req-a", "test-result")),
		Evidence: []evidence.Loaded{
			evidenceEntry("ev-u1", "test-result", "ci-runner", evidence.SubjectBindingExact, evaluationInstant.Add(-time.Minute)),
			evidenceEntry("ev-u2", "test-result", "ci-runner", evidence.SubjectBindingExact, evaluationInstant.Add(-time.Minute)),
		},
		EvaluatedAt: evaluationInstant,
	}

	result, err := Evaluate(input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	requirement := result.Requirements[0]
	if requirement.EvidenceState != domain.EvidenceUntrusted {
		t.Fatalf("EvidenceState = %q, want UNTRUSTED", requirement.EvidenceState)
	}
	if len(requirement.EvidenceIDs) != 2 {
		t.Fatalf("EvidenceIDs = %#v, want both untrusted envelopes", requirement.EvidenceIDs)
	}
}

func TestEvaluateNormalizeResultOrderFollowsPolicy(t *testing.T) {
	t.Parallel()

	input := Input{
		Contract: advisoryContract(),
		Policy: advisoryPolicy(
			policyRequirement("req-z", "test-result"),
			policyRequirement("req-a", "test-result"),
		),
		EvaluatedAt: evaluationInstant,
	}

	result, err := Evaluate(input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if result.Requirements[0].RequirementID != "req-z" || result.Requirements[1].RequirementID != "req-a" {
		t.Fatalf("result order = [%q, %q], want policy order [req-z, req-a]",
			result.Requirements[0].RequirementID, result.Requirements[1].RequirementID)
	}
}

func TestEvaluateNormalizeEvidenceIDsSorted(t *testing.T) {
	t.Parallel()

	// Ingestion order (by file name) differs from envelope ID order on
	// purpose: candidates are consumed in ingestion order, but the emitted
	// EvidenceIDs are sorted.
	input := Input{
		Contract: advisoryContract(),
		Policy:   advisoryPolicy(policyRequirement("req-a", "test-result")),
		Evidence: []evidence.Loaded{
			evidenceEntry("ev-zulu", "test-result", "local-user", evidence.SubjectBindingExact, evaluationInstant.Add(-time.Minute)),
			evidenceEntry("ev-alpha", "test-result", "local-user", evidence.SubjectBindingExact, evaluationInstant.Add(-time.Minute)),
		},
		EvaluatedAt: evaluationInstant,
	}

	result, err := Evaluate(input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	requirement := result.Requirements[0]
	if len(requirement.EvidenceIDs) != 2 || requirement.EvidenceIDs[0] != "ev-alpha" || requirement.EvidenceIDs[1] != "ev-zulu" {
		t.Fatalf("EvidenceIDs = %#v, want sorted [ev-alpha, ev-zulu]", requirement.EvidenceIDs)
	}
	if requirement.EvidenceState != domain.EvidenceValid || requirement.Outcome != domain.OutcomePassed {
		t.Fatalf("state/outcome = (%q, %q), want (VALID, PASSED)", requirement.EvidenceState, requirement.Outcome)
	}
}
