package receipt

import (
	"testing"
	"time"

	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/evaluation"
	"github.com/kefyusuf/assurectl/internal/evidence"
)

func TestBuildRejectsNonAdvisoryAuthority(t *testing.T) {
	t.Parallel()

	input := buildInput()
	input.Authority = domain.AuthorityCIAsserted
	if _, err := Build(input); err == nil {
		t.Fatalf("Build() accepted a non-advisory authority")
	}
}

func TestBuildRejectsZeroEvaluatedAt(t *testing.T) {
	t.Parallel()

	input := buildInput()
	input.EvaluatedAt = time.Time{}
	if _, err := Build(input); err == nil {
		t.Fatalf("Build() accepted a zero evaluation instant")
	}
}

func TestBuildRejectsDuplicateEnvelopeIDs(t *testing.T) {
	t.Parallel()

	input := buildInput()
	input.Evidence = append(input.Evidence, buildEvidence("ev-b"))
	input.Evidence[1].Digest = seededDigest("other")
	if _, err := Build(input); err == nil {
		t.Fatalf("Build() accepted duplicate evidence ids")
	}
}

func TestBuildRejectsUnknownEvidenceReference(t *testing.T) {
	t.Parallel()

	input := buildInput()
	input.Evaluation.Requirements[0].EvidenceIDs = []string{"ev-ghost"}
	if _, err := Build(input); err == nil {
		t.Fatalf("Build() accepted an unknown evidence reference")
	}
}

func TestBuildRejectsValidWithoutEvidence(t *testing.T) {
	t.Parallel()

	input := buildInput()
	input.Evaluation.Requirements[0].EvidenceIDs = nil
	if _, err := Build(input); err == nil {
		t.Fatalf("Build() accepted a VALID result without evidence")
	}
}

func TestBuildRejectsBlockingFindingOnApproved(t *testing.T) {
	t.Parallel()

	input := buildInput()
	input.Evaluation.Findings = []domain.Finding{{
		Code:     "CONTRACT_UNKNOWN_REQUIREMENT",
		Category: domain.FindingCategoryContract,
		Severity: domain.SeverityError,
		Message:  "unresolved reference",
		Blocking: true,
	}}
	if _, err := Build(input); err == nil {
		t.Fatalf("Build() accepted a non-BLOCKED receipt with a blocking finding")
	}
}

func waivableFailedEvaluation() evaluation.Result {
	result := buildEvaluation()
	result.Requirements[0] = domain.RequirementResult{
		RequirementID: "req-a",
		EvidenceState: domain.EvidenceValid,
		EvidenceIDs:   []string{"ev-a"},
		Outcome:       domain.OutcomeFailed,
		Waivable:      true,
		WaiverStatus:  domain.WaiverNotApplicable,
	}
	result.Verdict = domain.EvaluationResult{
		Verdict:  domain.VerdictFailed,
		Decision: domain.DecisionRejected,
	}
	return result
}

func validWaiver(target string) WaiverRecord {
	return WaiverRecord{
		ID:           "wv-001",
		Target:       WaiverTarget{Kind: "REQUIREMENT", ID: target},
		Scope:        WaiverScope{RepositoryURI: "github.com/acme/checkout", WorkUnitID: "wu-001"},
		Reason:       "known flake",
		AcceptedRisk: "retry before merge",
		Approver:     WaiverApprover{Identity: "release-engineer", Authority: "RELEASE_MANAGER"},
		IssuedAt:     "2026-11-01T09:00:00Z",
		ExpiresAt:    "2026-12-01T09:00:00Z",
		Status:       domain.WaiverValid,
		Digest:       seededDigest("waiver"),
	}
}

func TestValidateReceiptRejectsWaiverTargetingUnknownRequirement(t *testing.T) {
	t.Parallel()

	built, err := Build(buildInput())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	built.Waivers = []WaiverRecord{validWaiver("req-ghost")}
	if err := validateReceipt(built); err == nil {
		t.Fatalf("validateReceipt() accepted a waiver with an unknown target")
	}
}

func TestValidateReceiptRejectsWaiverOnNonWaivable(t *testing.T) {
	t.Parallel()

	built, err := Build(buildInput())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	built.Waivers = []WaiverRecord{validWaiver("req-a")}
	if err := validateReceipt(built); err == nil {
		t.Fatalf("validateReceipt() accepted a waiver on a non-waivable requirement")
	}
}

func TestValidateReceiptAcceptsExactWaiverMatch(t *testing.T) {
	t.Parallel()

	input := buildInput()
	input.Evaluation = waivableFailedEvaluation()
	input.Evidence = []evidence.Loaded{buildEvidence("ev-a")}
	built, err := Build(input)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	built.Waivers = []WaiverRecord{validWaiver("req-a")}
	if err := validateReceipt(built); err != nil {
		t.Fatalf("validateReceipt() rejected an exact waiver match: %v", err)
	}
}

func TestValidateReceiptRejectsIncompleteVerifier(t *testing.T) {
	t.Parallel()

	built, err := Build(buildInput())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	built.Verifier.Version = ""
	if err := validateReceipt(built); err == nil {
		t.Fatalf("validateReceipt() accepted an incomplete verifier block")
	}
}

func TestValidateReceiptRejectsUnparseableEvaluatedAt(t *testing.T) {
	t.Parallel()

	built, err := Build(buildInput())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	built.EvaluatedAt = "not-a-timestamp"
	if err := validateReceipt(built); err == nil {
		t.Fatalf("validateReceipt() accepted an unparseable evaluated_at")
	}
}
