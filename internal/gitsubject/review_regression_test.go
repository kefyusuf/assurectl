package gitsubject

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCanonicalizeRepositoryURIRejectsSurroundingWhitespace(t *testing.T) {
	for _, raw := range []string{
		" git@github.com:acme/checkout.git",
		"git@github.com:acme/checkout.git ",
		" ssh://git@github.com/acme/checkout.git",
	} {
		if _, err := canonicalizeRepositoryURI(raw); err == nil {
			t.Fatalf("canonicalizeRepositoryURI accepted surrounding whitespace in %q", raw)
		}
	}
}

func TestCanonicalizeRepositoryURIRejectsIdentitySignificantSSHUsers(t *testing.T) {
	for _, raw := range []string{
		"ssh://alice@example.com/acme/checkout.git",
		"bob@example.com:acme/checkout.git",
	} {
		if _, err := canonicalizeRepositoryURI(raw); err == nil {
			t.Fatalf("canonicalizeRepositoryURI accepted identity-significant SSH user in %q", raw)
		}
	}
}

func TestResolveDoesNotLeakCredentialedOrigin(t *testing.T) {
	repo := newTestRepository(t)
	commit := commitFile(t, repo, "tracked.txt", "tracked\n")
	rawOrigin := "https://alice:super-secret@example.com/acme/checkout.git"
	runTestGit(t, repo, "remote", "add", "origin", rawOrigin)

	_, err := Resolve(context.Background(), repo, Options{BaseRef: commit, HeadRef: commit})
	if err == nil {
		t.Fatal("Resolve accepted credentialed origin")
	}
	message := err.Error()
	for _, secret := range []string{"alice", "super-secret", rawOrigin} {
		if strings.Contains(message, secret) {
			t.Fatalf("Resolve error leaked origin credential data %q: %v", secret, err)
		}
	}
}

func TestSanitizedGitEnvironmentPinsSafetyControls(t *testing.T) {
	got := environmentMap(sanitizedGitEnvironment([]string{
		"PATH=/usr/bin",
		"GIT_CONFIG_GLOBAL=/tmp/attacker-global-config",
		"GIT_CONFIG_NOSYSTEM=0",
		"GIT_TERMINAL_PROMPT=1",
		"GIT_NO_LAZY_FETCH=0",
		"GIT_NO_REPLACE_OBJECTS=0",
		"GIT_OPTIONAL_LOCKS=1",
	}))

	want := map[string]string{
		"GIT_CONFIG_GLOBAL":      os.DevNull,
		"GIT_CONFIG_NOSYSTEM":    "1",
		"GIT_TERMINAL_PROMPT":    "0",
		"GIT_NO_LAZY_FETCH":      "1",
		"GIT_NO_REPLACE_OBJECTS": "1",
		"GIT_OPTIONAL_LOCKS":     "0",
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("%s = %q, want %q", key, got[key], value)
		}
	}
}

func TestSanitizedGitEnvironmentRemovesUnsafeOverrides(t *testing.T) {
	got := environmentMap(sanitizedGitEnvironment([]string{
		"PATH=/usr/bin",
		"GIT_TRACE_SETUP=/tmp/trace-setup",
		"GIT_TRACE2_EVENT=/tmp/trace2-event",
		"GIT_EXEC_PATH=/tmp/git-exec",
		"GIT_NAMESPACE=attacker",
		"GIT_SSH_COMMAND=/tmp/ssh-wrapper",
		"GIT_ASKPASS=/tmp/askpass",
	}))

	for _, key := range []string{
		"GIT_TRACE_SETUP",
		"GIT_TRACE2_EVENT",
		"GIT_EXEC_PATH",
		"GIT_NAMESPACE",
		"GIT_SSH_COMMAND",
		"GIT_ASKPASS",
	} {
		if _, ok := got[key]; ok {
			t.Fatalf("unsafe Git environment override %s was preserved", key)
		}
	}
}

func TestResolveDoesNotWriteInheritedGitTrace(t *testing.T) {
	repo := newTestRepository(t)
	commit := commitFile(t, repo, "tracked.txt", "tracked\n")
	tracePath := filepath.Join(t.TempDir(), "git-trace.log")
	t.Setenv("GIT_TRACE_SETUP", tracePath)

	if _, err := Resolve(context.Background(), repo, Options{
		RepositoryURI: "https://github.com/acme/checkout.git",
		BaseRef:       commit,
		HeadRef:       commit,
	}); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if _, err := os.Stat(tracePath); err == nil {
		t.Fatalf("Resolve allowed Git trace output at %q", tracePath)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat Git trace output: %v", err)
	}
}

func TestResolveRejectsReplacementRefMasqueradingAsCommit(t *testing.T) {
	repo := newTestRepository(t)
	commit := commitFile(t, repo, "tracked.txt", "tracked\n")
	payload := filepath.Join(repo, "payload.bin")
	if err := os.WriteFile(payload, []byte("blob payload\n"), 0o600); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	blob := strings.TrimSpace(runTestGit(t, repo, "hash-object", "-w", "--", payload))
	runTestGit(t, repo, "update-ref", "refs/replace/"+blob, commit)

	if _, err := Resolve(context.Background(), repo, Options{
		RepositoryURI: "https://github.com/acme/checkout.git",
		BaseRef:       blob,
		HeadRef:       commit,
	}); err == nil {
		t.Fatal("Resolve accepted a blob object through a replacement ref")
	}
}

func TestResolveMarksAssumeUnchangedTrackedEditDirty(t *testing.T) {
	repo := newTestRepository(t)
	commit := commitFile(t, repo, "tracked.txt", "tracked\n")
	runTestGit(t, repo, "update-index", "--assume-unchanged", "--", "tracked.txt")
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
		t.Fatal("Resolve treated an assume-unchanged tracked edit as clean")
	}
}

func TestResolvePreservesSignificantTrailingSpaceInWorktreeRoot(t *testing.T) {
	parent := t.TempDir()
	sibling := filepath.Join(parent, "repo")
	spaced := filepath.Join(parent, "repo ")
	initRepositoryAt(t, sibling)
	initRepositoryAt(t, spaced)
	_ = commitFile(t, sibling, "sibling.txt", "sibling\n")
	spacedCommit := commitFile(t, spaced, "spaced.txt", "spaced\n")

	got, err := Resolve(context.Background(), spaced, Options{
		RepositoryURI: "https://github.com/acme/checkout.git",
		BaseRef:       "HEAD",
		HeadRef:       "HEAD",
	})
	if err != nil {
		t.Fatalf("Resolve spaced worktree: %v", err)
	}
	if got.Subject.HeadRevision != spacedCommit {
		t.Fatalf("head revision = %q, want spaced-repository commit %q", got.Subject.HeadRevision, spacedCommit)
	}
}

func TestResolveDoesNotExecuteFSMonitorHook(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable shell hook regression is POSIX-specific")
	}

	repo := newTestRepository(t)
	commit := commitFile(t, repo, "tracked.txt", "tracked\n")
	sentinel := filepath.Join(t.TempDir(), "fsmonitor-called")
	hook := filepath.Join(t.TempDir(), "fsmonitor-hook")
	script := "#!/bin/sh\nprintf called > " + shellQuote(sentinel) + "\nexit 1\n"
	if err := os.WriteFile(hook, []byte(script), 0o700); err != nil {
		t.Fatalf("write fsmonitor hook: %v", err)
	}
	runTestGit(t, repo, "config", "--local", "core.fsmonitor", hook)

	if _, err := Resolve(context.Background(), repo, Options{
		RepositoryURI: "https://github.com/acme/checkout.git",
		BaseRef:       commit,
		HeadRef:       commit,
	}); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if _, err := os.Stat(sentinel); err == nil {
		t.Fatal("Resolve executed repository-controlled fsmonitor hook")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat fsmonitor sentinel: %v", err)
	}
}

func environmentMap(environment []string) map[string]string {
	result := make(map[string]string, len(environment))
	for _, entry := range environment {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			result[key] = value
		}
	}
	return result
}

func initRepositoryAt(t *testing.T, repo string) {
	t.Helper()
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatalf("mkdir repository %q: %v", repo, err)
	}
	runTestGit(t, repo, "init")
	runTestGit(t, repo, "config", "user.name", "AssureCTL Test")
	runTestGit(t, repo, "config", "user.email", "assurectl-test@example.invalid")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
