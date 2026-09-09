package gitsubject

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

var gitExecutablePath, gitExecutableErr = resolveGitExecutable()

func resolveGitExecutable() (string, error) {
	path, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("resolve Git executable at process start: %w", err)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("normalize Git executable path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve Git executable symlinks: %w", err)
	}
	return filepath.Clean(resolved), nil
}
