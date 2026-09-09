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

	dirty, err := worktreeDirty(ctx, root, headRevision)
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
	callerPath, err := normalizeFilesystemPath(absolute)
	if err != nil {
		return "", fmt.Errorf("normalize caller worktree path: %w", err)
	}

	initialGitDir, err := resolveGitDirectory(ctx, callerPath)
	if err != nil {
		return "", fmt.Errorf("resolve Git directory from caller path: %w", err)
	}

	rootOutput, err := runGit(ctx, callerPath, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("resolve Git worktree root: %w", err)
	}
	root, err := parseGitPathOutput(rootOutput)
	if err != nil {
		return "", fmt.Errorf("parse Git worktree root: %w", err)
	}
	root, err = normalizeFilesystemPath(root)
	if err != nil {
		return "", fmt.Errorf("normalize Git worktree root: %w", err)
	}

	inside, err := pathWithin(root, callerPath)
	if err != nil {
		return "", fmt.Errorf("compare caller and discovered worktree paths: %w", err)
	}
	if !inside {
		return "", fmt.Errorf("caller path is outside the discovered Git worktree")
	}

	rootGitDir, err := resolveGitDirectory(ctx, root)
	if err != nil {
		return "", fmt.Errorf("resolve Git directory from discovered root: %w", err)
	}
	if initialGitDir != rootGitDir {
		return "", fmt.Errorf("discovered worktree root belongs to a different Git directory")
	}

	insideOutput, err := runGit(ctx, root, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return "", fmt.Errorf("verify Git worktree: %w", err)
	}
	if strings.TrimSpace(string(insideOutput)) != "true" {
		return "", fmt.Errorf("path is not inside a Git worktree")
	}
	return root, nil
}

func pathWithin(root, candidate string) (bool, error) {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false, err
	}
	if relative == "." {
		return true, nil
	}
	parent := ".." + string(filepath.Separator)
	return relative != ".." && !strings.HasPrefix(relative, parent), nil
}

func resolveGitDirectory(ctx context.Context, worktree string) (string, error) {
	output, err := runGit(ctx, worktree, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return "", err
	}
	path, err := parseGitPathOutput(output)
	if err != nil {
		return "", err
	}
	return normalizeFilesystemPath(path)
}

func normalizeFilesystemPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	absolute = filepath.Clean(absolute)
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

func parseGitPathOutput(output []byte) (string, error) {
	text := string(output)
	if !strings.HasSuffix(text, "\n") {
		return "", fmt.Errorf("git path output is not newline-terminated")
	}
	text = strings.TrimSuffix(text, "\n")
	text = strings.TrimSuffix(text, "\r")
	if text == "" {
		return "", fmt.Errorf("git path output is empty")
	}
	if strings.ContainsAny(text, "\x00\r\n") {
		return "", fmt.Errorf("git path output contains an unexpected record separator")
	}
	return text, nil
}

func resolveCommit(ctx context.Context, root, label, ref string) (string, error) {
	if ref == "" || strings.TrimSpace(ref) != ref || strings.ContainsAny(ref, "\x00\r\n") {
		return "", fmt.Errorf("%s ref is empty or malformed", label)
	}
	if err := validateRefInput(ctx, root, label, ref); err != nil {
		return "", err
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

func validateRefInput(ctx context.Context, root, label, ref string) error {
	if ref == "HEAD" {
		return nil
	}
	if validateObjectID(ref) == nil {
		return nil
	}
	if !strings.HasPrefix(ref, "refs/") {
		return fmt.Errorf("%s ref must be HEAD, a full object ID, or a fully qualified refs/... name", label)
	}
	if _, err := runGit(ctx, root, "check-ref-format", ref); err != nil {
		return fmt.Errorf("%s ref %q is not a canonical full ref: %w", label, ref, err)
	}
	return nil
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
	for index, raw := range strings.Split(string(output), "\x00") {
		if raw == "" {
			continue
		}
		canonical, canonicalErr := canonicalizeRepositoryURI(raw)
		if canonicalErr != nil {
			return "", false, fmt.Errorf("canonicalize origin repository URI #%d: %w", index+1, canonicalErr)
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

func worktreeDirty(ctx context.Context, root, expectedHead string) (bool, error) {
	checkoutHead, err := resolveCommit(ctx, root, "checkout HEAD", "HEAD")
	if err != nil {
		return false, fmt.Errorf("resolve checkout HEAD: %w", err)
	}
	if checkoutHead != expectedHead {
		return true, nil
	}

	externalFilter, err := repositoryHasExternalCleanFilter(ctx, root)
	if err != nil {
		return false, err
	}
	if externalFilter {
		return true, nil
	}

	indexOutput, err := runGit(ctx, root, "ls-files", "-s", "-v", "-z")
	if err != nil {
		return false, fmt.Errorf("inspect Git index flags: %w", err)
	}
	for _, record := range strings.Split(string(indexOutput), "\x00") {
		if record == "" {
			continue
		}
		tab := strings.IndexByte(record, '\t')
		if tab < 0 {
			return false, fmt.Errorf("inspect Git index flags: malformed ls-files record")
		}
		metadata := strings.Fields(record[:tab])
		if len(metadata) != 4 || len(metadata[0]) != 1 {
			return false, fmt.Errorf("inspect Git index flags: malformed ls-files metadata")
		}
		tag := metadata[0][0]
		mode := metadata[1]
		if (tag >= 'a' && tag <= 'z') || tag == 'S' || mode == "160000" {
			return true, nil
		}
	}

	output, err := runGit(ctx, root, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignore-submodules=none")
	if err != nil {
		return false, fmt.Errorf("inspect Git worktree status: %w", err)
	}
	return len(output) != 0, nil
}

func repositoryHasExternalCleanFilter(ctx context.Context, root string) (bool, error) {
	output, err := runGit(ctx, root, "config", "--includes", "--local", "--null", "--get-regexp", `^filter\..*\.(clean|process)$`)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("inspect Git filter configuration: %w", err)
	}
	return len(output) != 0, nil
}

func runGit(ctx context.Context, worktree string, args ...string) ([]byte, error) {
	commandArgs := make([]string, 0, len(args)+16)
	commandArgs = append(commandArgs,
		"-c", "core.fsmonitor=false",
		"-c", "core.filemode=true",
		"-c", "core.symlinks=true",
		"-c", "core.trustctime=true",
		"-c", "core.checkStat=default",
		"-c", "core.ignoreStat=false",
		"-C", worktree,
	)
	commandArgs = append(commandArgs, args...)

	gitExecutable, err := trustedGitExecutable()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, gitExecutable, commandArgs...)
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
		"GIT_ASKPASS":                      {},
		"GIT_CEILING_DIRECTORIES":          {},
		"GIT_COMMON_DIR":                   {},
		"GIT_CONFIG":                       {},
		"GIT_CONFIG_COUNT":                 {},
		"GIT_CONFIG_GLOBAL":                {},
		"GIT_CONFIG_NOSYSTEM":              {},
		"GIT_CONFIG_PARAMETERS":            {},
		"GIT_CONFIG_SYSTEM":                {},
		"GIT_DIR":                          {},
		"GIT_DISCOVERY_ACROSS_FILESYSTEM":  {},
		"GIT_EXEC_PATH":                    {},
		"GIT_INDEX_FILE":                   {},
		"GIT_NAMESPACE":                    {},
		"GIT_NO_LAZY_FETCH":                {},
		"GIT_NO_REPLACE_OBJECTS":           {},
		"GIT_OBJECT_DIRECTORY":             {},
		"GIT_OPTIONAL_LOCKS":               {},
		"GIT_SSH":                          {},
		"GIT_SSH_COMMAND":                  {},
		"GIT_TERMINAL_PROMPT":              {},
		"GIT_WORK_TREE":                    {},
		"SSH_ASKPASS":                      {},
	}

	clean := make([]string, 0, len(environment)+6)
	for _, entry := range environment {
		name, _, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		canonicalName := strings.ToUpper(name)
		if _, blocked := blockedExact[canonicalName]; blocked {
			continue
		}
		if strings.HasPrefix(canonicalName, "GIT_CONFIG_KEY_") || strings.HasPrefix(canonicalName, "GIT_CONFIG_VALUE_") {
			continue
		}
		if strings.HasPrefix(canonicalName, "GIT_TRACE") {
			continue
		}
		clean = append(clean, entry)
	}

	clean = append(clean,
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_NO_LAZY_FETCH=1",
		"GIT_NO_REPLACE_OBJECTS=1",
		"GIT_OPTIONAL_LOCKS=0",
	)
	return clean
}
