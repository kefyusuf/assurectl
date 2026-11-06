package attestation

import (
	"encoding/json"
	"strings"
	"testing"
)

const fixtureReceiptDocument = `{
  "schema_version": "assurectl/receipt/v0",
  "authority": "ADVISORY",
  "subject": {
    "repository_uri": "github.com/acme/checkout",
    "base_revision": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "head_revision": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
    "change_set_algorithm": "assurectl.git-change-set/v0",
    "change_set_digest": "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
  },
  "work_unit": {"id": "wu-001", "type": "commit", "objective": "fixture"},
  "requirement_results": [],
  "findings": [],
  "waivers": []
}`

const fixtureChangeSetDigest = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"

func TestWrapReceiptBuildsInTotoStatement(t *testing.T) {
	t.Parallel()

	statement, err := WrapReceipt([]byte(fixtureReceiptDocument), fixtureChangeSetDigest)
	if err != nil {
		t.Fatalf("WrapReceipt() error = %v", err)
	}
	if statement.Type != InTotoStatementType {
		t.Fatalf("_type = %q", statement.Type)
	}
	if statement.PredicateType != PayloadTypeReceiptV0 {
		t.Fatalf("predicateType = %q", statement.PredicateType)
	}
	if len(statement.Subject) != 1 {
		t.Fatalf("subject = %d entries, want 1", len(statement.Subject))
	}
	subject := statement.Subject[0]
	if subject.Name != "github.com/acme/checkout" {
		t.Fatalf("subject name = %q", subject.Name)
	}
	if subject.Digest["gitChangeSet"] != fixtureChangeSetDigest {
		t.Fatalf("subject digest = %#v", subject.Digest)
	}
	var predicate map[string]any
	if err := json.Unmarshal(statement.Predicate, &predicate); err != nil {
		t.Fatalf("predicate is not JSON: %v", err)
	}
	if predicate["schema_version"] != "assurectl/receipt/v0" {
		t.Fatalf("predicate = %#v", predicate)
	}
}

func TestWrapReceiptRejectsReceiptWithoutSubject(t *testing.T) {
	t.Parallel()

	if _, err := WrapReceipt([]byte(`{"schema_version":"assurectl/receipt/v0"}`), fixtureChangeSetDigest); err == nil {
		t.Fatalf("WrapReceipt() accepted a receipt without a subject")
	}
}

func TestStatementRoundTrips(t *testing.T) {
	t.Parallel()

	statement, err := WrapReceipt([]byte(fixtureReceiptDocument), fixtureChangeSetDigest)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(statement)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Statement
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.PredicateType != statement.PredicateType || !strings.Contains(string(decoded.Predicate), "assurectl/receipt/v0") {
		t.Fatalf("round-trip changed the statement: %#v", decoded)
	}
}
