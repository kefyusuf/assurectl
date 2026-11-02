package receipt

import (
	"testing"

	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/evaluation"
	"github.com/kefyusuf/assurectl/internal/evidence"
	"github.com/kefyusuf/assurectl/internal/waiver"
)

func waivableBuildInput() BuildInput {
	input := buildInput()
	input.Evaluation = evaluation.Result{
		Requirements: []domain.RequirementResult{
			{
				RequirementID: "req-a",
				EvidenceState: domain.EvidenceValid,
				EvidenceIDs:   []string{"ev-a"},
				Outcome:       domain.OutcomeFailed,
				Waivable:      true,
				WaiverStatus:  domain.WaiverValid,
			},
		},
		Findings: []domain.Finding{},
		Waivers:  []waiver.Record{appliedWaiver()},
		Verdict: domain.EvaluationResult{
			Verdict:  domain.VerdictFailed,
			Decision: domain.DecisionAcceptedWithRisk,
		},
	}
	input.Evidence = []evidence.Loaded{buildEvidence("ev-a")}
	return input
}

func appliedWaiver() waiver.Record {
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
	record.Digest = waiver.CanonicalDigest(record)
	return record
}

func TestBuildEmitsWaiverRecords(t *testing.T) {
	t.Parallel()

	built, err := Build(waivableBuildInput())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(built.Waivers) != 1 {
		t.Fatalf("Waivers = %d entries, want 1", len(built.Waivers))
	}
	record := built.Waivers[0]
	if record.ID != "wv-001" || record.Status != domain.WaiverValid {
		t.Fatalf("Waiver = (id=%q status=%q), want (wv-001 VALID)", record.ID, record.Status)
	}
	if record.Digest != appliedWaiver().Digest {
		t.Fatalf("Waiver digest = %#v, want the canonical record digest", record.Digest)
	}
}

func TestBuildAcceptsAcceptedWithRiskReceipt(t *testing.T) {
	t.Parallel()

	if _, err := Build(waivableBuildInput()); err != nil {
		t.Fatalf("Build() rejected an all-waivered failure receipt: %v", err)
	}
}

func TestBuildKeepsRejectedDecisionWithInvalidWaiver(t *testing.T) {
	t.Parallel()

	input := waivableBuildInput()
	rejected := appliedWaiver()
	rejected.Status = domain.WaiverInvalid
	input.Evaluation.Waivers = []waiver.Record{rejected}
	input.Evaluation.Requirements[0].WaiverStatus = domain.WaiverNotApplicable
	input.Evaluation.Verdict = domain.EvaluationResult{
		Verdict:  domain.VerdictFailed,
		Decision: domain.DecisionRejected,
	}

	built, err := Build(input)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if built.CompletionDecision != domain.DecisionRejected {
		t.Fatalf("Decision = %q, want REJECTED", built.CompletionDecision)
	}
	if built.Waivers[0].Status != domain.WaiverInvalid {
		t.Fatalf("resolved waiver status = %q, want INVALID", built.Waivers[0].Status)
	}
}
