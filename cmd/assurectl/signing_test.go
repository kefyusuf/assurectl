package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// keyIDOf mirrors the attestation package's deterministic key id.
func keyIDOf(t *testing.T, public ed25519.PublicKey) string {
	t.Helper()
	sum := sha256.Sum256(public)
	return hex.EncodeToString(sum[:])
}

// base64Std encodes the raw public key for the trusted-root document.
func base64Std(t *testing.T, public ed25519.PublicKey) string {
	t.Helper()
	return base64.StdEncoding.EncodeToString(public)
}

const fixtureReceiptDoc = `{
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
  "waivers": [],
  "verification_verdict": "PASSED",
  "completion_decision": "APPROVED",
  "verifier": {"name": "assurectl", "version": "dev", "protocol_version": "v0"},
  "evaluated_at": "2026-11-07T12:00:00Z"
}`

func fixtureKeyPair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey, string) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	seedPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: der,
	})
	return public, private, string(seedPEM)
}

func tempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSignReceiptProducesVerifiableAttestation(t *testing.T) {
	t.Parallel()

	public, _, seedPEM := fixtureKeyPair(t)
	keyPath := tempFile(t, "key.pem", seedPEM)
	receiptPath := tempFile(t, "receipt.json", fixtureReceiptDoc)
	attestationPath := filepath.Join(t.TempDir(), "attestation.json")

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	if code := run([]string{
		"sign-receipt",
		"--key", keyPath,
		"--key-id", keyIDOf(t, public),
		"--in", receiptPath,
		"--out", attestationPath,
	}, stdout, stderr); code != 0 {
		t.Fatalf("sign-receipt exit = %d, stdout: %s, stderr: %s", code, stdout.String(), stderr.String())
	}

	attestation, err := os.ReadFile(attestationPath)
	if err != nil {
		t.Fatalf("read attestation: %v", err)
	}
	// The statement travels base64-encoded inside the DSSE payload; the
	// verify-receipt step below is the structural check.
	if !strings.Contains(string(attestation), `"payloadType":"https://assurectl.dev/attestations/receipt/v0"`) {
		t.Fatalf("attestation = %q, want a receipt attestation", attestation)
	}

	// The attestation must verify against the matching trusted root.
	rootJSON := `{"root_version":1,"issuers":[{"id":"test-issuer","keys":[{"keyid":"` + keyIDOf(t, public) + `","ed25519_public":"` + base64Std(t, public) + `"}],"valid":{"from":"2026-01-01T00:00:00Z","until":"2027-01-01T00:00:00Z"}}]}`
	rootPath := tempFile(t, "root.json", rootJSON)
	stdout = &strings.Builder{}
	stderr = &strings.Builder{}
	if code := run([]string{
		"verify-receipt",
		"--trusted-root", rootPath,
		"--in", attestationPath,
	}, stdout, stderr); code != 0 {
		t.Fatalf("verify-receipt exit = %d, stdout: %s, stderr: %s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), `"completion_decision":"APPROVED"`) {
		t.Fatalf("stdout = %q, want the embedded receipt", stdout.String())
	}
}

func TestVerifyReceiptRejectsTamperedAttestation(t *testing.T) {
	t.Parallel()

	public, _, seedPEM := fixtureKeyPair(t)
	keyPath := tempFile(t, "key.pem", seedPEM)
	receiptPath := tempFile(t, "receipt.json", fixtureReceiptDoc)
	attestationPath := filepath.Join(t.TempDir(), "attestation.json")
	if code := run([]string{
		"sign-receipt", "--key", keyPath, "--key-id", keyIDOf(t, public), "--in", receiptPath, "--out", attestationPath,
	}, &strings.Builder{}, &strings.Builder{}); code != 0 {
		t.Fatalf("sign-receipt exit = %d", code)
	}

	attestation, err := os.ReadFile(attestationPath)
	if err != nil {
		t.Fatal(err)
	}
	// Flip the first character of the base64 payload: the signature covers
	// the payload bytes, so this must fail verification.
	var envelope map[string]any
	if err := json.Unmarshal(attestation, &envelope); err != nil {
		t.Fatal(err)
	}
	payload, _ := envelope["payload"].(string)
	if len(payload) == 0 {
		t.Fatal("envelope carries no payload")
	}
	envelope["payload"] = "A" + payload[1:]
	tampered, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	tamperedPath := tempFile(t, "tampered.json", string(tampered))

	rootJSON := `{"root_version":1,"issuers":[{"id":"test-issuer","keys":[{"keyid":"` + keyIDOf(t, public) + `","ed25519_public":"` + base64Std(t, public) + `"}],"valid":{"from":"2026-01-01T00:00:00Z","until":"2027-01-01T00:00:00Z"}}]}`
	rootPath := tempFile(t, "root.json", rootJSON)
	stdout := &strings.Builder{}
	if code := run([]string{
		"verify-receipt", "--trusted-root", rootPath, "--in", tamperedPath,
	}, stdout, &strings.Builder{}); code != 2 {
		t.Fatalf("verify-receipt exit = %d, want 2 for tampering", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on failure", stdout.String())
	}
}
