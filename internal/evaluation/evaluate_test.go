package evaluation

import (
	"testing"
	"time"

	"github.com/kefyusuf/assurectl/internal/contract"
	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/inputmeta"
	"github.com/kefyusuf/assurectl/internal/policy"
)

var evaluationInstant = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

func evaluationDigest(seed string) inputmeta.Digest {
	return inputmeta.SHA256([]byte(seed))
}

func advisoryPolicy(requirements ...policy.Requirement) policy.Loaded {
	return policy.Loaded{
		Policy: policy.Policy{
			SchemaVersion: policy.SchemaVersion,
			Requirements:  requirements,
		},
		Digest:         evaluationDigest("policy"),
		TrustStatus:    inputmeta.TrustStatusUntrusted,
		AuthorityBasis: inputmeta.AuthorityBasisAdvisoryWorkspace,
	}
}

func advisoryContract(criteria ...contract.AcceptanceCriterion) contract.Loaded {
	return contract.Loaded{
		Contract: contract.Contract{
			SchemaVersion:      contract.SchemaVersion,
			AcceptanceCriteria: criteria,
		},
		Digest:      evaluationDigest("contract"),
		TrustStatus: inputmeta.TrustStatusUntrusted,
	}
}

func policyRequirement(id, evidenceType string) policy.Requirement {
	return policy.Requirement{
		ID:                   id,
		EvidenceType:         evidenceType,
		Waivable:             false,
		AllowedProducerTypes: []string{"local-user"},
	}
}

func validInput() Input {
	return Input{
		Contract:    advisoryContract(),
		Policy:      advisoryPolicy(policyRequirement("req-a", "test-result")),
		Evidence:    nil,
		EvaluatedAt: evaluationInstant,
	}
}

func TestEvaluateRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Input)
	}{
		{name: "zero evaluated at", mutate: func(in *Input) { in.EvaluatedAt = time.Time{} }},
		{name: "empty contract digest", mutate: func(in *Input) { in.Contract.Digest = inputmeta.Digest{} }},
		{name: "empty policy digest", mutate: func(in *Input) { in.Policy.Digest = inputmeta.Digest{} }},
		{
			name: "contract trust status not untrusted",
			mutate: func(in *Input) {
				in.Contract.TrustStatus = inputmeta.TrustStatus("CI_ASSERTED")
			},
		},
		{
			name: "policy trust status not untrusted",
			mutate: func(in *Input) {
				in.Policy.TrustStatus = inputmeta.TrustStatus("CI_ASSERTED")
			},
		},
		{
			name: "policy authority basis not advisory workspace",
			mutate: func(in *Input) {
				in.Policy.AuthorityBasis = inputmeta.AuthorityBasis("CANDIDATE_HEAD")
			},
		},
		{
			name: "contract schema version unsupported",
			mutate: func(in *Input) {
				in.Contract.Contract.SchemaVersion = "assurectl/verification-contract/v9"
			},
		},
		{
			name: "policy schema version unsupported",
			mutate: func(in *Input) {
				in.Policy.Policy.SchemaVersion = "assurectl/policy/v9"
			},
		},
		{
			name: "empty requirement id",
			mutate: func(in *Input) {
				in.Policy.Policy.Requirements = []policy.Requirement{policyRequirement("", "test-result")}
			},
		},
		{
			name: "non canonical requirement id",
			mutate: func(in *Input) {
				in.Policy.Policy.Requirements = []policy.Requirement{policyRequirement("bad id", "test-result")}
			},
		},
		{
			name: "duplicate requirement ids",
			mutate: func(in *Input) {
				in.Policy.Policy.Requirements = []policy.Requirement{
					policyRequirement("req-a", "test-result"),
					policyRequirement("req-a", "lint-result"),
				}
			},
		},
		{
			name: "criterion references empty requirement id",
			mutate: func(in *Input) {
				in.Contract.Contract.AcceptanceCriteria = []contract.AcceptanceCriterion{
					{ID: "crit-a", Statement: "everything passes", Requirements: []string{""}},
				}
			},
		},
		{
			name: "criterion references non canonical requirement id",
			mutate: func(in *Input) {
				in.Contract.Contract.AcceptanceCriteria = []contract.AcceptanceCriterion{
					{ID: "crit-a", Statement: "everything passes", Requirements: []string{"bad id"}},
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			input := validInput()
			tt.mutate(&input)
			if _, err := Evaluate(input); err == nil {
				t.Fatalf("Evaluate() = nil error, want rejection")
			}
		})
	}
}

func TestEvaluateAcceptsMinimalInput(t *testing.T) {
	t.Parallel()

	result, err := Evaluate(validInput())
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	if len(result.Requirements) != 1 {
		t.Fatalf("Requirements = %d entries, want 1", len(result.Requirements))
	}
	requirement := result.Requirements[0]
	if requirement.RequirementID != "req-a" {
		t.Fatalf("RequirementID = %q", requirement.RequirementID)
	}
	if requirement.EvidenceState != domain.EvidenceMissing {
		t.Fatalf("EvidenceState = %q, want MISSING", requirement.EvidenceState)
	}
	if len(requirement.EvidenceIDs) != 0 {
		t.Fatalf("EvidenceIDs = %#v, want empty", requirement.EvidenceIDs)
	}
	if requirement.Outcome != "" {
		t.Fatalf("Outcome = %q, want empty", requirement.Outcome)
	}
	if requirement.Waivable {
		t.Fatalf("Waivable = true, want false")
	}
	if requirement.WaiverStatus != domain.WaiverNotApplicable {
		t.Fatalf("WaiverStatus = %q, want NOT_APPLICABLE", requirement.WaiverStatus)
	}
}
