package gitsubject

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kefyusuf/assurectl/internal/domain"
)

const localRepositoryIdentityAlgorithm = "assurectl.local-repository/v0"

type Options struct {
	RepositoryURI string
	BaseRef       string
	HeadRef       string
}

type Resolution struct {
	Subject   domain.Subject
	Dirty     bool
	LocalOnly bool
}

func Resolve(ctx context.Context, worktree string, opts Options) (Resolution, error) {
	if ctx == nil {
		return Resolution{}, fmt.Errorf("context is nil")
	}

	root, err := resolveWorktreeRoot(ctx, worktree)
	if err != nil {
		return Resolution{}, err
	}

	baseRevision, err := resolveCommit(ctx, root, "base", opts.BaseRef)
	if err != nil {
		return Resolution{}, err
	}
	headRevision, err := resolveCommit(ctx, root, "head", opts.HeadRef)
	if err != nil {
		return Resolution{}, err
	}

	repositoryURI, localOnly, err := resolveRepositoryIdentity(ctx, root, opts.RepositoryURI)
	if err != nil {
		return Resolution{}, err
	}

	digest, err := changeSetDigest(repositoryURI, baseRevision, headRevision)
	if err != nil {
		return Resolution{}, fmt.Errorf("compute change-set digest: %w", err)
	}

	dirty, err := worktreeDirty(ctx, root)
	if err != nil {
		return Resolution{}, err
	}

	return Resolution{
		Subject: domain.Subject{
			RepositoryURI: repositoryURI,
			BaseRevision:  baseRevision,
			HeadRevision:  headRevision,
			ChangeSetAlgo: changeSetAlgorithm,
			ChangeSetHash: digest,
		},
		Dirty:     dirty,
		LocalOnly: localOnly,
	}, nil
}

func resolveWorktreeRoot(ctx context.Context, worktree string) (string, error) {
	if strings.TrimSpace(worktree) == "" {
		return "", fmt.Errorf("worktree path is empty")
	}

	absolute, err := filepath.Abs(worktree)
	if err != nil {
		return "", fmt.Errorf("resolve worktree path: %w", err)
	}
	absolute = filepath.Clean(absolute)

	rootOutput, err := runGit(ctx, absolute, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("resolve Git worktree root: %w", err)
	}
	root := strings.TrimSpace(string(rootOutput))
	if root == "" {
		return "", fmt.Errorf("Git worktree root is empty")
	}

	insideOutput, err := runGit(ctx, root, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return "", fmt.Errorf("verify Git worktree: %w", err)
	}
	if strings.TrimSpace(string(insideOutput)) != "true" {
		return "", fmt.Errorf("path is not inside a Git worktree")
	}

	root, err = filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("normalize Git worktree root: %w", err)
	}
	root = filepath.Clean(root)
	if resolved, evalErr := filepath.EvalSymlinks(root); evalErr == nil {
		root = filepath.Clean(resolved)
	} else {
		return "", fmt.Errorf("resolve Git worktree symlinks: %w", evalErr)
	}
	return root, nil
}

func resolveCommit(ctx context.Context, root, label, ref string) (string, error) {
	if ref == "" || strings.TrimSpace(ref) != ref || strings.ContainsAny(ref, "\x00\r\n") {
		return "", fmt.Errorf("%s ref is empty or malformed", label)
	}

	revision := ref + "^{commit}"
	output, err := runGit(ctx, root, "rev-parse", "--verify", "--end-of-options", revision)
	if err != nil {
		return "", fmt.Errorf("resolve %s ref %q: %w", label, ref, err)
	}
	oid := strings.TrimSpace(string(output))
	if err := validateObjectID(oid); err != nil {
		return "", fmt.Errorf("resolve %s ref %q: %w", label, ref, err)
	}
	return oid, nil
}

func resolveRepositoryIdentity(ctx context.Context, root, explicit string) (string, bool, error) {
	if explicit != "" {
		canonical, err := canonicalizeRepositoryURI(explicit)
		if err != nil {
			return "", false, fmt.Errorf("canonicalize explicit repository URI: %w", err)
		}
		return canonical, false, nil
	}

	output, err := runGit(ctx, root, "config", "--local", "--null", "--get-all", "remote.origin.url")
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			return "", false, fmt.Errorf("read local origin URLs: %w", err)
		}
		return localRepositoryIdentity(root), true, nil
	}

	identities := make(map[string]struct{})
	for _, raw := range strings.Split(string(output), "\x00") {
		if raw == "" {
			continue
		}
		canonical, canonicalErr := canonicalizeRepositoryURI(raw)
		if canonicalErr != nil {
			return "", false, fmt.Errorf("canonicalize origin repository URI %q: %w", raw, canonicalErr)
		}
		identities[canonical] = struct{}{}
	}

	if len(identities) == 0 {
		return localRepositoryIdentity(root), true, nil
	}
	if len(identities) != 1 {
		return "", false, fmt.Errorf("origin resolves to %d distinct repository identities", len(identities))
	}
	for identity := range identities {
		return identity, false, nil
	}
	return "", false, fmt.Errorf("origin repository identity resolution failed")
}

func localRepositoryIdentity(root string) string {
	preimage := localRepositoryIdentityAlgorithm + "\n" + filepath.ToSlash(filepath.Clean(root)) + "\n"
	digest := sha256.Sum256([]byte(preimage))
	return "local://sha256/" + hex.EncodeToString(digest[:])
}

func worktreeDirty(ctx context.Context, root string) (bool, error) {
	output, err := runGit(ctx, root, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignore-submodules=none")
	if err != nil {
		return false, fmt.Errorf("inspect Git worktree status: %w", err)
	}
	return len(output) != 0, nil
}

func runGit(ctx context.Context, worktree string, args ...string) ([]byte, error) {
	commandArgs := make([]string, 0, len(args)+2)
	commandArgs = append(commandArgs, "-C", worktree)
	commandArgs = append(commandArgs, args...)

	cmd := exec.CommandContext(ctx, "git", commandArgs...)
	cmd.Env = sanitizedGitEnvironment(os.Environ())
	output, err := cmd.Output()
	if err == nil {
		return output, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		message := strings.TrimSpace(string(exitErr.Stderr))
		if message != "" {
			return nil, fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), message, err)
		}
	}
	return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
}

func sanitizedGitEnvironment(environment []string) []string {
	blockedExact := map[string]struct{}{
		"GIT_ALTERNATE_OBJECT_DIRECTORIES": {},
		"GIT_CEILING_DIRECTORIES":          {},
		"GIT_COMMON_DIR":                   {},
		"GIT_CONFIG":                       {},
		"GIT_CONFIG_COUNT":                 {},
		"GIT_CONFIG_GLOBAL":                {},
		"GIT_CONFIG_PARAMETERS":            {},
		"GIT_CONFIG_SYSTEM":                {},
		"GIT_DIR":                          {},
		"GIT_DISCOVERY_ACROSS_FILESYSTEM":  {},
		"GIT_INDEX_FILE":                   {},
		"GIT_OBJECT_DIRECTORY":             {},
		"GIT_WORK_TREE":                    {},
	}

	clean := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		if _, blocked := blockedExact[name]; blocked {
			continue
		}
		if strings.HasPrefix(name, "GIT_CONFIG_KEY_") || strings.HasPrefix(name, "GIT_CONFIG_VALUE_") {
			continue
		}
		clean = append(clean, entry)
	}
	return clean
}
