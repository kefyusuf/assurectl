package cicontext

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kefyusuf/assurectl/internal/gitsubject"
)

// Context is the closed view of the GitHub Actions variables this slice
// consumes. It records what the runner asserts; it authenticates nothing
// (see the M2 ci-context design and the threat model's deferred CI/OIDC
// protections).
type Context struct {
	Repository  string // GITHUB_REPOSITORY, "owner/name"
	SHA         string // GITHUB_SHA
	Ref         string // GITHUB_REF (optional)
	WorkflowRef string // GITHUB_WORKFLOW_REF
	WorkflowSHA string // GITHUB_WORKFLOW_SHA
	RunID       string // GITHUB_RUN_ID
	RunAttempt  string // GITHUB_RUN_ATTEMPT (optional)
	EventName   string // GITHUB_EVENT_NAME (optional)
}

var (
	objectIDPattern   = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)
	repositoryPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*$`)
)

// Read builds a context from a caller-supplied environment. The core
// evaluator never reads the environment; the CLI adapter passes its own
// environ here. Unknown variables are ignored and never echoed.
func Read(environ []string) (Context, error) {
	values := make(map[string]string, len(environ))
	for _, entry := range environ {
		if key, value, ok := strings.Cut(entry, "="); ok {
			values[key] = value
		}
	}

	var context Context
	required := map[string]*string{
		"GITHUB_REPOSITORY":   &context.Repository,
		"GITHUB_SHA":          &context.SHA,
		"GITHUB_WORKFLOW_REF": &context.WorkflowRef,
		"GITHUB_WORKFLOW_SHA": &context.WorkflowSHA,
		"GITHUB_RUN_ID":       &context.RunID,
	}
	for name, target := range required {
		value := values[name]
		if strings.TrimSpace(value) == "" {
			return Context{}, fmt.Errorf("required variable %s is missing or empty", name)
		}
		*target = value
	}
	context.Ref = values["GITHUB_REF"]
	context.RunAttempt = values["GITHUB_RUN_ATTEMPT"]
	context.EventName = values["GITHUB_EVENT_NAME"]

	if !objectIDPattern.MatchString(context.SHA) {
		return Context{}, fmt.Errorf("GITHUB_SHA is not a full lowercase object id")
	}
	if !objectIDPattern.MatchString(context.WorkflowSHA) {
		return Context{}, fmt.Errorf("GITHUB_WORKFLOW_SHA is not a full lowercase object id")
	}
	if !repositoryPattern.MatchString(context.Repository) {
		return Context{}, fmt.Errorf("GITHUB_REPOSITORY is not an owner/name pair")
	}
	return context, nil
}

// CanonicalRepository maps the asserted owner/name pair onto the canonical
// hosted identity the subject resolver produces.
func (c Context) CanonicalRepository() (string, error) {
	canonical, err := gitsubject.CanonicalizeRepositoryIdentity("github.com/" + c.Repository)
	if err != nil {
		return "", fmt.Errorf("canonicalize CI repository: %w", err)
	}
	return canonical, nil
}
