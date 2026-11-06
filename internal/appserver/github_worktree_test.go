package appserver

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGitHelper(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func TestWorktreeAdapterFetchesRevision(t *testing.T) {
	t.Parallel()

	origin := t.TempDir()
	runGitHelper(t, origin, "init")
	runGitHelper(t, origin, "config", "user.name", "T")
	runGitHelper(t, origin, "config", "user.email", "t@example.invalid")
	if err := os.WriteFile(filepath.Join(origin, "tracked.txt"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitHelper(t, origin, "add", "tracked.txt")
	runGitHelper(t, origin, "commit", "-m", "base", "-q")
	revision := runGitHelper(t, origin, "rev-parse", "HEAD")

	adapter := worktreeAdapter{remote: origin, cache: t.TempDir()}
	worktree, err := adapter.Worktree(context.Background(), 7, "acme/checkout", revision)
	if err != nil {
		t.Fatalf("Worktree() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(worktree, "tracked.txt"))
	if err != nil {
		t.Fatalf("read worktree file: %v", err)
	}
	if string(data) != "content\n" {
		t.Fatalf("worktree content = %q", data)
	}
}

func TestWorktreeAdapterRejectsUnresolvedRevision(t *testing.T) {
	t.Parallel()

	origin := t.TempDir()
	runGitHelper(t, origin, "init")
	runGitHelper(t, origin, "commit", "--allow-empty", "-m", "base", "-q")

	adapter := worktreeAdapter{remote: origin, cache: t.TempDir()}
	if _, err := adapter.Worktree(context.Background(), 7, "acme/checkout", "HEAD"); err == nil {
		t.Fatalf("Worktree() accepted an unresolved revision")
	}
	if _, err := adapter.Worktree(context.Background(), 7, "acme/checkout", ""); err == nil {
		t.Fatalf("Worktree() accepted an empty revision")
	}
}
