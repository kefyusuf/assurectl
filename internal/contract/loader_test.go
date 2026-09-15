package contract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadLocalReturnsTypedAdvisoryContractAndCanonicalDigest(t *testing.T) {
	root := t.TempDir()
	writeContractFile(t, root, `{
  "acceptance_criteria": [
    {
      "requirements": ["unit-tests"],
      "statement": "Tests pass.",
      "id": "AC-1"
    }
  ],
  "work_unit": {
    "objective": "Verify change.",
    "type": "commit",
    "id": "wu-1"
  },
  "schema_version": "assurectl/verification-contract/v0"
}`)

	loaded, err := LoadLocal(root)
	if err != nil {
		t.Fatalf("LoadLocal() error = %v", err)
	}

	if loaded.Contract.SchemaVersion != "assurectl/verification-contract/v0" {
		t.Fatalf("SchemaVersion = %q", loaded.Contract.SchemaVersion)
	}
	if loaded.Contract.WorkUnit.ID != "wu-1" || string(loaded.Contract.WorkUnit.Type) != "commit" {
		t.Fatalf("WorkUnit = %#v", loaded.Contract.WorkUnit)
	}
	if got := loaded.Contract.AcceptanceCriteria[0].Requirements[0]; got != "unit-tests" {
		t.Fatalf("requirement = %q", got)
	}
	if loaded.Source != "workspace:.assurectl/contract.json" {
		t.Fatalf("Source = %q", loaded.Source)
	}
	if string(loaded.TrustStatus) != "UNTRUSTED" {
		t.Fatalf("TrustStatus = %q", loaded.TrustStatus)
	}
	if loaded.Digest.Algorithm != "sha256" {
		t.Fatalf("Digest.Algorithm = %q", loaded.Digest.Algorithm)
	}
	if loaded.Digest.Value != "426a2076022a763bf5e3b1c9bf7daef3473e1f691604c9f01274cbcd5753dc7e" {
		t.Fatalf("Digest.Value = %q", loaded.Digest.Value)
	}
}

func TestLoadLocalDigestIgnoresJSONFormattingAndObjectKeyOrder(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()

	writeContractFile(t, rootA, `{"schema_version":"assurectl/verification-contract/v0","work_unit":{"id":"wu-1","type":"commit","objective":"Verify change."},"acceptance_criteria":[{"id":"AC-1","statement":"Tests pass.","requirements":["unit-tests"]}]}`)
	writeContractFile(t, rootB, `{
  "acceptance_criteria": [{"requirements":["unit-tests"],"statement":"Tests pass.","id":"AC-1"}],
  "schema_version": "assurectl/verification-contract/v0",
  "work_unit": {"objective":"Verify change.","id":"wu-1","type":"commit"}
}`)

	a, err := LoadLocal(rootA)
	if err != nil {
		t.Fatalf("LoadLocal(rootA) error = %v", err)
	}
	b, err := LoadLocal(rootB)
	if err != nil {
		t.Fatalf("LoadLocal(rootB) error = %v", err)
	}
	if a.Digest != b.Digest {
		t.Fatalf("digests differ: %#v != %#v", a.Digest, b.Digest)
	}
}

func TestLoadLocalRejectsMalformedOrAmbiguousContracts(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name: "self asserted trust",
			content: `{
  "schema_version":"assurectl/verification-contract/v0",
  "trust_status":"TRUSTED",
  "work_unit":{"id":"wu-1","type":"commit","objective":"Verify change."},
  "acceptance_criteria":[{"id":"AC-1","statement":"Tests pass.","requirements":["unit-tests"]}]
}`,
			wantErr: "unknown field",
		},
		{
			name: "unsupported schema",
			content: `{
  "schema_version":"assurectl/verification-contract/v9",
  "work_unit":{"id":"wu-1","type":"commit","objective":"Verify change."},
  "acceptance_criteria":[{"id":"AC-1","statement":"Tests pass.","requirements":["unit-tests"]}]
}`,
			wantErr: "schema_version",
		},
		{
			name: "duplicate json key",
			content: `{
  "schema_version":"assurectl/verification-contract/v0",
  "schema_version":"assurectl/verification-contract/v0",
  "work_unit":{"id":"wu-1","type":"commit","objective":"Verify change."},
  "acceptance_criteria":[{"id":"AC-1","statement":"Tests pass.","requirements":["unit-tests"]}]
}`,
			wantErr: "duplicate object key",
		},
		{
			name: "duplicate criterion id",
			content: `{
  "schema_version":"assurectl/verification-contract/v0",
  "work_unit":{"id":"wu-1","type":"commit","objective":"Verify change."},
  "acceptance_criteria":[
    {"id":"AC-1","statement":"Tests pass.","requirements":["unit-tests"]},
    {"id":"AC-1","statement":"Integration passes.","requirements":["integration-tests"]}
  ]
}`,
			wantErr: "duplicate acceptance criterion id",
		},
		{
			name: "duplicate requirement reference",
			content: `{
  "schema_version":"assurectl/verification-contract/v0",
  "work_unit":{"id":"wu-1","type":"commit","objective":"Verify change."},
  "acceptance_criteria":[{"id":"AC-1","statement":"Tests pass.","requirements":["unit-tests","unit-tests"]}]
}`,
			wantErr: "duplicate requirement id",
		},
		{
			name: "unknown work unit type",
			content: `{
  "schema_version":"assurectl/verification-contract/v0",
  "work_unit":{"id":"wu-1","type":"arbitrary","objective":"Verify change."},
  "acceptance_criteria":[{"id":"AC-1","statement":"Tests pass.","requirements":["unit-tests"]}]
}`,
			wantErr: "work_unit.type",
		},
		{
			name: "nested unknown field",
			content: `{
  "schema_version":"assurectl/verification-contract/v0",
  "work_unit":{"id":"wu-1","type":"commit","objective":"Verify change.","trusted":true},
  "acceptance_criteria":[{"id":"AC-1","statement":"Tests pass.","requirements":["unit-tests"]}]
}`,
			wantErr: "unknown field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeContractFile(t, root, tt.content)

			_, err := LoadLocal(root)
			if err == nil {
				t.Fatal("LoadLocal() error = nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("LoadLocal() error = %q, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadLocalRejectsMissingContract(t *testing.T) {
	_, err := LoadLocal(t.TempDir())
	if err == nil {
		t.Fatal("LoadLocal() error = nil")
	}
}

func writeContractFile(t *testing.T, root, content string) {
	t.Helper()

	dir := filepath.Join(root, ".assurectl")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "contract.json"), []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
