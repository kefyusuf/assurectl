package cicontext

import (
	"strings"
	"testing"
)

func completeEnviron() []string {
	return []string{
		"GITHUB_REPOSITORY=acme/checkout",
		"GITHUB_SHA=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"GITHUB_REF=refs/pull/184/merge",
		"GITHUB_WORKFLOW_REF=.github/workflows/ci.yml@refs/pull/184/merge",
		"GITHUB_WORKFLOW_SHA=2a5165d0f3f9bbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"GITHUB_RUN_ID=1234567890",
		"GITHUB_RUN_ATTEMPT=1",
		"GITHUB_EVENT_NAME=pull_request",
	}
}

func TestReadMapsGitHubActionsContext(t *testing.T) {
	t.Parallel()

	context, err := Read(completeEnviron())
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if context.Repository != "acme/checkout" || context.SHA != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Fatalf("Repository/SHA = %q / %q", context.Repository, context.SHA)
	}
	if context.WorkflowRef != ".github/workflows/ci.yml@refs/pull/184/merge" || context.WorkflowSHA == "" {
		t.Fatalf("Workflow = %q / %q", context.WorkflowRef, context.WorkflowSHA)
	}
	if context.RunID != "1234567890" || context.RunAttempt != "1" || context.EventName != "pull_request" {
		t.Fatalf("Run = %q / %q / %q", context.RunID, context.RunAttempt, context.EventName)
	}

	canonical, err := context.CanonicalRepository()
	if err != nil {
		t.Fatalf("CanonicalRepository() error = %v", err)
	}
	if canonical != "github.com/acme/checkout" {
		t.Fatalf("CanonicalRepository() = %q", canonical)
	}
}

func TestReadRejectsMissingRequiredVariables(t *testing.T) {
	t.Parallel()

	required := []string{"GITHUB_REPOSITORY", "GITHUB_SHA", "GITHUB_WORKFLOW_REF", "GITHUB_WORKFLOW_SHA", "GITHUB_RUN_ID"}

	for _, name := range required {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var environ []string
			for _, entry := range completeEnviron() {
				if strings.HasPrefix(entry, name+"=") {
					continue
				}
				environ = append(environ, entry)
			}
			if _, err := Read(environ); err == nil {
				t.Fatalf("Read() accepted a missing %s", name)
			}
		})
	}

	t.Run("empty required value", func(t *testing.T) {
		t.Parallel()

		environ := append(completeEnviron(), "GITHUB_SHA=")
		if _, err := Read(environ); err == nil {
			t.Fatalf("Read() accepted an empty GITHUB_SHA")
		}
	})
}

func TestReadRejectsMalformedValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		environ []string
	}{
		{name: "short sha", environ: []string{"GITHUB_REPOSITORY=acme/checkout", "GITHUB_SHA=abc", "GITHUB_WORKFLOW_REF=w@r", "GITHUB_WORKFLOW_SHA=2a5165d0f3f9", "GITHUB_RUN_ID=1"}},
		{name: "uppercase sha", environ: []string{"GITHUB_REPOSITORY=acme/checkout", "GITHUB_SHA=" + strings.ToUpper(strings.Repeat("b", 40)), "GITHUB_WORKFLOW_REF=w@r", "GITHUB_WORKFLOW_SHA=2a5165d0f3f9", "GITHUB_RUN_ID=1"}},
		{name: "repository without owner", environ: []string{"GITHUB_REPOSITORY=checkout", "GITHUB_SHA=" + strings.Repeat("b", 40), "GITHUB_WORKFLOW_REF=w@r", "GITHUB_WORKFLOW_SHA=2a5165d0f3f9", "GITHUB_RUN_ID=1"}},
		{name: "repository with slash", environ: []string{"GITHUB_REPOSITORY=acme//checkout", "GITHUB_SHA=" + strings.Repeat("b", 40), "GITHUB_WORKFLOW_REF=w@r", "GITHUB_WORKFLOW_SHA=2a5165d0f3f9", "GITHUB_RUN_ID=1"}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := Read(tt.environ); err == nil {
				t.Fatalf("Read() accepted malformed values")
			}
		})
	}
}

func TestReadOptionalVariablesStayEmpty(t *testing.T) {
	t.Parallel()

	minimal := []string{
		"GITHUB_REPOSITORY=acme/checkout",
		"GITHUB_SHA=" + strings.Repeat("b", 40),
		"GITHUB_WORKFLOW_REF=.github/workflows/ci.yml@refs/heads/main",
		"GITHUB_WORKFLOW_SHA=2a5165d0f3f9bbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"GITHUB_RUN_ID=42",
		"UNRELATED=value",
	}

	context, err := Read(minimal)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if context.Ref != "" || context.RunAttempt != "" || context.EventName != "" {
		t.Fatalf("optional fields = %q / %q / %q, want empty", context.Ref, context.RunAttempt, context.EventName)
	}
}
