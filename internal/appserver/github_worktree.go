package appserver

import (
	"context"
	"fmt"

	"github.com/kefyusuf/assurectl/internal/gitsubject"
)

// worktreeAdapter implements WorktreeSource by delegating to the vetted Git
// path in internal/gitsubject (ADR-0010): the remote comes from App
// configuration, the revision must already be a resolved object id, and all
// execution stays inside the package that owns the trusted Git machinery.
type worktreeAdapter struct {
	remote string
	cache  string
}

func (a worktreeAdapter) Worktree(ctx context.Context, installation int64, repository, revision string) (string, error) {
	if a.remote == "" || a.cache == "" {
		return "", fmt.Errorf("app: worktree: remote and cache directory are required")
	}
	worktree, err := gitsubject.FetchWorktree(ctx, a.remote, revision, a.cache)
	if err != nil {
		return "", fmt.Errorf("app: worktree: %w", err)
	}
	return worktree, nil
}
