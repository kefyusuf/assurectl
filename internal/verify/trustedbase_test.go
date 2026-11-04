package verify

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/inputmeta"
)

const (
	basePolicyBody = `{
  "schema_version": "assurectl/policy/v0",
  "requirements": [{"id": "req-a", "evidence_type": "test-result", "waivable": false, "allowed_producer_types": ["local-user"]}]
}`
	candidatePolicyBody = `{
  "schema_version": "assurectl/policy/v0",
  "requirements": [{"id": "req-candidate", "evidence_type": "other-result", "waivable": true, "allowed_producer_types": ["anyone"]}]
}`
	trustedBaseContractBody = `{
  "schema_version": "assurectl/verification-contract/v0",
  "work_unit": {"id": "wu-001", "type": "commit", "objective": "trusted base"},
  "acceptance_criteria": [{"id": "crit-a", "statement": "passes", "requirements": ["req-a"]}]
}`
)

// trustedBaseFixture commits policy and contract at the base revision, then
// a candidate commit whose workspace policy differs from the base's.
func trustedBaseFixture(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "--initial-branch=main")
	runGit(t, root, "config", "user.name", "AssureCTL Test")
	runGit(t, root, "config", "user.email", "assurectl-test@example.invalid")
	assurectl := filepath.Join(root, ".assurectl")
	if err := os.MkdirAll(assurectl, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".assurectl/evidence/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assurectl, "policy.json"), []byte(basePolicyBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assurectl, "contract.json"), []byte(trustedBaseContractBody), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".gitignore", ".assurectl/policy.json", ".assurectl/contract.json")
	runGit(t, root, "commit", "-m", "base", "-q")
	baseRevision := runGit(t, root, "rev-parse", "HEAD")

	// The candidate weakens its own workspace policy; trusted-base mode
	// must ignore it.
	if err := os.WriteFile(filepath.Join(assurectl, "policy.json"), []byte(candidatePolicyBody), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".assurectl/policy.json")
	runGit(t, root, "commit", "-m", "candidate", "-q")
	headRevision := runGit(t, root, "rev-parse", "HEAD")

	// Evidence binds the candidate head; ingestion reads it from the
	// (ignored) evidence directory.
	if err := os.MkdirAll(filepath.Join(assurectl, "evidence", "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assurectl, "evidence", "artifacts", "unit.json"), []byte("artifact-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	artifactDigest := inputmeta.SHA256([]byte("artifact-bytes"))
	envelope := fmt.Sprintf(`{"schema_version":"assurectl/evidence-envelope/v0","id":"ev-unit","type":"test-result","subject":{"repository_uri":"github.com/acme/checkout","revision":%q},"producer":{"type":"local-user","identity":"fixture"},"invocation":{"command_id":"test.unit","started_at":"2026-11-01T09:00:00Z","finished_at":"2026-11-01T09:01:00Z"},"outcome":{"status":"PASSED","exit_code":0},"artifact":{"uri":"artifacts/unit.json","digest":{"algorithm":"sha256","value":%q},"media_type":"application/json"}}`,
		headRevision, artifactDigest.Value)
	if err := os.WriteFile(filepath.Join(assurectl, "evidence", "unit-tests.json"), []byte(envelope), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, baseRevision, headRevision
}

func TestRunTrustedBaseResolvesFromBase(t *testing.T) {
	t.Parallel()

	root, baseRevision, headRevision := trustedBaseFixture(t)
	result, err := Run(context.Background(), root, Options{
		RepositoryURI: "https://github.com/acme/checkout.git",
		BaseRef:       baseRevision,
		HeadRef:       headRevision,
		EvaluatedAt:   buildInstantHelper(),
		TrustedBase:   true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Receipt.Authority != domain.AuthorityAuthoritative {
		t.Fatalf("Authority = %q, want AUTHORITATIVE", result.Receipt.Authority)
	}
	projectSource := ""
	for _, source := range result.Receipt.Policy.Sources {
		if source.Layer == "project" {
			projectSource = source.Source
		}
	}
	if projectSource != "base:"+baseRevision+":.assurectl/policy.json" {
		t.Fatalf("project policy source = %q, want the base revision", projectSource)
	}
	if result.Receipt.CompletionDecision != domain.DecisionApproved {
		t.Fatalf("Decision = %q, want APPROVED under the base gate", result.Receipt.CompletionDecision)
	}
}

func TestRunTrustedBaseFailsWithoutBasePolicy(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runGit(t, root, "init", "--initial-branch=main")
	runGit(t, root, "config", "user.name", "AssureCTL Test")
	runGit(t, root, "config", "user.email", "assurectl-test@example.invalid")
	runGit(t, root, "commit", "--allow-empty", "-m", "base", "-q")
	revision := runGit(t, root, "rev-parse", "HEAD")

	if _, err := Run(context.Background(), root, Options{
		BaseRef:     revision,
		HeadRef:     revision,
		EvaluatedAt: buildInstantHelper(),
		TrustedBase: true,
	}); err == nil {
		t.Fatalf("Run() accepted a base without a policy document")
	}
}

func TestRunTrustedBaseDominatesCIContext(t *testing.T) {
	t.Parallel()

	root, baseRevision, headRevision := trustedBaseFixture(t)
	result, err := Run(context.Background(), root, Options{
		RepositoryURI: "https://github.com/acme/checkout.git",
		BaseRef:       baseRevision,
		HeadRef:       headRevision,
		EvaluatedAt:   buildInstantHelper(),
		TrustedBase:   true,
		CI:            ciFixture("acme/checkout", headRevision),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Receipt.Authority != domain.AuthorityAuthoritative {
		t.Fatalf("Authority = %q, want AUTHORITATIVE to dominate", result.Receipt.Authority)
	}
}
