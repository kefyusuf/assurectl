package evidence

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kefyusuf/assurectl/internal/inputmeta"
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

func TestArtifactSymlinkContainment(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	evidenceRoot := filepath.Join(workspace, ".assurectl", "evidence")
	artifactsRoot := filepath.Join(evidenceRoot, "artifacts")
	if err := os.MkdirAll(artifactsRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	data := []byte("artifact")
	expected := inputmeta.SHA256(data)

	outsideEvidence := filepath.Join(workspace, "outside-evidence.json")
	if err := os.WriteFile(outsideEvidence, data, 0o644); err != nil {
		t.Fatal(err)
	}
	outsideLink := filepath.Join(artifactsRoot, "outside-link.json")
	if err := os.Symlink(outsideEvidence, outsideLink); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if err := verifyArtifact(evidenceRoot, "artifacts/outside-link.json", expected); err == nil {
		t.Fatal("verifyArtifact(outside evidence-root symlink) error = nil, want error")
	}

	insideTarget := filepath.Join(artifactsRoot, "inside-target.json")
	if err := os.WriteFile(insideTarget, data, 0o644); err != nil {
		t.Fatal(err)
	}
	insideLink := filepath.Join(artifactsRoot, "inside-link.json")
	if err := os.Symlink(insideTarget, insideLink); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if err := verifyArtifact(evidenceRoot, "artifacts/inside-link.json", expected); err != nil {
		t.Fatalf("verifyArtifact(inside evidence-root symlink) error = %v", err)
	}
}
