// Package conformance provides a language-agnostic conformance test runner
// for the AssureCTL protocol. It reads JSON vector files from the
// conformance/ directory and validates each document's structure against
// the expected schema. Vectors are language-agnostic: any implementation can
// run the same checks against the same files.
package conformance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Vector is the wrapper format for one conformance test case.
type Vector struct {
	Description string          `json:"description"`
	Expect      string          `json:"expect"`
	Document    json.RawMessage `json:"document"`
}

// LoadVectors reads all JSON vector files from the given directory and
// returns them sorted by filename.
func LoadVectors(t *testing.T, dir string) []NamedVector {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read conformance directory %s: %v", dir, err)
	}
	var vectors []NamedVector
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatalf("read vector %s: %v", entry.Name(), err)
		}
		var vector Vector
		if err := json.Unmarshal(data, &vector); err != nil {
			t.Fatalf("unmarshal vector %s: %v", entry.Name(), err)
		}
		vectors = append(vectors, NamedVector{
			Name:   strings.TrimSuffix(entry.Name(), ".json"),
			Vector: vector,
		})
	}
	return vectors
}

// NamedVector pairs a vector with its file name for reporting.
type NamedVector struct {
	Name   string
	Vector Vector
}

// ValidateEvidenceEnvelope performs structural validation of an evidence
// envelope document: required fields, enum values, and format patterns.
func ValidateEvidenceEnvelope(document json.RawMessage) error {
	var envelope struct {
		SchemaVersion string `json:"schema_version"`
		ID            string `json:"id"`
		Type          string `json:"type"`
		Subject       struct {
			RepositoryURI string `json:"repository_uri"`
			Revision      string `json:"revision"`
		} `json:"subject"`
		Producer struct {
			Type     string `json:"type"`
			Identity string `json:"identity"`
		} `json:"producer"`
		Invocation struct {
			CommandID  string `json:"command_id"`
			StartedAt  string `json:"started_at"`
			FinishedAt string `json:"finished_at"`
		} `json:"invocation"`
		Outcome struct {
			Status   string `json:"status"`
			ExitCode *int   `json:"exit_code"`
		} `json:"outcome"`
		Artifact struct {
			URI    string `json:"uri"`
			Digest struct {
				Algorithm string `json:"algorithm"`
				Value     string `json:"value"`
			} `json:"digest"`
			MediaType string `json:"media_type"`
		} `json:"artifact"`
	}
	if err := json.Unmarshal(document, &envelope); err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	if envelope.SchemaVersion != "assurectl/evidence-envelope/v0" {
		return fmt.Errorf("schema_version %q is unsupported", envelope.SchemaVersion)
	}
	if envelope.ID == "" {
		return fmt.Errorf("id must not be empty")
	}
	if envelope.Type == "" {
		return fmt.Errorf("type must not be empty")
	}
	if envelope.Subject.RepositoryURI == "" || envelope.Subject.Revision == "" {
		return fmt.Errorf("subject is incomplete")
	}
	if !isHexRevision(envelope.Subject.Revision) {
		return fmt.Errorf("subject.revision %q is not a valid object id", envelope.Subject.Revision)
	}
	if envelope.Producer.Type == "" || envelope.Producer.Identity == "" {
		return fmt.Errorf("producer is incomplete")
	}
	if envelope.Invocation.CommandID == "" || envelope.Invocation.StartedAt == "" || envelope.Invocation.FinishedAt == "" {
		return fmt.Errorf("invocation is incomplete")
	}
	switch envelope.Outcome.Status {
	case "PASSED", "FAILED", "ERROR":
	default:
		return fmt.Errorf("outcome.status %q is unsupported", envelope.Outcome.Status)
	}
	if envelope.Artifact.URI == "" || envelope.Artifact.Digest.Value == "" {
		return fmt.Errorf("artifact is incomplete")
	}
	return nil
}

// ValidateReceipt performs structural validation of a receipt document.
func ValidateReceipt(document json.RawMessage) error {
	var receipt struct {
		SchemaVersion string `json:"schema_version"`
		Authority     string `json:"authority"`
		Subject       struct {
			RepositoryURI string `json:"repository_uri"`
		} `json:"subject"`
		VerificationVerdict string `json:"verification_verdict"`
		CompletionDecision  string `json:"completion_decision"`
		EvaluatedAt         string `json:"evaluated_at"`
		Verifier            struct {
			Name string `json:"name"`
		} `json:"verifier"`
	}
	if err := json.Unmarshal(document, &receipt); err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	if receipt.SchemaVersion != "assurectl/receipt/v0" {
		return fmt.Errorf("schema_version %q is unsupported", receipt.SchemaVersion)
	}
	if receipt.Authority == "" || receipt.Subject.RepositoryURI == "" {
		return fmt.Errorf("authority or subject is missing")
	}
	if receipt.VerificationVerdict == "" || receipt.CompletionDecision == "" {
		return fmt.Errorf("verdict or decision is missing")
	}
	if receipt.Verifier.Name != "assurectl" {
		return fmt.Errorf("verifier.name %q is unsupported", receipt.Verifier.Name)
	}
	if receipt.EvaluatedAt == "" {
		return fmt.Errorf("evaluated_at must not be empty")
	}
	return nil
}

// ValidateCheckPayload performs structural validation of a check-payload
// document.
func ValidateCheckPayload(document json.RawMessage) error {
	var payload struct {
		Name             string `json:"name"`
		Conclusion       string `json:"conclusion"`
		Title            string `json:"title"`
		Summary          string `json:"summary"`
		ReceiptReference struct {
			ChangeSetDigest string `json:"change_set_digest"`
			ReceiptSHA256   string `json:"receipt_sha256"`
			Authority       string `json:"authority"`
		} `json:"receipt_reference"`
	}
	if err := json.Unmarshal(document, &payload); err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	if payload.Name != "assurectl" {
		return fmt.Errorf("name %q is unsupported", payload.Name)
	}
	switch payload.Conclusion {
	case "success", "failure", "neutral":
	default:
		return fmt.Errorf("conclusion %q is unsupported", payload.Conclusion)
	}
	if payload.ReceiptReference.ChangeSetDigest == "" || payload.ReceiptReference.ReceiptSHA256 == "" {
		return fmt.Errorf("receipt_reference is incomplete")
	}
	return nil
}

// ValidateTrustedRoot performs structural validation of a trusted-root
// payload document.
func ValidateTrustedRoot(document json.RawMessage) error {
	var root struct {
		RootVersion int `json:"root_version"`
		Issuers     []struct {
			ID   string `json:"id"`
			Keys []struct {
				KeyID         string `json:"keyid"`
				Ed25519Public string `json:"ed25519_public"`
			} `json:"keys"`
			Valid struct {
				From  string `json:"from"`
				Until string `json:"until"`
			} `json:"valid"`
		} `json:"issuers"`
	}
	if err := json.Unmarshal(document, &root); err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	if root.RootVersion < 1 {
		return fmt.Errorf("root_version %d is unsupported", root.RootVersion)
	}
	for _, issuer := range root.Issuers {
		if issuer.ID == "" {
			return fmt.Errorf("issuer id must not be empty")
		}
		if issuer.Valid.From == "" || issuer.Valid.Until == "" {
			return fmt.Errorf("issuer %q validity window is incomplete", issuer.ID)
		}
	}
	return nil
}

// isHexRevision checks for a 40 or 64 character lowercase hex string.
func isHexRevision(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
