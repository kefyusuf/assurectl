package policy

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const validPolicyFixture = `{"schema_version":"assurectl/policy/v0","requirements":[{"id":"unit-tests","evidence_type":"test-result","waivable":false,"allowed_producer_types":["local-user"],"max_age_seconds":3600}]}`

func TestLoadLocalRejectsPolicyResolvedOutsideWorkspace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires privileges on some Windows runners")
	}

	root := t.TempDir()
	outside := t.TempDir()
	writePolicyFile(t, outside, validPolicyFixture)

	dir := filepath.Join(root, ".assurectl")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.Symlink(filepath.Join(outside, ".assurectl", "policy.json"), filepath.Join(dir, "policy.json")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	_, err := LoadLocal(root)
	if err == nil {
		t.Fatal("LoadLocal() error = nil")
	}
	if !strings.Contains(err.Error(), "outside workspace") {
		t.Fatalf("LoadLocal() error = %q, want outside workspace", err)
	}
}

func TestLoadLocalRejectsOversizedPolicy(t *testing.T) {
	root := t.TempDir()
	writePolicyFile(t, root, validPolicyFixture+strings.Repeat(" ", 2<<20))

	_, err := LoadLocal(root)
	if err == nil {
		t.Fatal("LoadLocal() error = nil")
	}
	if !strings.Contains(err.Error(), "exceeds maximum size") {
		t.Fatalf("LoadLocal() error = %q, want exceeds maximum size", err)
	}
}
