package contract

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kefyusuf/assurectl/internal/inputmeta"
)

func baseRunGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

const baseContractBody = `{
  "schema_version": "assurectl/verification-contract/v0",
  "work_unit": {"id": "wu-001", "type": "commit", "objective": "base contract"},
  "acceptance_criteria": [{"id": "crit-a", "statement": "passes", "requirements": ["req-a"]}]
}`

func baseFixture(t *testing.T, body string) (string, string) {
	t.Helper()
	root := t.TempDir()
	baseRunGit(t, root, "init", "--initial-branch=main")
	baseRunGit(t, root, "config", "user.name", "AssureCTL Test")
	baseRunGit(t, root, "config", "user.email", "assurectl-test@example.invalid")
	if err := os.MkdirAll(filepath.Join(root, ".assurectl"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".assurectl", "contract.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	baseRunGit(t, root, "add", ".assurectl/contract.json")
	baseRunGit(t, root, "commit", "-m", "fixture: contract", "-q")
	return root, baseRunGit(t, root, "rev-parse", "HEAD")
}

func TestLoadBaseMarksTrustedProvenance(t *testing.T) {
	t.Parallel()

	root, revision := baseFixture(t, baseContractBody)
	loaded, err := LoadBase(context.Background(), root, revision)
	if err != nil {
		t.Fatalf("LoadBase() error = %v", err)
	}
	if loaded.TrustStatus != inputmeta.TrustStatusTrusted {
		t.Fatalf("TrustStatus = %q, want TRUSTED", loaded.TrustStatus)
	}
	if loaded.Source != "base:"+revision+":.assurectl/contract.json" {
		t.Fatalf("Source = %q", loaded.Source)
	}
	if loaded.Contract.WorkUnit.ID != "wu-001" {
		t.Fatalf("WorkUnit = %#v", loaded.Contract.WorkUnit)
	}
}

func TestLoadBaseFailsClosedWithoutBaseDocument(t *testing.T) {
	t.Parallel()

	_, revision := baseFixture(t, baseContractBody)
	missing := t.TempDir()
	if _, err := LoadBase(context.Background(), missing, revision); err == nil {
		t.Fatalf("LoadBase() accepted a worktree without the base contract")
	}
}
