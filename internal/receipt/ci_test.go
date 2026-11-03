package receipt

import (
	"strings"
	"testing"

	"github.com/kefyusuf/assurectl/internal/cicontext"
	"github.com/kefyusuf/assurectl/internal/domain"
)

func TestBuildAcceptsCIAssertedWithContext(t *testing.T) {
	t.Parallel()

	input := buildInput()
	input.Authority = domain.AuthorityCIAsserted
	input.CI = &cicontext.Context{
		Repository:  "acme/checkout",
		SHA:         "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		WorkflowRef: ".github/workflows/ci.yml@refs/pull/184/merge",
		WorkflowSHA: "2a5165d0f3f9bbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		RunID:       "1234567890",
	}
	if built, err := Build(input); err != nil {
		t.Fatalf("Build() error = %v", err)
	} else if built.Authority != domain.AuthorityCIAsserted {
		t.Fatalf("Authority = %q, want CI_ASSERTED", built.Authority)
	}
}

func TestBuildRejectsCIAssertedWithoutContext(t *testing.T) {
	t.Parallel()

	input := buildInput()
	input.Authority = domain.AuthorityCIAsserted
	if _, err := Build(input); err == nil {
		t.Fatalf("Build() accepted CI_ASSERTED without a context")
	}
}

func TestBuildRejectsAdvisoryWithContext(t *testing.T) {
	t.Parallel()

	input := buildInput()
	input.CI = &cicontext.Context{Repository: "acme/checkout", SHA: strings.Repeat("b", 40)}
	if _, err := Build(input); err == nil {
		t.Fatalf("Build() accepted ADVISORY with a CI context")
	}
}
