package gitsubject

import (
	"context"
	"fmt"
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
