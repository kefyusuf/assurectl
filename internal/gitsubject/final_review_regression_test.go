package gitsubject

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestResolveRejectsCallerOutsideDiscoveredWorktree(t *testing.T) {
	parent := t.TempDir()
	caller := filepath.Join(parent, "caller")
	redirected := filepath.Join(parent, "redirected")
	initRepositoryAt(t, caller)
	commit := commitFile(t, caller, "tracked.txt", "tracked\n")

	if err := os.Mkdir(redirected, 0o755); err != nil {
		t.Fatalf("mkdir redirected worktree: %v", err)
	}
	gitDir := filepath.Join(caller, ".git")
	if err := os.WriteFile(filepath.Join(redirected, ".git"), []byte("gitdir: "+gitDir+"\n"), 0o600); err != nil {
		t.Fatalf("write redirected .git file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(redirected, "tracked.txt"), []byte("tracked\n"), 0o600); err != nil {
		t.Fatalf("write redirected tracked file: %v", err)
	}
	runTestGit(t, caller, "config", "--local", "core.worktree", redirected)

	if _, err := Resolve(context.Background(), caller, Options{
		RepositoryURI: "https://github.com/acme/checkout.git",
		BaseRef:       commit,
		HeadRef:       commit,
	}); err == nil {
		t.Fatal("Resolve accepted a discovered worktree that does not contain the caller path")
	}
}

func TestResolveMarksCheckoutHeadMismatchDirty(t *testing.T) {
	repo := newTestRepository(t)
	base := commitFile(t, repo, "tracked.txt", "base\n")
	head := commitFile(t, repo, "tracked.txt", "head\n")
	runTestGit(t, repo, "checkout", "--detach", base)

	got, err := Resolve(context.Background(), repo, Options{
		RepositoryURI: "https://github.com/acme/checkout.git",
		BaseRef:       base,
		HeadRef:       head,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !got.Dirty {
		t.Fatal("Resolve treated a checkout at a different HEAD as clean")
	}
}

func TestResolveRejectsAmbiguousShortRef(t *testing.T) {
	repo := newTestRepository(t)
	base := commitFile(t, repo, "tracked.txt", "base\n")
	head := commitFile(t, repo, "tracked.txt", "head\n")
	runTestGit(t, repo, "branch", "collision", head)
	runTestGit(t, repo, "tag", "collision", base)

	if _, err := Resolve(context.Background(), repo, Options{
		RepositoryURI: "https://github.com/acme/checkout.git",
		BaseRef:       "collision",
		HeadRef:       "HEAD",
	}); err == nil {
		t.Fatal("Resolve accepted an ambiguous short ref shared by a branch and tag")
	}
}

func TestResolveDoesNotUseGitFromUntrustedPATH(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH executable-substitution regression is POSIX-specific")
	}

	repo := newTestRepository(t)
	commit := commitFile(t, repo, "tracked.txt", "tracked\n")
	sentinel := filepath.Join(t.TempDir(), "fake-git-called")
	fakeDir := t.TempDir()
	fakeGit := filepath.Join(fakeDir, "git")
	script := "#!/bin/sh\nprintf called > " + shellQuote(sentinel) + "\nexit 42\n"
	if err := os.WriteFile(fakeGit, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	t.Setenv("PATH", fakeDir)

	got, err := Resolve(context.Background(), repo, Options{
		RepositoryURI: "https://github.com/acme/checkout.git",
		BaseRef:       commit,
		HeadRef:       commit,
	})
	if err != nil {
		t.Fatalf("Resolve used untrusted PATH instead of a pinned Git executable: %v", err)
	}
	if got.Subject.HeadRevision != commit {
		t.Fatalf("head revision = %q, want %q", got.Subject.HeadRevision, commit)
	}
	if _, err := os.Stat(sentinel); err == nil {
		t.Fatal("Resolve executed a PATH-controlled fake Git binary")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat fake-git sentinel: %v", err)
	}
}

func TestCanonicalizeRepositoryURIPreservesGenericSSHPathSemantics(t *testing.T) {
	scp, err := canonicalizeRepositoryURI("git@example.com:owner/repo.git")
	if err != nil {
		t.Fatalf("canonicalize SCP-like URI: %v", err)
	}
	sshURL, err := canonicalizeRepositoryURI("ssh://git@example.com/owner/repo.git")
	if err != nil {
		t.Fatalf("canonicalize SSH URL: %v", err)
	}
	if scp == sshURL {
		t.Fatalf("relative SCP path and absolute SSH URL collapsed to the same identity %q", scp)
	}
}

func TestCanonicalizeRepositoryURIRedactsMalformedCredentialParseError(t *testing.T) {
	raw := "https://alice:secret@[::1/repo"
	_, err := canonicalizeRepositoryURI(raw)
	if err == nil {
		t.Fatal("canonicalizeRepositoryURI accepted malformed credential-bearing URI")
	}
	for _, secret := range []string{"alice", "secret", raw} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("parse error leaked credential data %q: %v", secret, err)
		}
	}
}

func TestResolveMarksSymlinkTypeChangeDirtyWhenCoreSymlinksDisabled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink-type regression requires a symlink-capable POSIX checkout")
	}

	repo := newTestRepository(t)
	target := filepath.Join(repo, "target.txt")
	if err := os.WriteFile(target, []byte("target\n"), 0o600); err != nil {
		t.Fatalf("write symlink target: %v", err)
	}
	link := filepath.Join(repo, "link.txt")
	if err := os.Symlink("target.txt", link); err != nil {
		t.Fatalf("create tracked symlink: %v", err)
	}
	runTestGit(t, repo, "add", "target.txt", "link.txt")
	runTestGit(t, repo, "commit", "-m", "add symlink")
	commit := strings.TrimSpace(runTestGit(t, repo, "rev-parse", "HEAD"))
	runTestGit(t, repo, "config", "--local", "core.symlinks", "false")

	if err := os.Remove(link); err != nil {
		t.Fatalf("remove tracked symlink: %v", err)
	}
	if err := os.WriteFile(link, []byte("target.txt"), 0o600); err != nil {
		t.Fatalf("replace symlink with regular file: %v", err)
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
		t.Fatal("Resolve treated a symlink-to-regular-file replacement as clean")
	}
}

func TestResolveRejectsStatCacheWeakeningFalseClean(t *testing.T) {
	repo := newTestRepository(t)
	commit := commitFile(t, repo, "tracked.txt", "AAAA\n")
	tracked := filepath.Join(repo, "tracked.txt")
	originalInfo, err := os.Stat(tracked)
	if err != nil {
		t.Fatalf("stat tracked file: %v", err)
	}

	runTestGit(t, repo, "config", "--local", "core.trustctime", "false")
	runTestGit(t, repo, "config", "--local", "core.checkStat", "minimal")
	time.Sleep(1100 * time.Millisecond)
	if err := os.WriteFile(tracked, []byte("BBBB\n"), 0o600); err != nil {
		t.Fatalf("overwrite tracked file: %v", err)
	}
	if err := os.Chtimes(tracked, originalInfo.ModTime(), originalInfo.ModTime()); err != nil {
		t.Fatalf("restore tracked-file mtime: %v", err)
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
		t.Fatal("Resolve treated stat-cache-weakened changed content as clean")
	}
}

func TestSanitizedGitEnvironmentRemovesMixedCaseUnsafeOverrides(t *testing.T) {
	got := sanitizedGitEnvironment([]string{
		"PATH=/usr/bin",
		"Git_Dir=/tmp/attacker.git",
		"git_work_tree=/tmp/attacker-worktree",
		"Git_Trace_Setup=/tmp/trace",
		"Git_Config_Key_0=core.fsmonitor",
		"Git_Config_Value_0=/tmp/hook",
	})

	for _, entry := range got {
		name, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		upper := strings.ToUpper(name)
		if upper == "GIT_DIR" || upper == "GIT_WORK_TREE" || strings.HasPrefix(upper, "GIT_TRACE") || strings.HasPrefix(upper, "GIT_CONFIG_KEY_") || strings.HasPrefix(upper, "GIT_CONFIG_VALUE_") {
			t.Fatalf("mixed-case unsafe Git override was preserved: %q", entry)
		}
	}
}
