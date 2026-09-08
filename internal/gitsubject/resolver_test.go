package gitsubject

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveExplicitRepositoryURIAndExactRevisions(t *testing.T) {
	repo := newTestRepository(t)
	base := commitFile(t, repo, "base.txt", "base\n")
	head := commitFile(t, repo, "head.txt", "head\n")
	runTestGit(t, repo, "remote", "add", "origin", "https://github.com/other/project.git")

	got, err := Resolve(context.Background(), repo, Options{
		RepositoryURI: "git@github.com:acme/checkout.git",
		BaseRef:       base,
		HeadRef:       "HEAD",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if got.Subject.RepositoryURI != "github.com/acme/checkout" {
		t.Fatalf("repository URI = %q", got.Subject.RepositoryURI)
	}
	if got.Subject.BaseRevision != base {
		t.Fatalf("base revision = %q, want %q", got.Subject.BaseRevision, base)
	}
	if got.Subject.HeadRevision != head {
		t.Fatalf("head revision = %q, want %q", got.Subject.HeadRevision, head)
	}
	if got.Subject.ChangeSetAlgo != changeSetAlgorithm {
		t.Fatalf("change-set algorithm = %q, want %q", got.Subject.ChangeSetAlgo, changeSetAlgorithm)
	}

	wantDigest, err := changeSetDigest(got.Subject.RepositoryURI, base, head)
	if err != nil {
		t.Fatalf("changeSetDigest: %v", err)
	}
	if got.Subject.ChangeSetHash != wantDigest {
		t.Fatalf("change-set digest = %q, want %q", got.Subject.ChangeSetHash, wantDigest)
	}
	if got.Dirty {
		t.Fatal("clean repository resolved as dirty")
	}
	if got.LocalOnly {
		t.Fatal("explicit hosted repository resolved as local-only")
	}
}

func TestResolveUsesOriginWhenExplicitRepositoryURIOmitted(t *testing.T) {
	repo := newTestRepository(t)
	commit := commitFile(t, repo, "tracked.txt", "tracked\n")
	runTestGit(t, repo, "remote", "add", "origin", "ssh://git@GITHUB.COM/acme/checkout.git")

	got, err := Resolve(context.Background(), repo, Options{BaseRef: commit, HeadRef: commit})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Subject.RepositoryURI != "github.com/acme/checkout" {
		t.Fatalf("repository URI = %q", got.Subject.RepositoryURI)
	}
	if got.LocalOnly {
		t.Fatal("origin-backed repository resolved as local-only")
	}
}

func TestResolveAcceptsEquivalentMultipleOriginURLs(t *testing.T) {
	repo := newTestRepository(t)
	commit := commitFile(t, repo, "tracked.txt", "tracked\n")
	runTestGit(t, repo, "config", "--local", "--add", "remote.origin.url", "https://github.com/acme/checkout.git")
	runTestGit(t, repo, "config", "--local", "--add", "remote.origin.url", "git@github.com:acme/checkout.git")

	got, err := Resolve(context.Background(), repo, Options{BaseRef: commit, HeadRef: commit})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Subject.RepositoryURI != "github.com/acme/checkout" {
		t.Fatalf("repository URI = %q", got.Subject.RepositoryURI)
	}
}

func TestResolveRejectsAmbiguousOriginURLs(t *testing.T) {
	repo := newTestRepository(t)
	commit := commitFile(t, repo, "tracked.txt", "tracked\n")
	runTestGit(t, repo, "config", "--local", "--add", "remote.origin.url", "https://github.com/acme/checkout.git")
	runTestGit(t, repo, "config", "--local", "--add", "remote.origin.url", "https://github.com/acme/other.git")

	if _, err := Resolve(context.Background(), repo, Options{BaseRef: commit, HeadRef: commit}); err == nil {
		t.Fatal("Resolve accepted ambiguous origin identities")
	}
}

func TestResolveLocalFallbackIsStableAndDoesNotLeakWorktreePath(t *testing.T) {
	repo := newTestRepository(t)
	commit := commitFile(t, repo, "tracked.txt", "tracked\n")
	nested := filepath.Join(repo, "nested", "directory")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested worktree path: %v", err)
	}

	first, err := Resolve(context.Background(), repo, Options{BaseRef: commit, HeadRef: commit})
	if err != nil {
		t.Fatalf("Resolve root: %v", err)
	}
	second, err := Resolve(context.Background(), nested, Options{BaseRef: commit, HeadRef: commit})
	if err != nil {
		t.Fatalf("Resolve nested: %v", err)
	}

	if !first.LocalOnly || !second.LocalOnly {
		t.Fatal("no-origin repository did not resolve as local-only")
	}
	if first.Subject.RepositoryURI != second.Subject.RepositoryURI {
		t.Fatalf("local identity differs by starting directory: %q != %q", first.Subject.RepositoryURI, second.Subject.RepositoryURI)
	}
	if !strings.HasPrefix(first.Subject.RepositoryURI, "local://sha256/") {
		t.Fatalf("local repository URI = %q", first.Subject.RepositoryURI)
	}
	if strings.Contains(first.Subject.RepositoryURI, repo) || strings.Contains(first.Subject.RepositoryURI, filepath.Base(repo)) {
		t.Fatalf("local repository URI leaks worktree path: %q", first.Subject.RepositoryURI)
	}
}

func TestResolveReportsDirtyWorktree(t *testing.T) {
	repo := newTestRepository(t)
	commit := commitFile(t, repo, "tracked.txt", "tracked\n")

	clean, err := Resolve(context.Background(), repo, Options{
		RepositoryURI: "https://github.com/acme/checkout.git",
		BaseRef:       commit,
		HeadRef:       commit,
	})
	if err != nil {
		t.Fatalf("Resolve clean: %v", err)
	}
	if clean.Dirty {
		t.Fatal("clean worktree resolved as dirty")
	}

	if err := os.WriteFile(filepath.Join(repo, "untracked.txt"), []byte("untracked\n"), 0o600); err != nil {
		t.Fatalf("write untracked file: %v", err)
	}
	dirty, err := Resolve(context.Background(), repo, Options{
		RepositoryURI: "https://github.com/acme/checkout.git",
		BaseRef:       commit,
		HeadRef:       commit,
	})
	if err != nil {
		t.Fatalf("Resolve dirty: %v", err)
	}
	if !dirty.Dirty {
		t.Fatal("dirty worktree resolved as clean")
	}
}

func TestResolveRejectsNonRepositoryAndBareRepository(t *testing.T) {
	notRepo := t.TempDir()
	if _, err := Resolve(context.Background(), notRepo, Options{BaseRef: "HEAD", HeadRef: "HEAD"}); err == nil {
		t.Fatal("Resolve accepted non-repository directory")
	}

	bare := filepath.Join(t.TempDir(), "bare.git")
	cmd := exec.CommandContext(t.Context(), "git", "init", "--bare", bare)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, output)
	}
	if _, err := Resolve(context.Background(), bare, Options{BaseRef: "HEAD", HeadRef: "HEAD"}); err == nil {
		t.Fatal("Resolve accepted bare repository")
	}
}

func TestResolveRejectsMissingAndOptionLikeRefs(t *testing.T) {
	repo := newTestRepository(t)
	commit := commitFile(t, repo, "tracked.txt", "tracked\n")
	uri := "https://github.com/acme/checkout.git"

	for _, baseRef := range []string{"", "missing-ref", "--help"} {
		_, err := Resolve(context.Background(), repo, Options{RepositoryURI: uri, BaseRef: baseRef, HeadRef: commit})
		if err == nil {
			t.Fatalf("Resolve accepted base ref %q", baseRef)
		}
	}
	for _, headRef := range []string{"", "missing-ref", "--help"} {
		_, err := Resolve(context.Background(), repo, Options{RepositoryURI: uri, BaseRef: commit, HeadRef: headRef})
		if err == nil {
			t.Fatalf("Resolve accepted head ref %q", headRef)
		}
	}
}

func TestResolveIsRepeatable(t *testing.T) {
	repo := newTestRepository(t)
	base := commitFile(t, repo, "base.txt", "base\n")
	head := commitFile(t, repo, "head.txt", "head\n")
	opts := Options{RepositoryURI: "https://github.com/acme/checkout.git", BaseRef: base, HeadRef: head}

	first, err := Resolve(context.Background(), repo, opts)
	if err != nil {
		t.Fatalf("Resolve first: %v", err)
	}
	second, err := Resolve(context.Background(), repo, opts)
	if err != nil {
		t.Fatalf("Resolve second: %v", err)
	}
	if first != second {
		t.Fatalf("repeat resolution differs: %#v != %#v", first, second)
	}
}

func newTestRepository(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	runTestGit(t, repo, "init")
	runTestGit(t, repo, "config", "user.name", "AssureCTL Test")
	runTestGit(t, repo, "config", "user.email", "assurectl-test@example.invalid")
	return repo
}

func commitFile(t *testing.T, repo, name, contents string) string {
	t.Helper()

	path := filepath.Join(repo, name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	runTestGit(t, repo, "add", "--", name)
	runTestGit(t, repo, "commit", "-m", "test commit")
	return strings.TrimSpace(runTestGit(t, repo, "rev-parse", "HEAD"))
}

func runTestGit(t *testing.T, repo string, args ...string) string {
	t.Helper()

	commandArgs := append([]string{"-C", repo}, args...)
	cmd := exec.CommandContext(t.Context(), "git", commandArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(commandArgs, " "), err, output)
	}
	return string(output)
}
