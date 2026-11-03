package verify

import (
	"context"
	"testing"

	"github.com/kefyusuf/assurectl/internal/cicontext"
	"github.com/kefyusuf/assurectl/internal/domain"
)

func ciFixture(repository, sha string) *cicontext.Context {
	return &cicontext.Context{
		Repository:  repository,
		SHA:         sha,
		WorkflowRef: ".github/workflows/ci.yml@refs/pull/184/merge",
		WorkflowSHA: "2a5165d0f3f9bbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		RunID:       "1234567890",
	}
}

func TestRunEmitsCIAssertedReceipt(t *testing.T) {
	t.Parallel()

	root, head := fixtureWorkspace(t, func(string, string) string { return "github.com/acme/checkout" })
	result, err := Run(context.Background(), root, Options{
		RepositoryURI: "https://github.com/acme/checkout.git",
		BaseRef:       head,
		HeadRef:       head,
		EvaluatedAt:   buildInstantHelper(),
		CI:            ciFixture("acme/checkout", head),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Receipt.Authority != domain.AuthorityCIAsserted {
		t.Fatalf("Authority = %q, want CI_ASSERTED", result.Receipt.Authority)
	}
	if result.Receipt.CompletionDecision != domain.DecisionApproved {
		t.Fatalf("Decision = %q, want APPROVED", result.Receipt.CompletionDecision)
	}
	if result.Receipt.Subject.HeadRevision != head {
		t.Fatalf("HeadRevision = %q, want the resolved head", result.Receipt.Subject.HeadRevision)
	}
}

func TestRunFailsWhenCIRepositoryMismatches(t *testing.T) {
	t.Parallel()

	root, head := fixtureWorkspace(t, func(string, string) string { return "github.com/acme/checkout" })
	if _, err := Run(context.Background(), root, Options{
		RepositoryURI: "https://github.com/acme/checkout.git",
		BaseRef:       head,
		HeadRef:       head,
		EvaluatedAt:   buildInstantHelper(),
		CI:            ciFixture("acme/other", head),
	}); err == nil {
		t.Fatalf("Run() accepted a mismatching CI repository")
	}
}

func TestRunFailsWhenCIShaMismatches(t *testing.T) {
	t.Parallel()

	root, head := fixtureWorkspace(t, func(string, string) string { return "github.com/acme/checkout" })
	if _, err := Run(context.Background(), root, Options{
		RepositoryURI: "https://github.com/acme/checkout.git",
		BaseRef:       head,
		HeadRef:       head,
		EvaluatedAt:   buildInstantHelper(),
		CI:            ciFixture("acme/checkout", "cccccccccccccccccccccccccccccccccccccccc"),
	}); err == nil {
		t.Fatalf("Run() accepted a mismatching CI SHA")
	}
}

func TestRunWithoutCIStaysAdvisory(t *testing.T) {
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
	if result.Receipt.Authority != domain.AuthorityAdvisory {
		t.Fatalf("Authority = %q, want ADVISORY without CI context", result.Receipt.Authority)
	}
}
