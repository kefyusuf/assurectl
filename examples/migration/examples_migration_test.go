package migration_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestExampleJSONHasSchemaVersion(t *testing.T) {
	var count int
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		count++
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var value map[string]any
		if err := json.Unmarshal(data, &value); err != nil {
			t.Errorf("json.Unmarshal(%q) error = %v", path, err)
			return nil
		}
		if version, ok := value["schema_version"].(string); !ok || version == "" {
			t.Errorf("%s: schema_version = %#v, want non-empty string", path, value["schema_version"])
		}
		return nil
	})
	if err != nil {
		t.Fatalf("filepath.WalkDir() error = %v", err)
	}
	if count != 3 {
		t.Fatalf("JSON example count = %d, want 3 (contract, policy, evidence)", count)
	}
}

func TestWorkUnitTypeIsMigration(t *testing.T) {
	data, err := os.ReadFile("contract.json")
	if err != nil {
		t.Fatalf("os.ReadFile(contract) error = %v", err)
	}
	var contract struct {
		WorkUnit struct {
			Type string `json:"type"`
		} `json:"work_unit"`
	}
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatal(err)
	}
	if contract.WorkUnit.Type != "dependency_upgrade" {
		t.Fatalf("work unit type = %q, want dependency_upgrade", contract.WorkUnit.Type)
	}
}

func TestEvidenceArtifactDigestMatches(t *testing.T) {
	envelopeData, err := os.ReadFile("evidence/dep-upgrade.json")
	if err != nil {
		t.Fatalf("os.ReadFile(envelope) error = %v", err)
	}
	var envelope struct {
		Artifact struct {
			URI    string `json:"uri"`
			Digest struct {
				Algorithm string `json:"algorithm"`
				Value     string `json:"value"`
			} `json:"digest"`
		} `json:"artifact"`
	}
	if err := json.Unmarshal(envelopeData, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Artifact.Digest.Algorithm != "sha256" {
		t.Fatalf("digest algorithm = %q, want sha256", envelope.Artifact.Digest.Algorithm)
	}
	artifactData, err := os.ReadFile(filepath.Join("evidence", filepath.FromSlash(envelope.Artifact.URI)))
	if err != nil {
		t.Fatalf("os.ReadFile(artifact) error = %v", err)
	}
	sum := sha256.Sum256(artifactData)
	if got := hex.EncodeToString(sum[:]); got != envelope.Artifact.Digest.Value {
		t.Fatalf("artifact digest = %s, want %s", got, envelope.Artifact.Digest.Value)
	}
}
