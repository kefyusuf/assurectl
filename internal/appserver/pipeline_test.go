package appserver

import (
	"context"
	"encoding/json"
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

func skeletonRunGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

const skeletonBasePolicy = `{
  "schema_version": "assurectl/policy/v0",
  "requirements": [{"id": "req-a", "evidence_type": "test-result", "waivable": false, "allowed_producer_types": ["local-user"]}]
}`

const skeletonBaseContract = `{
  "schema_version": "assurectl/verification-contract/v0",
  "work_unit": {"id": "wu-001", "type": "commit", "objective": "app skeleton fixture"},
  "acceptance_criteria": [{"id": "crit-a", "statement": "passes", "requirements": ["req-a"]}]
}`

// skeletonFixture commits the base gate documents, a candidate commit, and
// evidence binding the candidate head; it returns the repo root plus both
// revisions and the resolved canonical repository identity.
func skeletonFixture(t *testing.T, outcome domain.ObservedOutcome) (string, string, string, string) {
	t.Helper()
	root := t.TempDir()
	skeletonRunGit(t, root, "init", "--initial-branch=main")
	skeletonRunGit(t, root, "config", "user.name", "AssureCTL Test")
	skeletonRunGit(t, root, "config", "user.email", "assurectl-test@example.invalid")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".assurectl/evidence/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	assurectl := filepath.Join(root, ".assurectl")
	if err := os.MkdirAll(assurectl, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assurectl, "policy.json"), []byte(skeletonBasePolicy), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assurectl, "contract.json"), []byte(skeletonBaseContract), 0o644); err != nil {
		t.Fatal(err)
	}
	skeletonRunGit(t, root, "add", ".gitignore", ".assurectl/policy.json", ".assurectl/contract.json")
	skeletonRunGit(t, root, "commit", "-m", "base", "-q")
	base := skeletonRunGit(t, root, "rev-parse", "HEAD")

	skeletonRunGit(t, root, "commit", "--allow-empty", "-m", "candidate", "-q")
	head := skeletonRunGit(t, root, "rev-parse", "HEAD")

	resolution, err := gitsubject.Resolve(context.Background(), root, gitsubject.Options{BaseRef: base, HeadRef: head})
	if err != nil {
		t.Fatalf("resolve fixture subject: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(assurectl, "evidence", "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assurectl, "evidence", "artifacts", "unit.json"), []byte("artifact-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	artifactDigest := inputmeta.SHA256([]byte("artifact-bytes"))
	envelope := fmt.Sprintf(`{"schema_version":"assurectl/evidence-envelope/v0","id":"ev-unit","type":"test-result","subject":{"repository_uri":%q,"revision":%q},"producer":{"type":"local-user","identity":"fixture"},"invocation":{"command_id":"test.unit","started_at":"2026-11-01T09:00:00Z","finished_at":"2026-11-01T09:01:00Z"},"outcome":{"status":%q,"exit_code":0},"artifact":{"uri":"artifacts/unit.json","digest":{"algorithm":"sha256","value":%q},"media_type":"application/json"}}`,
		resolution.Subject.RepositoryURI, head, string(outcome), artifactDigest.Value)
	if err := os.WriteFile(filepath.Join(assurectl, "evidence", "unit-tests.json"), []byte(envelope), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, base, head, resolution.Subject.RepositoryURI
}

func skeletonDeps(t *testing.T, root string, ciToken string, validator TokenValidator, now time.Time) Dependencies {
	t.Helper()
	return Dependencies{
		WorktreeSource: skeletonWorktreeSource{root: root},
		EvidenceSource: skeletonEvidenceSource{root: root, token: ciToken},
		TokenValidator: validator,
		CheckPoster:    &skeletonCheckPoster{},
		Now:            func() time.Time { return now },
	}
}

type skeletonWorktreeSource struct{ root string }

func (s skeletonWorktreeSource) Worktree(ctx context.Context, installation int64, repository, revision string) (string, error) {
	return s.root, nil
}

type skeletonEvidenceSource struct {
	root  string
	token string
}

func (s skeletonEvidenceSource) EvidenceBundle(ctx context.Context, installation int64, repository string, prNumber int64, headSHA string) (string, string, error) {
	return s.root, s.token, nil
}

type skeletonCheckPoster struct {
	payloads []CheckPayload
}

func (s *skeletonCheckPoster) PostCheck(ctx context.Context, installation int64, repository string, prNumber int64, payload CheckPayload) error {
	s.payloads = append(s.payloads, payload)
	return nil
}

var skeletonInstant = time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)

func TestPipelineApprovesWithValidatedProducer(t *testing.T) {
	t.Parallel()

	root, base, head, canonical := skeletonFixture(t, domain.OutcomePassed)
	poster := &skeletonCheckPoster{}
	deps := skeletonDeps(t, root, "fixture-token", skeletonTokenValidator{claims: ValidatedClaims{Repository: canonical, SHA: head}}, skeletonInstant)
	deps.CheckPoster = poster

	outcome, err := EvaluatePullRequest(context.Background(), PullRequestEvent{
		InstallationID: 7,
		Repository:     "acme/checkout",
		Action:         "synchronize",
		PRNumber:       184,
		BaseSHA:        base,
		HeadSHA:        head,
	}, deps)
	if err != nil {
		t.Fatalf("EvaluatePullRequest() error = %v", err)
	}
	if outcome.Payload.Conclusion != "success" {
		t.Fatalf("Conclusion = %q, want success", outcome.Payload.Conclusion)
	}
	if outcome.Payload.ReceiptReference.Authority != string(domain.AuthorityAuthoritative) {
		t.Fatalf("Authority = %q, want AUTHORITATIVE", outcome.Payload.ReceiptReference.Authority)
	}
	if len(poster.payloads) != 1 || len(outcome.Payload.ReceiptReference.ReceiptSHA256) != 64 {
		t.Fatalf("posted = %d payloads, receipt digest len = %d", len(poster.payloads), len(outcome.Payload.ReceiptReference.ReceiptSHA256))
	}
}

type skeletonTokenValidator struct {
	claims ValidatedClaims
	err    error
}

func (s skeletonTokenValidator) Validate(ctx context.Context, token string, expect ExpectedClaims) (ValidatedClaims, error) {
	return s.claims, s.err
}

func TestPipelineCapsUnvalidatedProducer(t *testing.T) {
	t.Parallel()

	root, base, head, _ := skeletonFixture(t, domain.OutcomePassed)
	deps := skeletonDeps(t, root, "", skeletonTokenValidator{}, skeletonInstant)

	outcome, err := EvaluatePullRequest(context.Background(), PullRequestEvent{
		InstallationID: 7,
		Repository:     "acme/checkout",
		Action:         "synchronize",
		PRNumber:       184,
		BaseSHA:        base,
		HeadSHA:        head,
	}, deps)
	if err != nil {
		t.Fatalf("EvaluatePullRequest() error = %v", err)
	}
	if outcome.Payload.ReceiptReference.Authority != string(domain.AuthorityAdvisory) {
		t.Fatalf("Authority = %q, want the ADVISORY cap (ADR-0010)", outcome.Payload.ReceiptReference.Authority)
	}
}

func TestPipelineReportsRejected(t *testing.T) {
	t.Parallel()

	root, base, head, _ := skeletonFixture(t, domain.OutcomeFailed)
	deps := skeletonDeps(t, root, "fixture-token", skeletonTokenValidator{claims: ValidatedClaims{Repository: "github.com/acme/checkout", SHA: head}}, skeletonInstant)

	outcome, err := EvaluatePullRequest(context.Background(), PullRequestEvent{
		InstallationID: 7,
		Repository:     "acme/checkout",
		Action:         "synchronize",
		PRNumber:       184,
		BaseSHA:        base,
		HeadSHA:        head,
	}, deps)
	if err != nil {
		t.Fatalf("EvaluatePullRequest() error = %v", err)
	}
	if outcome.Payload.Conclusion != "failure" {
		t.Fatalf("Conclusion = %q, want failure", outcome.Payload.Conclusion)
	}
}

func TestPipelineFailsOnWorktreeError(t *testing.T) {
	t.Parallel()

	root, base, head, _ := skeletonFixture(t, domain.OutcomePassed)
	deps := skeletonDeps(t, root, "", skeletonTokenValidator{}, skeletonInstant)
	deps.WorktreeSource = failingWorktreeSource{}

	if _, err := EvaluatePullRequest(context.Background(), PullRequestEvent{
		InstallationID: 7,
		Repository:     "acme/checkout",
		Action:         "synchronize",
		PRNumber:       184,
		BaseSHA:        base,
		HeadSHA:        head,
	}, deps); err == nil {
		t.Fatalf("EvaluatePullRequest() accepted a worktree failure")
	}
}

type failingWorktreeSource struct{}

func (failingWorktreeSource) Worktree(ctx context.Context, installation int64, repository, revision string) (string, error) {
	return "", context.DeadlineExceeded
}

func TestPipelinePayloadsAreDeterministic(t *testing.T) {
	t.Parallel()

	root, base, head, canonical := skeletonFixture(t, domain.OutcomePassed)
	event := PullRequestEvent{
		InstallationID: 7,
		Repository:     "acme/checkout",
		Action:         "synchronize",
		PRNumber:       184,
		BaseSHA:        base,
		HeadSHA:        head,
	}

	first, err := EvaluatePullRequest(context.Background(), event, skeletonDeps(t, root, "fixture-token", skeletonTokenValidator{claims: ValidatedClaims{Repository: canonical, SHA: head}}, skeletonInstant))
	if err != nil {
		t.Fatalf("EvaluatePullRequest(first) error = %v", err)
	}
	second, err := EvaluatePullRequest(context.Background(), event, skeletonDeps(t, root, "fixture-token", skeletonTokenValidator{claims: ValidatedClaims{Repository: canonical, SHA: head}}, skeletonInstant))
	if err != nil {
		t.Fatalf("EvaluatePullRequest(second) error = %v", err)
	}

	firstBytes, err := json.Marshal(first.Payload)
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := json.Marshal(second.Payload)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstBytes) != string(secondBytes) {
		t.Fatalf("payloads differ for identical events:\n%s\n%s", firstBytes, secondBytes)
	}
}
