package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kefyusuf/assurectl/internal/contract"
	"github.com/kefyusuf/assurectl/internal/policy"
)

func TestInitScaffoldsValidDocuments(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if code := run([]string{"init", "--type", "dependency-upgrade", "--requirement-id", "dep-upgrade", "--evidence-type", "dependency-result", "--output-dir", dir}, &strings.Builder{}, &strings.Builder{}); code != 0 {
		t.Fatalf("init exit = %d", code)
	}

	contract, err := contract.LoadLocal(dir)
	if err != nil {
		t.Fatalf("LoadLocal(contract) error = %v", err)
	}
	if contract.Contract.WorkUnit.Type != "dependency_upgrade" {
		t.Fatalf("work unit type = %q", contract.Contract.WorkUnit.Type)
	}

	policyLoaded, err := policy.LoadLocal(dir)
	if err != nil {
		t.Fatalf("LoadLocal(policy) error = %v", err)
	}
	if len(policyLoaded.Policy.Requirements) != 1 {
		t.Fatalf("requirements = %d, want 1", len(policyLoaded.Policy.Requirements))
	}
	if policyLoaded.Policy.Requirements[0].ID != "dep-upgrade" {
		t.Fatalf("requirement id = %q", policyLoaded.Policy.Requirements[0].ID)
	}
}

func TestInitFailsOnExistingFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	assurectl := filepath.Join(dir, ".assurectl")
	if err := os.MkdirAll(assurectl, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assurectl, "contract.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if code := run([]string{"init", "--type", "dependency-upgrade", "--requirement-id", "dep-upgrade", "--evidence-type", "dependency-result", "--output-dir", dir}, &strings.Builder{}, &strings.Builder{}); code != 2 {
		t.Fatalf("init exit = %d, want 2 for existing files", code)
	}
}

func TestInitRejectsUnknownType(t *testing.T) {
	t.Parallel()

	if code := run([]string{"init", "--type", "nonexistent", "--requirement-id", "x", "--evidence-type", "y", "--output-dir", t.TempDir()}, &strings.Builder{}, &strings.Builder{}); code != 64 {
		t.Fatalf("init exit = %d, want 64 for unknown type", code)
	}
}
