package basic_test

import (
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
	if count != 4 {
		t.Fatalf("JSON example count = %d, want 4", count)
	}
}

func TestEvidenceArtifactDigestMatches(t *testing.T) {
	envelopeData, err := os.ReadFile("evidence/unit-tests.json")
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
		t.Fatalf("json.Unmarshal(envelope) error = %v", err)
	}
	if envelope.Artifact.Digest.Algorithm != "sha256" {
		t.Fatalf("digest algorithm = %q, want sha256", envelope.Artifact.Digest.Algorithm)
	}
	artifactData, err := os.ReadFile(filepath.Join("evidence", filepath.FromSlash(envelope.Artifact.URI)))
	if err != nil {
		t.Fatalf("os.ReadFile(artifact) error = %v", err)
	}
	got := sha256Hex(artifactData)
	if got != envelope.Artifact.Digest.Value {
		t.Fatalf("artifact digest = %s, want %s", got, envelope.Artifact.Digest.Value)
	}
}
