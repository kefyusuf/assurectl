package gitsubject

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// allowlistedBasePaths are the only documents ReadBlob may fetch from a
// revision. The allowlist is checked before Git runs: trusted-base
// resolution needs exactly these two documents and nothing else.
var allowlistedBasePaths = map[string]struct{}{
	".assurectl/policy.json":   {},
	".assurectl/contract.json": {},
}

// ReadBlob returns the bytes of one allowlisted document at an
// already-resolved object id, using the same trusted executable, sanitized
// environment, and argument-list execution as the subject resolver
// (ADR-0009: no free-form paths, no revision expressions, no shell).
func ReadBlob(ctx context.Context, worktree, revision, path string) ([]byte, error) {
	if _, allowed := allowlistedBasePaths[path]; !allowed {
		return nil, fmt.Errorf("read base blob: path %q is not allowlisted", path)
	}
	if err := validateObjectID(revision); err != nil {
		return nil, fmt.Errorf("read base blob: %w", err)
	}
	data, err := runGit(ctx, worktree, "show", revision+":"+path)
	if err != nil {
		return nil, fmt.Errorf("read base blob %s at %s: %w", path, revision, err)
	}
	return data, nil
}

// FetchWorktree materializes a worktree at an already-resolved object id by
// fetching it from the configured remote into a managed cache directory —
// the server-side counterpart of ReadBlob for the reference App (ADR-0010).
// The remote comes from App configuration, never from webhook payloads; the
// revision must already be a resolved object id; execution reuses the same
// vetted Git path as the resolver.
func FetchWorktree(ctx context.Context, remote, revision, cacheDir string) (string, error) {
	if err := validateObjectID(revision); err != nil {
		return "", fmt.Errorf("fetch worktree: %w", err)
	}
	if strings.TrimSpace(remote) == "" || strings.TrimSpace(cacheDir) == "" {
		return "", fmt.Errorf("fetch worktree: remote and cache directory are required")
	}

	worktree := filepath.Join(cacheDir, revision)
	if _, statErr := os.Stat(filepath.Join(worktree, ".git")); statErr == nil {
		return worktree, nil
	}

	steps := [][]string{
		{"init", "-q", worktree},
		{"-C", worktree, "remote", "add", "origin", remote},
		{"-C", worktree, "fetch", "-q", "origin", revision},
		{"-C", worktree, "checkout", "-q", "--detach", "FETCH_HEAD"},
	}
	for _, step := range steps {
		if _, err := runGit(ctx, cacheDir, step...); err != nil {
			return "", fmt.Errorf("fetch worktree: git %s: %w", strings.Join(step, " "), err)
		}
	}
	return worktree, nil
}
