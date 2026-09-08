package gitsubject

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

func TestResolveRejectsCoreWorktreeRedirectToDifferentRepository(t *testing.T) {
	parent := t.TempDir()
	first := filepath.Join(parent, "first")
	second := filepath.Join(parent, "second")
	initRepositoryAt(t, first)
	initRepositoryAt(t, second)
	_ = commitFile(t, first, "first.txt", "first\n")
	_ = commitFile(t, second, "second.txt", "second\n")
	runTestGit(t, first, "config", "--local", "core.worktree", second)

	if _, err := Resolve(context.Background(), first, Options{
		RepositoryURI: "https://github.com/acme/checkout.git",
		BaseRef:       "HEAD",
		HeadRef:       "HEAD",
	}); err == nil {
		t.Fatal("Resolve followed core.worktree into a different repository")
	}
}

func TestResolveTreatsGitlinkAsAdvisoryDirty(t *testing.T) {
	source := filepath.Join(t.TempDir(), "submodule-source")
	initRepositoryAt(t, source)
	_ = commitFile(t, source, "submodule.txt", "submodule\n")

	repo := newTestRepository(t)
	runTestGit(t, repo, "-c", "protocol.file.allow=always", "submodule", "add", "--", source, "submodule")
	runTestGit(t, repo, "commit", "-m", "add submodule")
	commit := strings.TrimSpace(runTestGit(t, repo, "rev-parse", "HEAD"))

	got, err := Resolve(context.Background(), repo, Options{
		RepositoryURI: "https://github.com/acme/checkout.git",
		BaseRef:       commit,
		HeadRef:       commit,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !got.Dirty {
		t.Fatal("Resolve treated a repository containing a gitlink as clean")
	}
}
