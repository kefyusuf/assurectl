package evaluation

import (
	"reflect"
	"testing"
	"time"

	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/evidence"
	"github.com/kefyusuf/assurectl/internal/inputmeta"
	"github.com/kefyusuf/assurectl/internal/policy"
)

func TestSecurityFutureDatedEvidenceBlocks(t *testing.T) {
	t.Parallel()

	input := Input{
		Contract: advisoryContract(referencedCriterion("req-a")),
		Policy:   advisoryPolicy(policyRequirement("req-a", "test-result")),
		Evidence: []evidence.Loaded{
			evidenceEntry("ev-future", "test-result", "local-user", evidence.SubjectBindingExact, evaluationInstant.Add(time.Minute)),
		},
		EvaluatedAt: evaluationInstant,
	}

	result, err := Evaluate(input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if result.Requirements[0].EvidenceState != domain.EvidenceStale {
		t.Fatalf("EvidenceState = %q, want STALE", result.Requirements[0].EvidenceState)
	}
	if result.Verdict.Decision != domain.DecisionBlocked {
		t.Fatalf("Decision = %q, want BLOCKED", result.Verdict.Decision)
	}
}

func TestSecurityConflictingCandidatesProduceConflictFinding(t *testing.T) {
	t.Parallel()

	input := Input{
		Contract: advisoryContract(referencedCriterion("req-a")),
		Policy:   advisoryPolicy(policyRequirement("req-a", "test-result")),
		Evidence: []evidence.Loaded{
			candidate("ev-pass"),
			conflictSibling("ev-fail", domain.OutcomeFailed),
		},
		EvaluatedAt: evaluationInstant,
	}

	result, err := Evaluate(input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	requirement := result.Requirements[0]
	if requirement.EvidenceState != domain.EvidenceInvalid {
		t.Fatalf("EvidenceState = %q, want INVALID", requirement.EvidenceState)
	}
	found := false
	for _, finding := range result.Findings {
		if finding.Code == "EVIDENCE_CONFLICT" {
			found = true
			if len(finding.EvidenceIDs) != 2 || finding.EvidenceIDs[0] != "ev-fail" || finding.EvidenceIDs[1] != "ev-pass" {
				t.Fatalf("EVIDENCE_CONFLICT EvidenceIDs = %#v, want sorted [ev-fail, ev-pass]", finding.EvidenceIDs)
			}
		}
	}
	if !found {
		t.Fatalf("EVIDENCE_CONFLICT finding missing: %#v", result.Findings)
	}
	if result.Verdict.Decision != domain.DecisionBlocked {
		t.Fatalf("Decision = %q, want BLOCKED", result.Verdict.Decision)
	}
}

func TestSecurityStaleSiblingDoesNotRescueOrVanish(t *testing.T) {
	t.Parallel()

	input := Input{
		Contract: advisoryContract(referencedCriterion("req-a")),
		Policy:   advisoryPolicy(policyRequirement("req-a", "test-result")),
		Evidence: []evidence.Loaded{
			candidate("ev-exact"),
			evidenceEntry("ev-stale", "test-result", "local-user", evidence.SubjectBindingOtherRevision, evaluationInstant.Add(-time.Minute)),
		},
		EvaluatedAt: evaluationInstant,
	}

	result, err := Evaluate(input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	// The exact candidate is used for the requirement, but the stale
	// sibling stays visible and blocks the evaluation.
	if result.Requirements[0].EvidenceState != domain.EvidenceValid {
		t.Fatalf("EvidenceState = %q, want VALID from the exact candidate", result.Requirements[0].EvidenceState)
	}
	staleVisible := false
	for _, finding := range result.Findings {
		if finding.Code == "EVIDENCE_STALE_REVISION" && finding.Blocking {
			staleVisible = true
		}
	}
	if !staleVisible {
		t.Fatalf("blocking EVIDENCE_STALE_REVISION finding missing: %#v", result.Findings)
	}
	if result.Verdict.Decision != domain.DecisionBlocked {
		t.Fatalf("Decision = %q, want BLOCKED", result.Verdict.Decision)
	}
}

func TestSecurityEmptyAllowlistBlocks(t *testing.T) {
	t.Parallel()

	requirement := policy.Requirement{
		ID:                   "req-a",
		EvidenceType:         "test-result",
		AllowedProducerTypes: nil,
	}
	input := Input{
		Contract:    advisoryContract(referencedCriterion("req-a")),
		Policy:      advisoryPolicy(requirement),
		Evidence:    []evidence.Loaded{candidate("ev-a")},
		EvaluatedAt: evaluationInstant,
	}

	result, err := Evaluate(input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if result.Requirements[0].EvidenceState != domain.EvidenceUntrusted {
		t.Fatalf("EvidenceState = %q, want UNTRUSTED", result.Requirements[0].EvidenceState)
	}
	if result.Verdict.Decision != domain.DecisionBlocked {
		t.Fatalf("Decision = %q, want BLOCKED", result.Verdict.Decision)
	}
}

func TestSecuritySelfAssertedTrustFieldsNeverConsulted(t *testing.T) {
	t.Parallel()

	base := Input{
		Contract:    advisoryContract(referencedCriterion("req-a")),
		Policy:      advisoryPolicy(policyRequirement("req-a", "test-result")),
		Evidence:    []evidence.Loaded{candidate("ev-a")},
		EvaluatedAt: evaluationInstant,
	}
	tampered := base
	tampered.Evidence = []evidence.Loaded{candidate("ev-a")}
	tampered.Evidence[0].TrustStatus = inputmeta.TrustStatus("AUTHORITATIVE")
	tampered.Evidence[0].AuthorityBasis = inputmeta.AuthorityBasis("TRUSTED_BASE")

	baseline, err := Evaluate(base)
	if err != nil {
		t.Fatalf("Evaluate(base) error = %v", err)
	}
	forged, err := Evaluate(tampered)
	if err != nil {
		t.Fatalf("Evaluate(tampered) error = %v", err)
	}
	if !reflect.DeepEqual(baseline, forged) {
		t.Fatalf("self-asserted trust fields changed the result:\nbaseline: %#v\nforged:   %#v", baseline, forged)
	}
}

func TestSecurityRepeatedEvaluationIsDeterministic(t *testing.T) {
	t.Parallel()

	build := func() Input {
		return Input{
			Contract: advisoryContract(referencedCriterion("req-a")),
			Policy: advisoryPolicy(
				policyRequirement("req-a", "test-result"),
				policyRequirement("req-b", "lint-result"),
			),
			Evidence: []evidence.Loaded{
				candidate("ev-a"),
				conflictSibling("ev-b", domain.OutcomeFailed),
				evidenceEntry("ev-stale", "lint-result", "local-user", evidence.SubjectBindingOtherRevision, evaluationInstant.Add(-time.Minute)),
			},
			EvaluatedAt: evaluationInstant,
		}
	}

	first, err := Evaluate(build())
	if err != nil {
		t.Fatalf("Evaluate(first) error = %v", err)
	}
	second, err := Evaluate(build())
	if err != nil {
		t.Fatalf("Evaluate(second) error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("repeated evaluation differs:\nfirst:  %#v\nsecond: %#v", first, second)
	}
}

func TestSecurityShuffledEvidencePreservesFindings(t *testing.T) {
	t.Parallel()

	requirements := advisoryPolicy(
		policyRequirement("req-a", "test-result"),
		policyRequirement("req-b", "lint-result"),
	)
	ordered := Input{
		Contract: advisoryContract(
			referencedCriterion("req-a"),
			referencedCriterion("req-b"),
		),
		Policy: requirements,
		Evidence: []evidence.Loaded{
			candidate("ev-a"),
			evidenceEntry("ev-stale", "lint-result", "local-user", evidence.SubjectBindingOtherRevision, evaluationInstant.Add(-time.Minute)),
			conflictSibling("ev-b", domain.OutcomeFailed),
		},
		EvaluatedAt: evaluationInstant,
	}
	shuffled := ordered
	shuffled.Evidence = []evidence.Loaded{
		ordered.Evidence[2],
		ordered.Evidence[0],
		ordered.Evidence[1],
	}

	forward, err := Evaluate(ordered)
	if err != nil {
		t.Fatalf("Evaluate(ordered) error = %v", err)
	}
	backward, err := Evaluate(shuffled)
	if err != nil {
		t.Fatalf("Evaluate(shuffled) error = %v", err)
	}
	if !reflect.DeepEqual(forward.Findings, backward.Findings) {
		t.Fatalf("findings order changed with input order:\nforward:  %#v\nbackward: %#v", forward.Findings, backward.Findings)
	}
	if !reflect.DeepEqual(forward.Verdict, backward.Verdict) {
		t.Fatalf("verdict changed with input order: %#v vs %#v", forward.Verdict, backward.Verdict)
	}
}
