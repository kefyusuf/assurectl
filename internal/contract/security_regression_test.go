package contract

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const validContractFixture = `{"schema_version":"assurectl/verification-contract/v0","work_unit":{"id":"wu-1","type":"commit","objective":"Verify change."},"acceptance_criteria":[{"id":"AC-1","statement":"Tests pass.","requirements":["unit-tests"]}]}`

func TestLoadLocalRejectsContractResolvedOutsideWorkspace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires privileges on some Windows runners")
	}

	root := t.TempDir()
	outside := t.TempDir()
	writeContractFile(t, outside, validContractFixture)

	dir := filepath.Join(root, ".assurectl")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.Symlink(filepath.Join(outside, ".assurectl", "contract.json"), filepath.Join(dir, "contract.json")); err != nil {
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

func TestLoadLocalRejectsOversizedContract(t *testing.T) {
	root := t.TempDir()
	writeContractFile(t, root, validContractFixture+strings.Repeat(" ", 2<<20))

	_, err := LoadLocal(root)
	if err == nil {
		t.Fatal("LoadLocal() error = nil")
	}
	if !strings.Contains(err.Error(), "exceeds maximum size") {
		t.Fatalf("LoadLocal() error = %q, want exceeds maximum size", err)
	}
}
