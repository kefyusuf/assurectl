package gitsubject

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadBlobReturnsBaseDocument(t *testing.T) {
	repo := newTestRepository(t)
	policyBody := `{"schema_version": "assurectl/policy/v0", "requirements": []}`
	commit := commitFile(t, repo, ".assurectl/policy.json", policyBody)

	data, err := ReadBlob(context.Background(), repo, commit, ".assurectl/policy.json")
	if err != nil {
		t.Fatalf("ReadBlob() error = %v", err)
	}
	if string(data) != policyBody {
		t.Fatalf("ReadBlob() = %q, want %q", data, policyBody)
	}
}

func TestReadBlobRejectsDisallowedPaths(t *testing.T) {
	repo := newTestRepository(t)
	commit := commitFile(t, repo, ".assurectl/policy.json", "{}")

	for _, path := range []string{
		".assurectl/other.json",
		"other/policy.json",
		"../outside.json",
		"/abs/policy.json",
		`.assurectl\policy.json`,
		".assurectl/policy.json ",
		"",
	} {
		if _, err := ReadBlob(context.Background(), repo, commit, path); err == nil {
			t.Fatalf("ReadBlob() accepted path %q", path)
		}
	}
}

func TestReadBlobRequiresResolvedRevision(t *testing.T) {
	repo := newTestRepository(t)
	commitFile(t, repo, ".assurectl/policy.json", "{}")

	for _, revision := range []string{"HEAD", "HEAD~1", "main", "abc", ""} {
		if _, err := ReadBlob(context.Background(), repo, revision, ".assurectl/policy.json"); err == nil {
			t.Fatalf("ReadBlob() accepted unresolved revision %q", revision)
		}
	}
}

func TestReadBlobErrorsOnMissingBlob(t *testing.T) {
	repo := newTestRepository(t)
	commit := commitFile(t, repo, ".assurectl/contract.json", "{}")

	// The revision exists but carries no policy document.
	if _, err := ReadBlob(context.Background(), repo, commit, ".assurectl/policy.json"); err == nil {
		t.Fatalf("ReadBlob() accepted a missing blob")
	} else if !strings.Contains(err.Error(), ".assurectl/policy.json") {
		t.Fatalf("error should name the path: %v", err)
	}
}

func TestReadBlobWorksFromNestedWorktree(t *testing.T) {
	repo := newTestRepository(t)
	policyBody := `{"schema_version": "assurectl/policy/v0", "requirements": []}`
	commit := commitFile(t, repo, ".assurectl/policy.json", policyBody)

	nested := filepath.Join(repo, "nested", "directory")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested worktree: %v", err)
	}
	data, err := ReadBlob(context.Background(), nested, commit, ".assurectl/policy.json")
	if err != nil {
		t.Fatalf("ReadBlob() error = %v", err)
	}
	if string(data) != policyBody {
		t.Fatalf("ReadBlob() = %q", data)
	}
}
