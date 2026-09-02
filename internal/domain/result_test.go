package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestVerificationVerdictValid(t *testing.T) {
	t.Parallel()

	for _, value := range []VerificationVerdict{VerdictPassed, VerdictFailed, VerdictIndeterminate} {
		if !value.Valid() {
			t.Fatalf("VerificationVerdict(%q).Valid() = false, want true", value)
		}
	}
	for _, value := range []VerificationVerdict{"", "PASS", "UNKNOWN"} {
		if value.Valid() {
			t.Fatalf("VerificationVerdict(%q).Valid() = true, want false", value)
		}
	}
}

func TestCompletionDecisionValid(t *testing.T) {
	t.Parallel()

	for _, value := range []CompletionDecision{
		DecisionApproved,
		DecisionRejected,
		DecisionBlocked,
		DecisionAcceptedWithRisk,
	} {
		if !value.Valid() {
			t.Fatalf("CompletionDecision(%q).Valid() = false, want true", value)
		}
	}
	for _, value := range []CompletionDecision{"", "PASS", "UNKNOWN"} {
		if value.Valid() {
			t.Fatalf("CompletionDecision(%q).Valid() = true, want false", value)
		}
	}
}

func TestFindingSeverityValid(t *testing.T) {
	t.Parallel()

	for _, value := range []FindingSeverity{SeverityInfo, SeverityWarning, SeverityError} {
		if !value.Valid() {
			t.Fatalf("FindingSeverity(%q).Valid() = false, want true", value)
		}
	}
	for _, value := range []FindingSeverity{"", "WARN", "UNKNOWN"} {
		if value.Valid() {
			t.Fatalf("FindingSeverity(%q).Valid() = true, want false", value)
		}
	}
}

func TestFindingCategoryValid(t *testing.T) {
	t.Parallel()

	for _, value := range []FindingCategory{
		FindingCategorySubject,
		FindingCategoryContract,
		FindingCategoryPolicy,
		FindingCategoryEvidence,
		FindingCategoryVerification,
		FindingCategoryWaiver,
		FindingCategoryAuthority,
		FindingCategoryProtocol,
		FindingCategoryInternal,
	} {
		if !value.Valid() {
			t.Fatalf("FindingCategory(%q).Valid() = false, want true", value)
		}
	}
	for _, value := range []FindingCategory{"", "INPUT", "UNKNOWN"} {
		if value.Valid() {
			t.Fatalf("FindingCategory(%q).Valid() = true, want false", value)
		}
	}
}

func TestFindingJSONIncludesCategory(t *testing.T) {
	t.Parallel()

	finding := Finding{
		Code:     "ASR-EV-REQUIRED-MISSING-001",
		Category: FindingCategoryEvidence,
		Severity: SeverityError,
		Message:  "required evidence is missing",
		Blocking: true,
	}

	encoded, err := json.Marshal(finding)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if !strings.Contains(string(encoded), `"category":"EVIDENCE"`) {
		t.Fatalf("JSON = %s, missing typed finding category", encoded)
	}
}

func TestWaiverStatusValid(t *testing.T) {
	t.Parallel()

	for _, value := range []WaiverStatus{WaiverNotApplicable, WaiverValid, WaiverInvalid} {
		if !value.Valid() {
			t.Fatalf("WaiverStatus(%q).Valid() = false, want true", value)
		}
	}
	for _, value := range []WaiverStatus{"", "APPROVED", "UNKNOWN"} {
		if value.Valid() {
			t.Fatalf("WaiverStatus(%q).Valid() = true, want false", value)
		}
	}
}

func TestRequirementResultJSONRoundTrip(t *testing.T) {
	t.Parallel()

	want := RequirementResult{
		RequirementID: "unit-tests",
		EvidenceState: EvidenceValid,
		Outcome:       OutcomeFailed,
		Waivable:      true,
		WaiverStatus:  WaiverValid,
	}

	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var got RequirementResult
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got != want {
		t.Fatalf("round trip = %#v, want %#v", got, want)
	}
}

func TestSubjectSerializesAlgorithmSeparatelyFromDigest(t *testing.T) {
	t.Parallel()

	subject := Subject{
		RepositoryURI: "github.com/acme/checkout",
		BaseRevision:  strings.Repeat("a", 40),
		HeadRevision:  strings.Repeat("b", 40),
		ChangeSetAlgo: "assurectl.git-change-set/v0",
		ChangeSetHash: strings.Repeat("c", 64),
	}

	encoded, err := json.Marshal(subject)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	text := string(encoded)
	if !strings.Contains(text, `"change_set_algorithm":"assurectl.git-change-set/v0"`) {
		t.Fatalf("JSON = %s, missing change_set_algorithm", text)
	}
	if !strings.Contains(text, `"change_set_digest":"`+strings.Repeat("c", 64)+`"`) {
		t.Fatalf("JSON = %s, missing change_set_digest", text)
	}
}

func TestUnknownEnumJSONIsNotCoerced(t *testing.T) {
	t.Parallel()

	var got RequirementResult
	if err := json.Unmarshal([]byte(`{"requirement_id":"unit-tests","evidence_state":"made-up","waiver_status":"NOT_APPLICABLE"}`), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got.EvidenceState.Valid() {
		t.Fatalf("unknown evidence state %q was treated as valid", got.EvidenceState)
	}
}
