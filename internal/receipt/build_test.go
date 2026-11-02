package receipt

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/kefyusuf/assurectl/internal/contract"
	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/evaluation"
	"github.com/kefyusuf/assurectl/internal/evidence"
	"github.com/kefyusuf/assurectl/internal/inputmeta"
	"github.com/kefyusuf/assurectl/internal/policy"
)

var buildInstant = time.Date(2026, 11, 2, 10, 30, 0, 0, time.UTC)

func seededDigest(seed string) inputmeta.Digest {
	return inputmeta.SHA256([]byte(seed))
}

func buildSubject() domain.Subject {
	return domain.Subject{
		RepositoryURI: "github.com/acme/checkout",
		BaseRevision:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		HeadRevision:  "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ChangeSetAlgo: "assurectl.git-change-set/v0",
		ChangeSetHash: seededDigest("change-set").Value,
	}
}

func buildContract() contract.Loaded {
	return contract.Loaded{
		Contract: contract.Contract{
			SchemaVersion: contract.SchemaVersion,
			WorkUnit: domain.WorkUnit{
				ID:        "wu-001",
				Type:      domain.WorkUnitCommit,
				Objective: "fix the login timeout",
			},
		},
		Source:      "workspace:.assurectl/contract.json",
		Digest:      seededDigest("contract"),
		TrustStatus: inputmeta.TrustStatusUntrusted,
	}
}

func buildPolicy() policy.Loaded {
	return policy.Loaded{
		Policy: policy.Policy{
			SchemaVersion: policy.SchemaVersion,
		},
		Source:         "workspace:.assurectl/policy.json",
		Digest:         seededDigest("policy"),
		TrustStatus:    inputmeta.TrustStatusUntrusted,
		AuthorityBasis: inputmeta.AuthorityBasisAdvisoryWorkspace,
	}
}

func buildEvidence(id string) evidence.Loaded {
	return evidence.Loaded{
		Envelope: evidence.Envelope{ID: id, Type: "test-result"},
		Digest:   seededDigest("envelope-" + id),
	}
}

func buildEvaluation() evaluation.Result {
	return evaluation.Result{
		Requirements: []domain.RequirementResult{
			{
				RequirementID: "req-a",
				EvidenceState: domain.EvidenceValid,
				EvidenceIDs:   []string{"ev-b", "ev-a"},
				Outcome:       domain.OutcomePassed,
				Waivable:      false,
				WaiverStatus:  domain.WaiverNotApplicable,
			},
		},
		Findings: []domain.Finding{},
		Verdict: domain.EvaluationResult{
			Verdict:  domain.VerdictPassed,
			Decision: domain.DecisionApproved,
		},
	}
}

func buildInput() BuildInput {
	return BuildInput{
		Authority:   domain.AuthorityAdvisory,
		Subject:     buildSubject(),
		Contract:    buildContract(),
		Policy:      buildPolicy(),
		Evidence:    []evidence.Loaded{buildEvidence("ev-b"), buildEvidence("ev-a")},
		Evaluation:  buildEvaluation(),
		EvaluatedAt: buildInstant,
	}
}

func TestBuildMapsAdvisoryReceipt(t *testing.T) {
	t.Parallel()

	built, err := Build(buildInput())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if built.SchemaVersion != "assurectl/receipt/v0" {
		t.Fatalf("SchemaVersion = %q", built.SchemaVersion)
	}
	if built.Authority != domain.AuthorityAdvisory {
		t.Fatalf("Authority = %q", built.Authority)
	}
	if built.Subject != buildSubject() {
		t.Fatalf("Subject = %#v", built.Subject)
	}
	if built.WorkUnit.ID != "wu-001" || built.WorkUnit.Type != domain.WorkUnitCommit {
		t.Fatalf("WorkUnit = %#v", built.WorkUnit)
	}
	if built.Contract.Source != "workspace:.assurectl/contract.json" || built.Contract.Digest != seededDigest("contract") {
		t.Fatalf("Contract = %#v", built.Contract)
	}
	if built.Contract.TrustStatus != inputmeta.TrustStatusUntrusted {
		t.Fatalf("Contract.TrustStatus = %q", built.Contract.TrustStatus)
	}
	if built.Policy.EffectiveDigest != seededDigest("policy") {
		t.Fatalf("Policy.EffectiveDigest = %#v", built.Policy.EffectiveDigest)
	}
	if len(built.Policy.Sources) != 2 {
		t.Fatalf("Policy.Sources = %d entries, want the workspace layer and the builtin protocol baseline", len(built.Policy.Sources))
	}

	if len(built.Evidence) != 2 {
		t.Fatalf("Evidence = %d entries, want 2", len(built.Evidence))
	}
	if built.Evidence[0].ID != "ev-a" || built.Evidence[1].ID != "ev-b" {
		t.Fatalf("Evidence order = [%q, %q], want sorted by id", built.Evidence[0].ID, built.Evidence[1].ID)
	}
	if built.Evidence[0].Digest != seededDigest("envelope-ev-a") {
		t.Fatalf("Evidence[0].Digest = %#v", built.Evidence[0].Digest)
	}

	if len(built.RequirementResults) != 1 || built.RequirementResults[0].RequirementID != "req-a" {
		t.Fatalf("RequirementResults = %#v", built.RequirementResults)
	}
	if len(built.Findings) != 0 {
		t.Fatalf("Findings = %#v, want empty", built.Findings)
	}
	if built.Waivers == nil || len(built.Waivers) != 0 {
		t.Fatalf("Waivers = %#v, want an empty non-nil array", built.Waivers)
	}
	if built.VerificationVerdict != domain.VerdictPassed || built.CompletionDecision != domain.DecisionApproved {
		t.Fatalf("verdict/decision = (%q, %q)", built.VerificationVerdict, built.CompletionDecision)
	}
	if built.Verifier.Name != "assurectl" || built.Verifier.ProtocolVersion != "v0" || built.Verifier.Version == "" {
		t.Fatalf("Verifier = %#v", built.Verifier)
	}
	if built.EvaluatedAt != "2026-11-02T10:30:00Z" {
		t.Fatalf("EvaluatedAt = %q", built.EvaluatedAt)
	}
}

func TestBuildEmitsSchemaFieldNames(t *testing.T) {
	t.Parallel()

	built, err := Build(buildInput())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	data, err := json.Marshal(built)
	if err != nil {
		t.Fatal(err)
	}
	document := string(data)

	for _, name := range []string{
		`"schema_version"`, `"authority"`, `"subject"`, `"work_unit"`,
		`"contract"`, `"policy"`, `"evidence"`, `"requirement_results"`,
		`"findings"`, `"waivers"`, `"verification_verdict"`,
		`"completion_decision"`, `"verifier"`, `"evaluated_at"`,
	} {
		if !strings.Contains(document, name) {
			t.Fatalf("marshaled receipt misses %s: %s", name, document)
		}
	}
	if strings.Contains(document, "null") {
		t.Fatalf("marshaled receipt contains a null array: %s", document)
	}
}
