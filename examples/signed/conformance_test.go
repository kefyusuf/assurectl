package signed_test

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/kefyusuf/assurectl/internal/attestation"
)

const conformanceReceipt = `{
  "schema_version": "assurectl/receipt/v0",
  "authority": "ADVISORY",
  "subject": {
    "repository_uri": "github.com/acme/checkout",
    "base_revision": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "head_revision": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
    "change_set_algorithm": "assurectl.git-change-set/v0",
    "change_set_digest": "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
  },
  "work_unit": {"id": "wu-001", "type": "commit", "objective": "conformance"},
  "requirement_results": [],
  "findings": [],
  "waivers": [],
  "verification_verdict": "PASSED",
  "completion_decision": "APPROVED",
  "verifier": {"name": "assurectl", "version": "dev", "protocol_version": "v0"},
  "evaluated_at": "2026-11-07T12:00:00Z"
}`

// conformanceSeed is fixed so the committed conformance fixtures are
// byte-for-byte reproducible; it protects nothing and signs nothing real.
var conformanceSeed = [32]byte{
	0x9d, 0x61, 0xb1, 0x9d, 0xef, 0xfd, 0x11, 0x0a, 0x5e, 0x71, 0xdb, 0xa1,
	0x30, 0xbd, 0x8e, 0x1c, 0x0c, 0x01, 0x1c, 0xa0, 0x93, 0x1e, 0x94, 0x65,
	0xfe, 0x66, 0x62, 0x8d, 0x4e, 0x0c, 0x6d, 0x65,
}

// TestConformanceRoundTrip pins the full protocol offline: the committed
// fixtures (receipt, attestation, trusted root) must verify and must match
// what the implementation produces from the fixed seed. Regenerate with
// FIXTURE_UPDATE=1.
func TestConformanceRoundTrip(t *testing.T) {
	t.Parallel()

	private := ed25519.NewKeyFromSeed(conformanceSeed[:])
	public := private.Public().(ed25519.PublicKey)

	statement, err := attestation.WrapReceipt([]byte(conformanceReceipt), "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")
	if err != nil {
		t.Fatalf("WrapReceipt() error = %v", err)
	}
	statementJSON, err := json.Marshal(statement)
	if err != nil {
		t.Fatal(err)
	}
	envelope := attestation.NewEnvelope(attestation.PayloadTypeReceiptV0, statementJSON)
	signed, err := attestation.SignEnvelope(envelope, attestation.KeyID(public), private)
	if err != nil {
		t.Fatalf("SignEnvelope() error = %v", err)
	}
	document, err := signed.MarshalDocument()
	if err != nil {
		t.Fatal(err)
	}

	root := `{"root_version":1,"issuers":[{"id":"conformance-issuer","keys":[{"keyid":"` + attestation.KeyID(public) + `","ed25519_public":"` + base64Std(public) + `"}],"valid":{"from":"2026-01-01T00:00:00Z","until":"2027-01-01T00:00:00Z"}}]}`

	if os.Getenv("FIXTURE_UPDATE") == "1" {
		if err := os.WriteFile("attestation.json", document, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile("root.json", []byte(root), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile("receipt.json", []byte(conformanceReceipt), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Log("fixtures regenerated")
		return
	}

	committed, err := os.ReadFile("attestation.json")
	if err != nil {
		t.Fatalf("read committed attestation: %v", err)
	}
	if string(committed) != string(document) {
		t.Fatalf("committed attestation does not match the implementation output")
	}
	committedRoot, err := os.ReadFile("root.json")
	if err != nil {
		t.Fatalf("read committed root: %v", err)
	}
	if string(committedRoot) != root {
		t.Fatalf("committed root does not match the implementation output")
	}

	parsed, err := attestation.ParseEnvelope(committed)
	if err != nil {
		t.Fatalf("ParseEnvelope() error = %v", err)
	}
	verifyAt := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	if err := attestation.VerifyEnvelope(parsed, mustRoot(t, committedRoot), verifyAt); err != nil {
		t.Fatalf("VerifyEnvelope() error = %v", err)
	}
}

func mustRoot(t *testing.T, document []byte) attestation.TrustedRoot {
	t.Helper()
	root, err := attestation.ParseTrustedRoot(document)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func base64Std(public ed25519.PublicKey) string {
	return base64.StdEncoding.EncodeToString(public)
}
