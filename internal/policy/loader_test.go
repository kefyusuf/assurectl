package policy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalPolicyPathIsFixed(t *testing.T) {
	if localPolicyPath != ".assurectl/policy.json" {
		t.Fatalf("localPolicyPath = %q", localPolicyPath)
	}
}

func TestLoadLocalReturnsTypedAdvisoryPolicyAndCanonicalDigest(t *testing.T) {
	root := t.TempDir()
	writePolicyFile(t, root, `{
  "requirements": [
    {
      "max_age_seconds": 3600,
      "allowed_producer_types": ["local-user"],
      "waivable": false,
      "evidence_type": "test-result",
      "id": "unit-tests"
    }
  ],
  "schema_version": "assurectl/policy/v0"
}`)

	loaded, err := LoadLocal(root)
	if err != nil {
		t.Fatalf("LoadLocal() error = %v", err)
	}

	if loaded.Policy.SchemaVersion != "assurectl/policy/v0" {
		t.Fatalf("SchemaVersion = %q", loaded.Policy.SchemaVersion)
	}
	if len(loaded.Policy.Requirements) != 1 {
		t.Fatalf("Requirements length = %d", len(loaded.Policy.Requirements))
	}
	requirement := loaded.Policy.Requirements[0]
	if requirement.ID != "unit-tests" || requirement.EvidenceType != "test-result" {
		t.Fatalf("Requirement = %#v", requirement)
	}
	if requirement.Waivable {
		t.Fatal("Waivable = true")
	}
	if len(requirement.AllowedProducerTypes) != 1 || requirement.AllowedProducerTypes[0] != "local-user" {
		t.Fatalf("AllowedProducerTypes = %#v", requirement.AllowedProducerTypes)
	}
	if requirement.MaxAgeSeconds == nil || *requirement.MaxAgeSeconds != 3600 {
		t.Fatalf("MaxAgeSeconds = %#v", requirement.MaxAgeSeconds)
	}
	if loaded.Source != "workspace:.assurectl/policy.json" {
		t.Fatalf("Source = %q", loaded.Source)
	}
	if string(loaded.TrustStatus) != "UNTRUSTED" {
		t.Fatalf("TrustStatus = %q", loaded.TrustStatus)
	}
	if string(loaded.AuthorityBasis) != "ADVISORY_WORKSPACE" {
		t.Fatalf("AuthorityBasis = %q", loaded.AuthorityBasis)
	}
	if loaded.Digest.Algorithm != "sha256" {
		t.Fatalf("Digest.Algorithm = %q", loaded.Digest.Algorithm)
	}
	if loaded.Digest.Value != "0caf93de894f6eddcf35cb31c8ab2ba0024af3c4c66ef4bbfacc195752f3fbf5" {
		t.Fatalf("Digest.Value = %q", loaded.Digest.Value)
	}
}

func TestLoadLocalPolicyDigestIgnoresJSONFormattingAndObjectKeyOrder(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()

	writePolicyFile(t, rootA, `{"schema_version":"assurectl/policy/v0","requirements":[{"id":"unit-tests","evidence_type":"test-result","waivable":false,"allowed_producer_types":["local-user"],"max_age_seconds":3600}]}`)
	writePolicyFile(t, rootB, `{
  "requirements": [{"max_age_seconds":3600,"allowed_producer_types":["local-user"],"waivable":false,"evidence_type":"test-result","id":"unit-tests"}],
  "schema_version": "assurectl/policy/v0"
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

func TestLoadLocalRejectsMalformedOrAmbiguousPolicies(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name: "self asserted trust",
			content: `{
  "schema_version":"assurectl/policy/v0",
  "trust_status":"TRUSTED",
  "requirements":[{"id":"unit-tests","evidence_type":"test-result","waivable":false,"allowed_producer_types":["local-user"]}]
}`,
			wantErr: "unknown field",
		},
		{
			name: "unsupported schema",
			content: `{
  "schema_version":"assurectl/policy/v9",
  "requirements":[{"id":"unit-tests","evidence_type":"test-result","waivable":false,"allowed_producer_types":["local-user"]}]
}`,
			wantErr: "schema_version",
		},
		{
			name: "missing waivable",
			content: `{
  "schema_version":"assurectl/policy/v0",
  "requirements":[{"id":"unit-tests","evidence_type":"test-result","allowed_producer_types":["local-user"]}]
}`,
			wantErr: "waivable is required",
		},
		{
			name: "null waivable",
			content: `{
  "schema_version":"assurectl/policy/v0",
  "requirements":[{"id":"unit-tests","evidence_type":"test-result","waivable":null,"allowed_producer_types":["local-user"]}]
}`,
			wantErr: "waivable must be a boolean",
		},
		{
			name: "null max age",
			content: `{
  "schema_version":"assurectl/policy/v0",
  "requirements":[{"id":"unit-tests","evidence_type":"test-result","waivable":false,"allowed_producer_types":["local-user"],"max_age_seconds":null}]
}`,
			wantErr: "max_age_seconds must be a non-negative integer",
		},
		{
			name: "negative max age",
			content: `{
  "schema_version":"assurectl/policy/v0",
  "requirements":[{"id":"unit-tests","evidence_type":"test-result","waivable":false,"allowed_producer_types":["local-user"],"max_age_seconds":-1}]
}`,
			wantErr: "max_age_seconds must be a non-negative integer",
		},
		{
			name: "duplicate json key",
			content: `{
  "schema_version":"assurectl/policy/v0",
  "requirements":[{"id":"unit-tests","id":"unit-tests","evidence_type":"test-result","waivable":false,"allowed_producer_types":["local-user"]}]
}`,
			wantErr: "duplicate object key",
		},
		{
			name: "duplicate requirement id",
			content: `{
  "schema_version":"assurectl/policy/v0",
  "requirements":[
    {"id":"unit-tests","evidence_type":"test-result","waivable":false,"allowed_producer_types":["local-user"]},
    {"id":"unit-tests","evidence_type":"test-result","waivable":true,"allowed_producer_types":["github-actions"]}
  ]
}`,
			wantErr: "duplicate requirement id",
		},
		{
			name: "duplicate producer type",
			content: `{
  "schema_version":"assurectl/policy/v0",
  "requirements":[{"id":"unit-tests","evidence_type":"test-result","waivable":false,"allowed_producer_types":["local-user","local-user"]}]
}`,
			wantErr: "duplicate allowed producer type",
		},
		{
			name: "nested unknown field",
			content: `{
  "schema_version":"assurectl/policy/v0",
  "requirements":[{"id":"unit-tests","evidence_type":"test-result","waivable":false,"allowed_producer_types":["local-user"],"trusted":true}]
}`,
			wantErr: "unknown field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writePolicyFile(t, root, tt.content)

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

func TestLoadLocalRejectsMissingPolicy(t *testing.T) {
	_, err := LoadLocal(t.TempDir())
	if err == nil {
		t.Fatal("LoadLocal() error = nil")
	}
}

func writePolicyFile(t *testing.T, root, content string) {
	t.Helper()

	dir := filepath.Join(root, ".assurectl")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "policy.json"), []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
