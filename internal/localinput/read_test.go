package localinput

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenWorkspaceRegularFile(t *testing.T) {
	t.Parallel()

	t.Run("regular file", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "data.txt")
		if err := os.WriteFile(path, []byte("ok"), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		file, err := OpenWorkspaceRegularFile(root, "data.txt")
		if err != nil {
			t.Fatalf("OpenWorkspaceRegularFile() error = %v", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	t.Run("relative traversal", func(t *testing.T) {
		root := t.TempDir()
		if file, err := OpenWorkspaceRegularFile(root, "../outside.txt"); err == nil {
			file.Close()
			t.Fatal("OpenWorkspaceRegularFile() error = nil")
		}
	})

	t.Run("directory target", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, "dir"), 0o755); err != nil {
			t.Fatalf("Mkdir() error = %v", err)
		}
		if file, err := OpenWorkspaceRegularFile(root, "dir"); err == nil {
			file.Close()
			t.Fatal("OpenWorkspaceRegularFile() error = nil")
		}
	})

	t.Run("symlink escape", func(t *testing.T) {
		root := t.TempDir()
		outsideRoot := t.TempDir()
		outside := filepath.Join(outsideRoot, "outside.txt")
		if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		link := filepath.Join(root, "escape.txt")
		if err := os.Symlink(outside, link); err != nil {
			t.Skipf("Symlink() unsupported: %v", err)
		}

		if file, err := OpenWorkspaceRegularFile(root, "escape.txt"); err == nil {
			file.Close()
			t.Fatal("OpenWorkspaceRegularFile() error = nil")
		}
	})
}

func TestReadWorkspaceFileStillBoundsInput(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	data := make([]byte, MaxWorkspaceInputBytes+1)
	if err := os.WriteFile(filepath.Join(root, "large.bin"), data, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := ReadWorkspaceFile(root, "large.bin")
	if err == nil {
		t.Fatal("ReadWorkspaceFile() error = nil")
	}
	if !strings.Contains(err.Error(), "exceeds maximum size") {
		t.Fatalf("ReadWorkspaceFile() error = %q, want size-limit error", err)
	}
}
