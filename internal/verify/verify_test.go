package verify

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/gitsubject"
	"github.com/kefyusuf/assurectl/internal/inputmeta"
)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, output)
	}
	return strings.TrimSpace(string(output))
}

func initRepository(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init", "--initial-branch=main")
	runGit(t, dir, "config", "user.name", "AssureCTL Test")
	runGit(t, dir, "config", "user.email", "assurectl-test@example.invalid")
}

func commitFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", name)
	runGit(t, dir, "commit", "-m", "fixture: "+name)
	return runGit(t, dir, "rev-parse", "HEAD")
}

const fixtureContract = `{
  "schema_version": "assurectl/verification-contract/v0",
  "work_unit": {"id": "wu-001", "type": "commit", "objective": "fixture"},
  "acceptance_criteria": [{"id": "crit-a", "statement": "passes", "requirements": ["req-a"]}]
}`

const fixturePolicy = `{
  "schema_version": "assurectl/policy/v0",
  "requirements": [{"id": "req-a", "evidence_type": "test-result", "waivable": false, "allowed_producer_types": ["local-user"]}]
}`

// fixtureWorkspace materializes a temporary Git worktree with a contract, a
// policy, and one valid evidence envelope. repositoryFor resolves the
// repository identity the envelope binds, computed after the head exists.
func fixtureWorkspace(t *testing.T, repositoryFor func(root, head string) string) (string, string) {
	t.Helper()

	root := t.TempDir()
	initRepository(t, root)
	// The .assurectl control directory is ignored so the fixture worktree
	// stays clean: dirty state is covered by its own dedicated test.
	head := commitFile(t, root, ".gitignore", ".assurectl/\n")
	repositoryURI := repositoryFor(root, head)

	assurectl := filepath.Join(root, ".assurectl")
	evidenceDir := filepath.Join(assurectl, "evidence", "artifacts")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	artifact := []byte("artifact-bytes")
	if err := os.WriteFile(filepath.Join(evidenceDir, "unit.json"), artifact, 0o644); err != nil {
		t.Fatal(err)
	}
	artifactDigest := inputmeta.SHA256(artifact)

	envelope := fmt.Sprintf(`{
  "schema_version": "assurectl/evidence-envelope/v0",
  "id": "ev-unit",
  "type": "test-result",
  "subject": {"repository_uri": %q, "revision": %q},
  "producer": {"type": "local-user", "identity": "fixture"},
  "invocation": {"command_id": "test.unit", "started_at": "2026-11-01T09:00:00Z", "finished_at": "2026-11-01T09:01:00Z"},
  "outcome": {"status": "PASSED", "exit_code": 0},
  "artifact": {"uri": "artifacts/unit.json", "digest": {"algorithm": "sha256", "value": %q}, "media_type": "application/json"}
}`, repositoryURI, head, artifactDigest.Value)

	if err := os.WriteFile(filepath.Join(assurectl, "contract.json"), []byte(fixtureContract), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assurectl, "policy.json"), []byte(fixturePolicy), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "..", "unit-tests.json"), []byte(envelope), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, head
}

func TestRunHappyPath(t *testing.T) {
	t.Parallel()

	root, head := fixtureWorkspace(t, func(string, string) string { return "github.com/acme/checkout" })
	result, err := Run(context.Background(), root, Options{
		RepositoryURI: "https://github.com/acme/checkout.git",
		BaseRef:       head,
		HeadRef:       head,
		EvaluatedAt:   buildInstantHelper(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Receipt.CompletionDecision != domain.DecisionApproved {
		t.Fatalf("Decision = %q, want APPROVED", result.Receipt.CompletionDecision)
	}
	if result.Receipt.Subject.HeadRevision != head {
		t.Fatalf("HeadRevision = %q, want %q", result.Receipt.Subject.HeadRevision, head)
	}
	for _, finding := range result.Receipt.Findings {
		if finding.Code == "SUBJECT_DIRTY_WORKTREE" || finding.Code == "SUBJECT_LOCAL_ONLY" {
			t.Fatalf("unexpected diagnostic finding %q", finding.Code)
		}
	}
}

func buildInstantHelper() time.Time {
	return time.Date(2026, 11, 2, 10, 30, 0, 0, time.UTC)
}

func TestRunReportsDirtyWorktreeFinding(t *testing.T) {
	t.Parallel()

	root, head := fixtureWorkspace(t, func(string, string) string { return "github.com/acme/checkout" })
	if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Run(context.Background(), root, Options{
		RepositoryURI: "https://github.com/acme/checkout.git",
		BaseRef:       head,
		HeadRef:       head,
		EvaluatedAt:   buildInstantHelper(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	dirtySeen := false
	for _, finding := range result.Receipt.Findings {
		if finding.Code == "SUBJECT_DIRTY_WORKTREE" && !finding.Blocking {
			dirtySeen = true
		}
	}
	if !dirtySeen {
		t.Fatalf("SUBJECT_DIRTY_WORKTREE finding missing: %#v", result.Receipt.Findings)
	}
}

func TestRunReportsLocalOnlyIdentityFinding(t *testing.T) {
	t.Parallel()

	root, head := fixtureWorkspace(t, func(root, head string) string {
		resolution, err := gitsubject.Resolve(context.Background(), root, gitsubject.Options{
			BaseRef: head,
			HeadRef: head,
		})
		if err != nil {
			t.Fatalf("Resolve() for fixture: %v", err)
		}
		return resolution.Subject.RepositoryURI
	})
	result, err := Run(context.Background(), root, Options{
		BaseRef:     head,
		HeadRef:     head,
		EvaluatedAt: buildInstantHelper(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	localOnlySeen := false
	for _, finding := range result.Receipt.Findings {
		if finding.Code == "SUBJECT_LOCAL_ONLY" && !finding.Blocking {
			localOnlySeen = true
		}
	}
	if !localOnlySeen {
		t.Fatalf("SUBJECT_LOCAL_ONLY finding missing: %#v", result.Receipt.Findings)
	}
}

func TestRunFailsOnRepositoryMismatch(t *testing.T) {
	t.Parallel()

	root, head := fixtureWorkspace(t, func(string, string) string { return "github.com/other/repo" })
	if _, err := Run(context.Background(), root, Options{
		RepositoryURI: "https://github.com/acme/checkout.git",
		BaseRef:       head,
		HeadRef:       head,
		EvaluatedAt:   buildInstantHelper(),
	}); err == nil {
		t.Fatalf("Run() accepted evidence bound to another repository")
	}
}

func TestRunFailsOnEmptyPolicy(t *testing.T) {
	t.Parallel()

	root, head := fixtureWorkspace(t, func(string, string) string { return "github.com/acme/checkout" })
	if err := os.WriteFile(filepath.Join(root, ".assurectl", "policy.json"),
		[]byte(`{"schema_version": "assurectl/policy/v0", "requirements": []}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(context.Background(), root, Options{
		RepositoryURI: "https://github.com/acme/checkout.git",
		BaseRef:       head,
		HeadRef:       head,
		EvaluatedAt:   buildInstantHelper(),
	}); err == nil {
		t.Fatalf("Run() accepted an empty policy")
	}
}

func TestRunFailsOnMissingContract(t *testing.T) {
	t.Parallel()

	root, head := fixtureWorkspace(t, func(string, string) string { return "github.com/acme/checkout" })
	if err := os.Remove(filepath.Join(root, ".assurectl", "contract.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(context.Background(), root, Options{
		RepositoryURI: "https://github.com/acme/checkout.git",
		BaseRef:       head,
		HeadRef:       head,
		EvaluatedAt:   buildInstantHelper(),
	}); err == nil {
		t.Fatalf("Run() accepted a missing contract")
	}
}
