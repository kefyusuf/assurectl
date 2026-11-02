package evaluation

import (
	"reflect"
	"testing"
	"time"

	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/evidence"
	"github.com/kefyusuf/assurectl/internal/policy"
	"github.com/kefyusuf/assurectl/internal/waiver"
)

func failedCandidate(id string) evidence.Loaded {
	entry := evidenceEntry(id, "test-result", "local-user", evidence.SubjectBindingExact, evaluationInstant.Add(-time.Minute))
	entry.Envelope.Outcome.Status = domain.OutcomeFailed
	return entry
}

func subjectFixture() domain.Subject {
	return domain.Subject{
		RepositoryURI: "github.com/acme/checkout",
		BaseRevision:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		HeadRevision:  "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
}

func waivableInput() Input {
	input := Input{
		Contract:    advisoryContract(referencedCriterion("req-a")),
		Policy:      advisoryPolicy(waivableRequirement()),
		Evidence:    []evidence.Loaded{failedCandidate("ev-a")},
		EvaluatedAt: evaluationInstant,
		Subject:     subjectFixture(),
	}
	return input
}

func waivableRequirement() policy.Requirement {
	requirement := policyRequirement("req-a", "test-result")
	requirement.Waivable = true
	return requirement
}

func matchingWaiver() waiver.Record {
	record := waiver.Record{
		ID:           "wv-001",
		Target:       waiver.Target{Kind: "REQUIREMENT", ID: "req-a"},
		Scope:        waiver.Scope{RepositoryURI: "github.com/acme/checkout", WorkUnitID: "wu-001"},
		Reason:       "known flaky test",
		AcceptedRisk: "retry before merge",
		Approver:     waiver.Approver{Identity: "release-engineer", Authority: "RELEASE_MANAGER"},
		IssuedAt:     "2026-11-01T00:00:00Z",
		ExpiresAt:    "2026-12-01T00:00:00Z",
		Status:       domain.WaiverValid,
	}
	return record
}

func TestApplyWaiversSetsValidStatusAndAppliedFinding(t *testing.T) {
	t.Parallel()

	input := waivableInput()
	input.Waivers = []waiver.Record{matchingWaiver()}

	result, err := Evaluate(input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	requirement := result.Requirements[0]
	if requirement.WaiverStatus != domain.WaiverValid {
		t.Fatalf("WaiverStatus = %q, want VALID", requirement.WaiverStatus)
	}
	if requirement.EvidenceState != domain.EvidenceValid || requirement.Outcome != domain.OutcomeFailed {
		t.Fatalf("facts mutated: state=%q outcome=%q", requirement.EvidenceState, requirement.Outcome)
	}
	if result.Verdict.Verdict != domain.VerdictFailed || result.Verdict.Decision != domain.DecisionAcceptedWithRisk {
		t.Fatalf("verdict/decision = (%q, %q), want (FAILED, ACCEPTED_WITH_RISK)",
			result.Verdict.Verdict, result.Verdict.Decision)
	}

	applied := false
	for _, finding := range result.Findings {
		if finding.Code == "WAIVER_APPLIED" && !finding.Blocking && finding.RequirementID == "req-a" {
			applied = true
		}
	}
	if !applied {
		t.Fatalf("WAIVER_APPLIED finding missing: %#v", result.Findings)
	}
	if len(result.Waivers) != 1 || result.Waivers[0].Status != domain.WaiverValid {
		t.Fatalf("resolved waivers = %#v, want the applied record as VALID", result.Waivers)
	}
}

func TestApplyWaiversSupersedesLaterDuplicates(t *testing.T) {
	t.Parallel()

	input := waivableInput()
	first := matchingWaiver()
	second := matchingWaiver()
	second.ID = "wv-002"
	input.Waivers = []waiver.Record{first, second}

	result, err := Evaluate(input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if result.Waivers[0].Status != domain.WaiverValid || result.Waivers[1].Status != domain.WaiverInvalid {
		t.Fatalf("resolved statuses = %q, %q; want VALID then INVALID", result.Waivers[0].Status, result.Waivers[1].Status)
	}
	superseded := false
	for _, finding := range result.Findings {
		if finding.Code == "WAIVER_SUPERSEDED" {
			superseded = true
		}
	}
	if !superseded {
		t.Fatalf("WAIVER_SUPERSEDED finding missing: %#v", result.Findings)
	}
}

func TestApplyWaiversRejectionTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mutate   func(*waiver.Record)
		policy   policy.Requirement
		outcome  domain.ObservedOutcome
		wantCode string
	}{
		{
			name:     "unknown requirement target",
			mutate:   func(r *waiver.Record) { r.Target.ID = "req-ghost" },
			policy:   waivableRequirement(),
			outcome:  domain.OutcomeFailed,
			wantCode: "WAIVER_REJECTED_TARGET",
		},
		{
			name:     "non waivable failure",
			mutate:   func(r *waiver.Record) {},
			policy:   policyRequirement("req-a", "test-result"),
			outcome:  domain.OutcomeFailed,
			wantCode: "WAIVER_REJECTED_TARGET",
		},
		{
			name:     "requirement did not fail",
			mutate:   func(r *waiver.Record) {},
			policy:   waivableRequirement(),
			outcome:  domain.OutcomePassed,
			wantCode: "WAIVER_REJECTED_TARGET",
		},
		{
			name:     "repository scope mismatch",
			mutate:   func(r *waiver.Record) { r.Scope.RepositoryURI = "github.com/other/repo" },
			policy:   waivableRequirement(),
			outcome:  domain.OutcomeFailed,
			wantCode: "WAIVER_REJECTED_SCOPE",
		},
		{
			name:     "work unit scope mismatch",
			mutate:   func(r *waiver.Record) { r.Scope.WorkUnitID = "wu-other" },
			policy:   waivableRequirement(),
			outcome:  domain.OutcomeFailed,
			wantCode: "WAIVER_REJECTED_SCOPE",
		},
		{
			name:     "expired before evaluation",
			mutate:   func(r *waiver.Record) { r.ExpiresAt = "2026-10-01T00:00:00Z" },
			policy:   waivableRequirement(),
			outcome:  domain.OutcomeFailed,
			wantCode: "WAIVER_REJECTED_EXPIRED",
		},
		{
			name:     "issued after evaluation",
			mutate:   func(r *waiver.Record) { r.IssuedAt = "2026-12-01T00:00:00Z" },
			policy:   waivableRequirement(),
			outcome:  domain.OutcomeFailed,
			wantCode: "WAIVER_REJECTED_TIMING",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			entry := failedCandidate("ev-a")
			if tt.outcome == domain.OutcomePassed {
				entry = candidate("ev-a")
			}
			input := Input{
				Contract:    advisoryContract(referencedCriterion("req-a")),
				Policy:      advisoryPolicy(tt.policy),
				Evidence:    []evidence.Loaded{entry},
				EvaluatedAt: evaluationInstant,
				Subject:     subjectFixture(),
				Waivers:     []waiver.Record{mutatedWaiver(tt.mutate)},
			}

			result, err := Evaluate(input)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			requirement := result.Requirements[0]
			if requirement.WaiverStatus != domain.WaiverNotApplicable {
				t.Fatalf("WaiverStatus = %q, want NOT_APPLICABLE after rejection", requirement.WaiverStatus)
			}
			wantDecision := domain.DecisionRejected
			if tt.outcome == domain.OutcomePassed {
				wantDecision = domain.DecisionApproved
			}
			if result.Verdict.Decision != wantDecision {
				t.Fatalf("Decision = %q, want %q after rejection", result.Verdict.Decision, wantDecision)
			}
			rejected := false
			for _, finding := range result.Findings {
				if finding.Code == tt.wantCode && !finding.Blocking {
					rejected = true
				}
			}
			if !rejected {
				t.Fatalf("%s finding missing: %#v", tt.wantCode, result.Findings)
			}
			if len(result.Waivers) != 1 || result.Waivers[0].Status != domain.WaiverInvalid {
				t.Fatalf("resolved waiver = %#v, want INVALID", result.Waivers)
			}
		})
	}
}

func mutatedWaiver(mutate func(*waiver.Record)) waiver.Record {
	record := matchingWaiver()
	mutate(&record)
	return record
}

func TestApplyWaiversRequiresSubjectWhenWaiversPresent(t *testing.T) {
	t.Parallel()

	input := waivableInput()
	input.Waivers = []waiver.Record{matchingWaiver()}
	input.Subject = domain.Subject{}
	if _, err := Evaluate(input); err == nil {
		t.Fatalf("Evaluate() accepted waivers without a subject")
	}
}

func TestApplyWaiversDeterministicUnderShuffle(t *testing.T) {
	t.Parallel()

	first := matchingWaiver()
	second := matchingWaiver()
	second.ID = "wv-002"
	second.Target.ID = "req-a"

	build := func(records []waiver.Record) Input {
		input := waivableInput()
		input.Waivers = records
		return input
	}

	forward, err := Evaluate(build([]waiver.Record{first, second}))
	if err != nil {
		t.Fatalf("Evaluate(forward) error = %v", err)
	}
	// The same single target cannot be waived twice; only the order of the
	// resolved facts may differ, never the decision or statuses.
	backward, err := Evaluate(build([]waiver.Record{second, first}))
	if err != nil {
		t.Fatalf("Evaluate(backward) error = %v", err)
	}
	if forward.Verdict != backward.Verdict {
		t.Fatalf("verdict changed with waiver input order: %#v vs %#v", forward.Verdict, backward.Verdict)
	}
	if !reflect.DeepEqual(forward.Requirements, backward.Requirements) {
		t.Fatalf("requirement results changed with waiver input order")
	}
}
