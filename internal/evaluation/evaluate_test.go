package evaluation

import (
	"math/big"
	"testing"
	"time"

	"github.com/kefyusuf/assurectl/internal/contract"
	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/evidence"
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

func TestEvaluateSurfacesContractCrossChecks(t *testing.T) {
	t.Parallel()

	t.Run("unknown criterion reference blocks without dropping requirements", func(t *testing.T) {
		t.Parallel()

		input := validInput()
		input.Contract.Contract.AcceptanceCriteria = []contract.AcceptanceCriterion{
			{ID: "crit-a", Statement: "the change is verified", Requirements: []string{"req-unknown"}},
		}

		result, err := Evaluate(input)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		if len(result.Requirements) != 1 || result.Requirements[0].RequirementID != "req-a" {
			t.Fatalf("Requirements = %#v, want req-a still evaluated", result.Requirements)
		}
		if result.Requirements[0].EvidenceState != domain.EvidenceMissing {
			t.Fatalf("EvidenceState = %q, want MISSING", result.Requirements[0].EvidenceState)
		}

		assertFindings(t, result.Findings,
			findingExpectation{
				Code:          "CONTRACT_UNKNOWN_REQUIREMENT",
				Category:      domain.FindingCategoryContract,
				Severity:      domain.SeverityError,
				Blocking:      true,
				RequirementID: "req-unknown",
			},
			findingExpectation{
				Code:          "EVIDENCE_MISSING",
				Category:      domain.FindingCategoryEvidence,
				Severity:      domain.SeverityError,
				Blocking:      true,
				RequirementID: "req-a",
			},
			findingExpectation{
				Code:          "POLICY_REQUIREMENT_UNREFERENCED",
				Category:      domain.FindingCategoryPolicy,
				Severity:      domain.SeverityInfo,
				Blocking:      false,
				RequirementID: "req-a",
			},
		)
	})

	t.Run("referenced requirement produces no unreferenced finding", func(t *testing.T) {
		t.Parallel()

		input := Input{
			Contract: advisoryContract(contract.AcceptanceCriterion{
				ID:           "crit-a",
				Statement:    "the change is verified",
				Requirements: []string{"req-a"},
			}),
			Policy:      advisoryPolicy(policyRequirement("req-a", "test-result")),
			EvaluatedAt: evaluationInstant,
		}

		result, err := Evaluate(input)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		assertFindings(t, result.Findings,
			findingExpectation{
				Code:          "EVIDENCE_MISSING",
				Category:      domain.FindingCategoryEvidence,
				Severity:      domain.SeverityError,
				Blocking:      true,
				RequirementID: "req-a",
			},
		)
	})

	t.Run("criteria with empty requirement lists produce nothing", func(t *testing.T) {
		t.Parallel()

		input := validInput()
		input.Contract.Contract.AcceptanceCriteria = []contract.AcceptanceCriterion{
			{ID: "crit-a", Statement: "manually verified", Requirements: nil},
		}

		result, err := Evaluate(input)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		assertFindings(t, result.Findings,
			findingExpectation{
				Code:          "EVIDENCE_MISSING",
				Category:      domain.FindingCategoryEvidence,
				Severity:      domain.SeverityError,
				Blocking:      true,
				RequirementID: "req-a",
			},
			findingExpectation{
				Code:          "POLICY_REQUIREMENT_UNREFERENCED",
				Category:      domain.FindingCategoryPolicy,
				Severity:      domain.SeverityInfo,
				Blocking:      false,
				RequirementID: "req-a",
			},
		)
	})
}

type findingExpectation struct {
	Code          string
	Category      domain.FindingCategory
	Severity      domain.FindingSeverity
	Blocking      bool
	RequirementID string
}

// assertFindings requires findings to match the expectations exactly, in order.
func assertFindings(t *testing.T, findings []domain.Finding, expected ...findingExpectation) {
	t.Helper()

	if len(findings) != len(expected) {
		t.Fatalf("Findings = %#v, want %d entries", findings, len(expected))
	}
	for i := range expected {
		got := findings[i]
		want := expected[i]
		if got.Code != want.Code ||
			got.Category != want.Category ||
			got.Severity != want.Severity ||
			got.Blocking != want.Blocking ||
			got.RequirementID != want.RequirementID {
			t.Fatalf("Findings[%d] = (code=%q category=%q severity=%q blocking=%t requirement=%q), want (code=%q category=%q severity=%q blocking=%t requirement=%q)",
				i, got.Code, got.Category, got.Severity, got.Blocking, got.RequirementID,
				want.Code, want.Category, want.Severity, want.Blocking, want.RequirementID)
		}
	}
}

func TestMatchEnvelopes(t *testing.T) {
	t.Parallel()

	envelopes := []evidence.Loaded{
		{Envelope: evidence.Envelope{ID: "ev-a", Type: "test-result"}},
		{Envelope: evidence.Envelope{ID: "ev-case", Type: "Test-Result"}},
		{Envelope: evidence.Envelope{ID: "ev-lint", Type: "lint-result"}},
	}

	tests := []struct {
		name         string
		evidenceType string
		want         []string
	}{
		{name: "exact match", evidenceType: "test-result", want: []string{"ev-a"}},
		{name: "no case folding", evidenceType: "Test-Result", want: []string{"ev-case"}},
		{name: "no cross type leakage", evidenceType: "lint-result", want: []string{"ev-lint"}},
		{name: "no match", evidenceType: "deploy-result", want: nil},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			matches := matchEnvelopes(tt.evidenceType, envelopes)
			ids := make([]string, 0, len(matches))
			for _, index := range matches {
				ids = append(ids, envelopes[index].Envelope.ID)
			}
			if len(ids) != len(tt.want) {
				t.Fatalf("matched = %v, want %v", ids, tt.want)
			}
			for i := range tt.want {
				if ids[i] != tt.want[i] {
					t.Fatalf("matched = %v, want %v", ids, tt.want)
				}
			}
		})
	}

	t.Run("one envelope satisfies two same type requirements", func(t *testing.T) {
		t.Parallel()

		first := matchEnvelopes("test-result", envelopes)
		second := matchEnvelopes("test-result", envelopes)
		if len(first) != 1 || len(second) != 1 || first[0] != second[0] {
			t.Fatalf("matches = %v, %v; want the same envelope for both requirements", first, second)
		}
	})
}

func evidenceEntry(id, envelopeType, producerType string, binding evidence.SubjectBinding, finishedAt time.Time) evidence.Loaded {
	return evidence.Loaded{
		Envelope: evidence.Envelope{
			ID:       id,
			Type:     envelopeType,
			Producer: evidence.Producer{Type: producerType},
		},
		FinishedAt:     finishedAt,
		SubjectBinding: binding,
	}
}

func TestDeriveFact(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mutate   func(*policy.Requirement)
		build    func() evidence.Loaded
		wantFact envelopeFact
	}{
		{
			name:   "exact binding allowed producer fresh age",
			mutate: func(*policy.Requirement) {},
			build: func() evidence.Loaded {
				return evidenceEntry("ev-a", "test-result", "local-user", evidence.SubjectBindingExact, evaluationInstant.Add(-time.Minute))
			},
			wantFact: factCandidate,
		},
		{
			name:   "other revision binds stale regardless of everything else",
			mutate: func(*policy.Requirement) {},
			build: func() evidence.Loaded {
				return evidenceEntry("ev-a", "test-result", "disallowed", evidence.SubjectBindingOtherRevision, evaluationInstant.Add(time.Hour))
			},
			wantFact: factStaleRevision,
		},
		{
			name:   "finished after evaluation instant is stale future",
			mutate: func(*policy.Requirement) {},
			build: func() evidence.Loaded {
				return evidenceEntry("ev-a", "test-result", "local-user", evidence.SubjectBindingExact, evaluationInstant.Add(time.Nanosecond))
			},
			wantFact: factStaleFuture,
		},
		{
			name:   "age exactly at the limit stays candidate",
			mutate: func(requirement *policy.Requirement) { requirement.MaxAgeSeconds = big.NewInt(60) },
			build: func() evidence.Loaded {
				return evidenceEntry("ev-a", "test-result", "local-user", evidence.SubjectBindingExact, evaluationInstant.Add(-60*time.Second))
			},
			wantFact: factCandidate,
		},
		{
			name:   "age one nanosecond beyond the limit is stale age",
			mutate: func(requirement *policy.Requirement) { requirement.MaxAgeSeconds = big.NewInt(60) },
			build: func() evidence.Loaded {
				return evidenceEntry("ev-a", "test-result", "local-user", evidence.SubjectBindingExact, evaluationInstant.Add(-60*time.Second).Add(-time.Nanosecond))
			},
			wantFact: factStaleAge,
		},
		{
			name:   "no age limit never goes stale by age",
			mutate: func(requirement *policy.Requirement) { requirement.MaxAgeSeconds = nil },
			build: func() evidence.Loaded {
				return evidenceEntry("ev-a", "test-result", "local-user", evidence.SubjectBindingExact, evaluationInstant.Add(-365*24*time.Hour))
			},
			wantFact: factCandidate,
		},
		{
			name:   "empty allowlist permits no producer",
			mutate: func(requirement *policy.Requirement) { requirement.AllowedProducerTypes = nil },
			build: func() evidence.Loaded {
				return evidenceEntry("ev-a", "test-result", "local-user", evidence.SubjectBindingExact, evaluationInstant.Add(-time.Minute))
			},
			wantFact: factUntrustedProducer,
		},
		{
			name:   "disallowed producer type is untrusted",
			mutate: func(*policy.Requirement) {},
			build: func() evidence.Loaded {
				return evidenceEntry("ev-a", "test-result", "ci-runner", evidence.SubjectBindingExact, evaluationInstant.Add(-time.Minute))
			},
			wantFact: factUntrustedProducer,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requirement := policyRequirement("req-a", "test-result")
			tt.mutate(&requirement)
			entry := tt.build()
			if got := deriveFact(requirement, entry, evaluationInstant); got != tt.wantFact {
				t.Fatalf("deriveFact() = %q, want %q", got, tt.wantFact)
			}
		})
	}
}
