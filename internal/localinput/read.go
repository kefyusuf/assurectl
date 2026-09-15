package localinput

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// MaxWorkspaceInputBytes bounds advisory configuration reads before parsing.
const MaxWorkspaceInputBytes int64 = 1 << 20

func ReadWorkspaceFile(root, relativePath string) ([]byte, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("workspace root is empty")
	}

	relativeOSPath := filepath.FromSlash(relativePath)
	cleanRelativePath := filepath.Clean(relativeOSPath)
	if cleanRelativePath == "." || filepath.IsAbs(cleanRelativePath) || cleanRelativePath == ".." || strings.HasPrefix(cleanRelativePath, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("workspace input path %q is invalid", relativePath)
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}
	rootInfo, err := os.Stat(resolvedRoot)
	if err != nil {
		return nil, fmt.Errorf("stat workspace root: %w", err)
	}
	if !rootInfo.IsDir() {
		return nil, errors.New("workspace root is not a directory")
	}

	logicalPath := filepath.Join(resolvedRoot, cleanRelativePath)
	resolvedPath, err := filepath.EvalSymlinks(logicalPath)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace input %q: %w", relativePath, err)
	}
	inside, err := pathWithin(resolvedRoot, resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace input %q: %w", relativePath, err)
	}
	if !inside {
		return nil, fmt.Errorf("workspace input %q resolves outside workspace", relativePath)
	}

	info, err := os.Stat(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("stat workspace input %q: %w", relativePath, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("workspace input %q is not a regular file", relativePath)
	}
	if info.Size() > MaxWorkspaceInputBytes {
		return nil, fmt.Errorf("workspace input %q exceeds maximum size of %d bytes", relativePath, MaxWorkspaceInputBytes)
	}

	file, err := os.Open(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("open workspace input %q: %w", relativePath, err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, MaxWorkspaceInputBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read workspace input %q: %w", relativePath, err)
	}
	if int64(len(data)) > MaxWorkspaceInputBytes {
		return nil, fmt.Errorf("workspace input %q exceeds maximum size of %d bytes", relativePath, MaxWorkspaceInputBytes)
	}
	return data, nil
}

func pathWithin(root, candidate string) (bool, error) {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false, err
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)), nil
}
