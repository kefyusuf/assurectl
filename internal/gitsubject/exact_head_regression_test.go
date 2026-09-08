package gitsubject

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCanonicalizeRepositoryURIRejectsImplicitSSHUser(t *testing.T) {
	if _, err := canonicalizeRepositoryURI("ssh://example.com/acme/checkout.git"); err == nil {
		t.Fatal("canonicalizeRepositoryURI accepted SSH URL without explicit git username")
	}
}

func TestResolveMarksSkipWorktreeTrackedEditDirty(t *testing.T) {
	repo := newTestRepository(t)
	commit := commitFile(t, repo, "tracked.txt", "tracked\n")
	runTestGit(t, repo, "update-index", "--skip-worktree", "--", "tracked.txt")
	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("changed\n"), 0o600); err != nil {
		t.Fatalf("modify tracked file: %v", err)
	}

	got, err := Resolve(context.Background(), repo, Options{
		RepositoryURI: "https://github.com/acme/checkout.git",
		BaseRef:       commit,
		HeadRef:       commit,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !got.Dirty {
		t.Fatal("Resolve treated a skip-worktree tracked edit as clean")
	}
}
