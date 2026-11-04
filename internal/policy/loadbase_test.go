package policy

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

func baseFixture(t *testing.T, body string) (string, string) {
	t.Helper()
	root := t.TempDir()
	baseRunGit(t, root, "init", "--initial-branch=main")
	baseRunGit(t, root, "config", "user.name", "AssureCTL Test")
	baseRunGit(t, root, "config", "user.email", "assurectl-test@example.invalid")
	if err := os.MkdirAll(filepath.Join(root, ".assurectl"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".assurectl", "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	baseRunGit(t, root, "add", ".assurectl/policy.json")
	baseRunGit(t, root, "commit", "-m", "fixture: policy", "-q")
	return root, baseRunGit(t, root, "rev-parse", "HEAD")
}

const basePolicyBody = `{
  "schema_version": "assurectl/policy/v0",
  "requirements": [{"id": "req-a", "evidence_type": "test-result", "waivable": false, "allowed_producer_types": ["local-user"]}]
}`

func TestLoadBaseMarksTrustedBaseProvenance(t *testing.T) {
	t.Parallel()

	root, revision := baseFixture(t, basePolicyBody)
	loaded, err := LoadBase(context.Background(), root, revision)
	if err != nil {
		t.Fatalf("LoadBase() error = %v", err)
	}
	if loaded.TrustStatus != inputmeta.TrustStatusTrusted || loaded.AuthorityBasis != inputmeta.AuthorityBasisTrustedBase {
		t.Fatalf("trust = %q / %q, want TRUSTED / TRUSTED_BASE", loaded.TrustStatus, loaded.AuthorityBasis)
	}
	if loaded.Source != "base:"+revision+":.assurectl/policy.json" {
		t.Fatalf("Source = %q", loaded.Source)
	}
	if len(loaded.Policy.Requirements) != 1 || loaded.Policy.Requirements[0].ID != "req-a" {
		t.Fatalf("Requirements = %#v", loaded.Policy.Requirements)
	}
}

func TestLoadBaseDigestMatchesLocalForSameBytes(t *testing.T) {
	t.Parallel()

	root, revision := baseFixture(t, basePolicyBody)
	fromBase, err := LoadBase(context.Background(), root, revision)
	if err != nil {
		t.Fatalf("LoadBase() error = %v", err)
	}

	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".assurectl"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".assurectl", "policy.json"), []byte(basePolicyBody), 0o644); err != nil {
		t.Fatal(err)
	}
	fromLocal, err := LoadLocal(workspace)
	if err != nil {
		t.Fatalf("LoadLocal() error = %v", err)
	}
	if fromBase.Digest != fromLocal.Digest {
		t.Fatalf("digests differ for identical bytes: base=%#v local=%#v", fromBase.Digest, fromLocal.Digest)
	}
}

func headOf(t *testing.T, dir string) string {
	t.Helper()
	return baseRunGit(t, dir, "rev-parse", "HEAD")
}

func TestLoadBaseFailsClosed(t *testing.T) {
	t.Parallel()

	_, revision := baseFixture(t, basePolicyBody)

	missing := t.TempDir()
	if _, err := LoadBase(context.Background(), missing, revision); err == nil {
		t.Fatalf("LoadBase() accepted a worktree without the base document")
	}
}
