package assurectl

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestReferenceActionShape pins the reference GitHub Action's structural
// guarantees: composite, pinned third-party steps, the documented inputs,
// the CI-asserted invocation, and no failure masking.
func TestReferenceActionShape(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("action.yml")
	if err != nil {
		t.Fatalf("read action.yml: %v", err)
	}
	document := string(data)

	if !strings.Contains(document, "using: \"composite\"") {
		t.Fatalf("action is not composite:\n%s", document)
	}

	pinned := regexp.MustCompile(`uses: "?actions/[a-z-]+@([0-9a-f]{40})"?`)
	matches := pinned.FindAllStringSubmatch(document, -1)
	if len(matches) < 2 {
		t.Fatalf("third-party steps are not pinned to immutable SHAs:\n%s", document)
	}

	for _, required := range []string{
		"base-ref:", "head-ref:", "repository:", "waivers-path:", "go-version:", "working-directory:",
	} {
		if !strings.Contains(document, required) {
			t.Fatalf("input %q missing from action.yml", required)
		}
	}

	if !strings.Contains(document, "verify --ci") {
		t.Fatalf("action does not run verify --ci:\n%s", document)
	}
	if strings.Contains(document, "continue-on-error: true") {
		t.Fatalf("action masks failures with continue-on-error:\n%s", document)
	}
}
