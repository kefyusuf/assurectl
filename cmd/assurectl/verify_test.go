package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kefyusuf/assurectl/internal/inputmeta"
)

func cliRunGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func cliFixtureWorkspace(t *testing.T) (string, string) {
	t.Helper()

	root := t.TempDir()
	cliRunGit(t, root, "init", "--initial-branch=main")
	cliRunGit(t, root, "config", "user.name", "AssureCTL Test")
	cliRunGit(t, root, "config", "user.email", "assurectl-test@example.invalid")
	head := cliRunGit(t, root, "commit", "--allow-empty", "-m", "fixture", "-q")

	assurectl := filepath.Join(root, ".assurectl")
	if err := os.MkdirAll(filepath.Join(assurectl, "evidence", "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".assurectl/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cliRunGit(t, root, "add", ".gitignore")
	cliRunGit(t, root, "commit", "-m", "fixture: gitignore", "-q")
	head = cliRunGit(t, root, "rev-parse", "HEAD")

	artifact := []byte("artifact-bytes")
	artifactPath := filepath.Join(assurectl, "evidence", "artifacts", "unit.json")
	if err := os.WriteFile(artifactPath, artifact, 0o644); err != nil {
		t.Fatal(err)
	}
	artifactDigest := inputmeta.SHA256(artifact)
	envelope := `{"schema_version": "assurectl/evidence-envelope/v0", "id": "ev-unit", "type": "test-result", ` +
		`"subject": {"repository_uri": "https://github.com/acme/checkout.git", "revision": "` + head + `"}, ` +
		`"producer": {"type": "local-user", "identity": "fixture"}, ` +
		`"invocation": {"command_id": "test.unit", "started_at": "2026-11-01T09:00:00Z", "finished_at": "2026-11-01T09:01:00Z"}, ` +
		`"outcome": {"status": "PASSED", "exit_code": 0}, ` +
		`"artifact": {"uri": "artifacts/unit.json", "digest": {"algorithm": "sha256", "value": "` + artifactDigest.Value + `"}, "media_type": "application/json"}}`
	if err := os.WriteFile(filepath.Join(assurectl, "evidence", "unit-tests.json"), []byte(envelope), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assurectl, "contract.json"), []byte(cliFixtureContract), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assurectl, "policy.json"), []byte(cliFixturePolicy), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, head
}

const cliFixtureContract = `{
  "schema_version": "assurectl/verification-contract/v0",
  "work_unit": {"id": "wu-001", "type": "commit", "objective": "fixture"},
  "acceptance_criteria": [{"id": "crit-a", "statement": "passes", "requirements": ["req-a"]}]
}`

const cliFixturePolicy = `{
  "schema_version": "assurectl/policy/v0",
  "requirements": [{"id": "req-a", "evidence_type": "test-result", "waivable": false, "allowed_producer_types": ["local-user"]}]
}`

func TestVerifyUsageErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "missing base flag", args: []string{"verify"}},
		{name: "missing flag value", args: []string{"verify", "--base"}},
		{name: "unknown flag", args: []string{"verify", "--base", "HEAD", "--nope"}},
		{name: "positional argument", args: []string{"verify", "extra"}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			if code := run(tt.args, stdout, stderr); code != 64 {
				t.Fatalf("run() exit = %d, want 64", code)
			}
		})
	}
}

func TestVerifyHappyPath(t *testing.T) {

	root, head := cliFixtureWorkspace(t)
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := run([]string{"verify", "--base", head, "--head", head, "--repository", "https://github.com/acme/checkout.git"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run() exit = %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"schema_version":"assurectl/receipt/v0"`) {
		t.Fatalf("stdout = %q, want the receipt JSON", stdout.String())
	}
	if !strings.Contains(stderr.String(), "APPROVED") {
		t.Fatalf("stderr = %q, want the decision summary", stderr.String())
	}
}

func TestVerifyExitCodeMapping(t *testing.T) {

	root, head := cliFixtureWorkspace(t)
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	// No evidence at all: the requirement is MISSING, which blocks.
	noEvidence := filepath.Join(root, ".assurectl", "evidence", "unit-tests.json")
	if err := os.Remove(noEvidence); err != nil {
		t.Fatal(err)
	}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	if code := run([]string{"verify", "--base", head, "--head", head, "--repository", "https://github.com/acme/checkout.git"}, stdout, stderr); code != 2 {
		t.Fatalf("run() exit = %d, want 2 for BLOCKED", code)
	}
	// A completed pipeline prints its receipt even when the decision is
	// BLOCKED; only pipeline errors leave stdout empty.
	if !strings.Contains(stdout.String(), `"completion_decision":"BLOCKED"`) {
		t.Fatalf("stdout = %q, want the BLOCKED receipt JSON", stdout.String())
	}
}

func TestVerifyRejectedDecisionExitsOne(t *testing.T) {
	root, head := cliFixtureWorkspace(t)
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	envelopePath := filepath.Join(root, ".assurectl", "evidence", "unit-tests.json")
	envelope, err := os.ReadFile(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	failed := bytes.Replace(envelope, []byte(`"status": "PASSED"`), []byte(`"status": "FAILED"`), 1)
	if err := os.WriteFile(envelopePath, failed, 0o644); err != nil {
		t.Fatal(err)
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	if code := run([]string{"verify", "--base", head, "--head", head, "--repository", "https://github.com/acme/checkout.git"}, stdout, stderr); code != 1 {
		t.Fatalf("run() exit = %d, want 1 for REJECTED (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"completion_decision":"REJECTED"`) {
		t.Fatalf("stdout = %q, want the REJECTED receipt JSON", stdout.String())
	}
}

func TestVerifyPipelineErrorExitsTwo(t *testing.T) {

	root, head := cliFixtureWorkspace(t)
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	if err := os.Remove(filepath.Join(root, ".assurectl", "contract.json")); err != nil {
		t.Fatal(err)
	}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	if code := run([]string{"verify", "--base", head, "--head", head, "--repository", "https://github.com/acme/checkout.git"}, stdout, stderr); code != 2 {
		t.Fatalf("run() exit = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on error", stdout.String())
	}
	if !strings.Contains(stderr.String(), "verify:") {
		t.Fatalf("stderr = %q, want diagnostics", stderr.String())
	}
}
