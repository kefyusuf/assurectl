package ci_test

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
	if count != 1 {
		t.Fatalf("JSON example count = %d, want 1", count)
	}
}

func TestEvidenceArtifactDigestMatches(t *testing.T) {
	envelopeData, err := os.ReadFile("evidence/ci-tests.json")
	if err != nil {
		t.Fatalf("os.ReadFile(envelope) error = %v", err)
	}
	var envelope struct {
		Producer struct {
			Type             string `json:"type"`
			Identity         string `json:"identity"`
			Workflow         string `json:"workflow"`
			WorkflowRevision string `json:"workflow_revision"`
		} `json:"producer"`
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
	if envelope.Producer.Type != "github-actions" {
		t.Fatalf("producer type = %q, want github-actions", envelope.Producer.Type)
	}
	if envelope.Producer.Workflow == "" || len(envelope.Producer.WorkflowRevision) != 40 {
		t.Fatalf("workflow identity = %q / %q", envelope.Producer.Workflow, envelope.Producer.WorkflowRevision)
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
