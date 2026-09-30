package evidence

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverLocalRejectsUnsafeEvidenceRoot(t *testing.T) {
	t.Parallel()

	t.Run("evidence root symlink outside workspace", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		outside := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".assurectl"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(root, ".assurectl", "evidence")); err != nil {
			t.Skipf("symlink unsupported: %v", err)
		}

		if got, err := discoverLocal(root); err == nil {
			t.Fatalf("discoverLocal() = %#v, want error", got)
		}
	})

	t.Run("evidence root must be a directory", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".assurectl"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".assurectl", "evidence"), []byte("not a directory"), 0o644); err != nil {
			t.Fatal(err)
		}

		if got, err := discoverLocal(root); err == nil {
			t.Fatalf("discoverLocal() = %#v, want error", got)
		}
	})

	t.Run("selected unreadable candidate fails discovery", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		evidenceRoot := filepath.Join(root, ".assurectl", "evidence")
		if err := os.MkdirAll(evidenceRoot, 0o755); err != nil {
			t.Fatal(err)
		}
		candidate := filepath.Join(evidenceRoot, "blocked.json")
		if err := os.WriteFile(candidate, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(candidate, 0); err != nil {
			t.Skipf("cannot remove read permission: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(candidate, 0o600) })

		if got, err := discoverLocal(root); err == nil {
			t.Skipf("platform/user can still read mode-000 file; discovery returned %#v", got)
		}
	})
}
