package verify

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/waiver"
)

const waivablePolicy = `{
  "schema_version": "assurectl/policy/v0",
  "requirements": [{"id": "req-a", "evidence_type": "test-result", "waivable": true, "allowed_producer_types": ["local-user"]}]
}`

func waiverFileFixture(t *testing.T, root string) string {
	t.Helper()
	assurectl := filepath.Join(root, ".assurectl")
	if err := os.MkdirAll(assurectl, 0o755); err != nil {
		t.Fatal(err)
	}
	record := waiver.Record{
		ID:           "wv-001",
		Target:       waiver.Target{Kind: "REQUIREMENT", ID: "req-a"},
		Scope:        waiver.Scope{RepositoryURI: "github.com/acme/checkout", WorkUnitID: "wu-001"},
		Reason:       "known flaky test",
		AcceptedRisk: "retry before merge",
		Approver:     waiver.Approver{Identity: "release-engineer", Authority: "RELEASE_MANAGER"},
		IssuedAt:     "2026-11-01T00:00:00Z",
		ExpiresAt:    "2026-12-01T00:00:00Z",
		Status:       domain.WaiverValid,
	}
	record.Digest = waiver.CanonicalDigest(record)
	document, err := jsonMarshalWaivers(record)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(assurectl, "waivers.json")
	if err := os.WriteFile(path, document, 0o644); err != nil {
		t.Fatal(err)
	}
	return ".assurectl/waivers.json"
}

func jsonMarshalWaivers(record waiver.Record) ([]byte, error) {
	document := map[string]any{
		"schema_version": "assurectl/waivers/v0",
		"waivers":        []waiver.Record{record},
	}
	return json.Marshal(document)
}

func TestRunAppliesWaiverAndAcceptsWithRisk(t *testing.T) {
	t.Parallel()

	root, head := fixtureWorkspace(t, func(string, string) string { return "github.com/acme/checkout" })
	if err := os.WriteFile(filepath.Join(root, ".assurectl", "policy.json"), []byte(waivablePolicy), 0o644); err != nil {
		t.Fatal(err)
	}
	// Flip the single evidence outcome to FAILED so the requirement is an
	// established, waivable failure.
	envelopePath := filepath.Join(root, ".assurectl", "evidence", "unit-tests.json")
	envelope, err := os.ReadFile(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	failed := bytes.Replace(envelope, []byte(`"status": "PASSED"`), []byte(`"status": "FAILED"`), 1)
	if err := os.WriteFile(envelopePath, failed, 0o644); err != nil {
		t.Fatal(err)
	}
	relative := waiverFileFixture(t, root)

	result, err := Run(context.Background(), root, Options{
		RepositoryURI: "https://github.com/acme/checkout.git",
		BaseRef:       head,
		HeadRef:       head,
		EvaluatedAt:   buildInstantHelper(),
		WaiversPath:   relative,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Receipt.CompletionDecision != domain.DecisionAcceptedWithRisk {
		t.Fatalf("Decision = %q, want ACCEPTED_WITH_RISK", result.Receipt.CompletionDecision)
	}
	if len(result.Receipt.Waivers) != 1 || result.Receipt.Waivers[0].Status != domain.WaiverValid {
		t.Fatalf("receipt waivers = %#v, want the applied record", result.Receipt.Waivers)
	}
}

func TestRunFailsOnMalformedWaiverFile(t *testing.T) {
	t.Parallel()

	root, head := fixtureWorkspace(t, func(string, string) string { return "github.com/acme/checkout" })
	if err := os.WriteFile(filepath.Join(root, ".assurectl", "waivers.json"),
		[]byte(`{"schema_version": "assurectl/waivers/v0", "waivers": [{"id": "broken"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	waiversPath := ".assurectl/waivers.json"
	_ = waiversPath

	if _, err := Run(context.Background(), root, Options{
		RepositoryURI: "https://github.com/acme/checkout.git",
		BaseRef:       head,
		HeadRef:       head,
		EvaluatedAt:   buildInstantHelper(),
		WaiversPath:   waiversPath,
	}); err == nil {
		t.Fatalf("Run() accepted a malformed waiver file")
	}
}
