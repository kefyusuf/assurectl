package evidence

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kefyusuf/assurectl/internal/domain"
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


func TestLoadLocalRejectsDuplicateEvidenceID(t *testing.T) {
	t.Parallel()

	root, evidenceRoot, artifactsRoot := newLoadLocalTestRoot(t)
	repository := "github.com/acme/checkout"
	head := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	aDigest := writeLoadLocalArtifact(t, artifactsRoot, "a.json", []byte("artifact-a"))
	bDigest := writeLoadLocalArtifact(t, artifactsRoot, "b.json", []byte("artifact-b"))
	writeLoadLocalEnvelope(t, evidenceRoot, "a.json", loadLocalEnvelopeJSON(
		"ev-duplicate",
		repository,
		head,
		domain.OutcomePassed,
		"artifacts/a.json",
		aDigest,
		"2026-09-02T09:10:00Z",
		"2026-09-02T09:10:01Z",
	))
	writeLoadLocalEnvelope(t, evidenceRoot, "b.json", loadLocalEnvelopeJSON(
		"ev-duplicate",
		repository,
		head,
		domain.OutcomeFailed,
		"artifacts/b.json",
		bDigest,
		"2026-09-02T09:20:00Z",
		"2026-09-02T09:20:01Z",
	))

	got, err := LoadLocal(root, domain.Subject{
		RepositoryURI: repository,
		HeadRevision:  head,
	})
	if err == nil {
		t.Fatalf("LoadLocal() = %#v, want duplicate evidence ID error", got)
	}
	if got != nil {
		t.Fatalf("LoadLocal() returned partial results %#v", got)
	}
	if !strings.Contains(err.Error(), "ev-duplicate") {
		t.Fatalf("LoadLocal() error = %v, want duplicate ID named", err)
	}
}

func TestLoadLocalRejectsDuplicateArtifactURI(t *testing.T) {
	t.Parallel()

	root, evidenceRoot, artifactsRoot := newLoadLocalTestRoot(t)
	repository := "github.com/acme/checkout"
	head := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	sharedDigest := writeLoadLocalArtifact(t, artifactsRoot, "shared.json", []byte("shared-artifact"))
	for _, fixture := range []struct {
		filename string
		id       string
	}{
		{filename: "a.json", id: "ev-a"},
		{filename: "b.json", id: "ev-b"},
	} {
		writeLoadLocalEnvelope(t, evidenceRoot, fixture.filename, loadLocalEnvelopeJSON(
			fixture.id,
			repository,
			head,
			domain.OutcomePassed,
			"artifacts/shared.json",
			sharedDigest,
			"2026-09-02T09:10:00Z",
			"2026-09-02T09:10:01Z",
		))
	}

	got, err := LoadLocal(root, domain.Subject{
		RepositoryURI: repository,
		HeadRevision:  head,
	})
	if err == nil {
		t.Fatalf("LoadLocal() = %#v, want duplicate artifact URI error", got)
	}
	if got != nil {
		t.Fatalf("LoadLocal() returned partial results %#v", got)
	}
	if !strings.Contains(err.Error(), "artifacts/shared.json") {
		t.Fatalf("LoadLocal() error = %v, want duplicate artifact path named", err)
	}
}

func TestLoadLocalFailsClosedForInvalidMember(t *testing.T) {
	t.Parallel()

	const (
		repository = "github.com/acme/checkout"
		head       = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	)

	t.Run("later artifact digest mismatch returns no partial results", func(t *testing.T) {
		t.Parallel()

		root, evidenceRoot, artifactsRoot := newLoadLocalTestRoot(t)
		aDigest := writeLoadLocalArtifact(t, artifactsRoot, "a.json", []byte("artifact-a"))
		_ = writeLoadLocalArtifact(t, artifactsRoot, "z.json", []byte("artifact-z"))
		writeLoadLocalEnvelope(t, evidenceRoot, "a.json", loadLocalEnvelopeJSON(
			"ev-a",
			repository,
			head,
			domain.OutcomePassed,
			"artifacts/a.json",
			aDigest,
			"2026-09-02T09:10:00Z",
			"2026-09-02T09:10:01Z",
		))
		writeLoadLocalEnvelope(t, evidenceRoot, "z.json", loadLocalEnvelopeJSON(
			"ev-z",
			repository,
			head,
			domain.OutcomePassed,
			"artifacts/z.json",
			inputmeta.SHA256([]byte("wrong-artifact")),
			"2026-09-02T09:20:00Z",
			"2026-09-02T09:20:01Z",
		))

		got, err := LoadLocal(root, domain.Subject{
			RepositoryURI: repository,
			HeadRevision:  head,
		})
		if err == nil {
			t.Fatalf("LoadLocal() = %#v, want error", got)
		}
		if got != nil {
			t.Fatalf("LoadLocal() returned partial results %#v", got)
		}
	})

	t.Run("repository mismatch fails complete set", func(t *testing.T) {
		t.Parallel()

		root, evidenceRoot, artifactsRoot := newLoadLocalTestRoot(t)
		aDigest := writeLoadLocalArtifact(t, artifactsRoot, "a.json", []byte("artifact-a"))
		zDigest := writeLoadLocalArtifact(t, artifactsRoot, "z.json", []byte("artifact-z"))
		writeLoadLocalEnvelope(t, evidenceRoot, "a.json", loadLocalEnvelopeJSON(
			"ev-a",
			repository,
			head,
			domain.OutcomePassed,
			"artifacts/a.json",
			aDigest,
			"2026-09-02T09:10:00Z",
			"2026-09-02T09:10:01Z",
		))
		writeLoadLocalEnvelope(t, evidenceRoot, "z.json", loadLocalEnvelopeJSON(
			"ev-z",
			"github.com/acme/other",
			head,
			domain.OutcomePassed,
			"artifacts/z.json",
			zDigest,
			"2026-09-02T09:20:00Z",
			"2026-09-02T09:20:01Z",
		))

		got, err := LoadLocal(root, domain.Subject{
			RepositoryURI: repository,
			HeadRevision:  head,
		})
		if err == nil {
			t.Fatalf("LoadLocal() = %#v, want error", got)
		}
		if got != nil {
			t.Fatalf("LoadLocal() returned partial results %#v", got)
		}
	})

	t.Run("malformed member fails complete set", func(t *testing.T) {
		t.Parallel()

		root, evidenceRoot, artifactsRoot := newLoadLocalTestRoot(t)
		aDigest := writeLoadLocalArtifact(t, artifactsRoot, "a.json", []byte("artifact-a"))
		writeLoadLocalEnvelope(t, evidenceRoot, "a.json", loadLocalEnvelopeJSON(
			"ev-a",
			repository,
			head,
			domain.OutcomePassed,
			"artifacts/a.json",
			aDigest,
			"2026-09-02T09:10:00Z",
			"2026-09-02T09:10:01Z",
		))
		writeLoadLocalEnvelope(t, evidenceRoot, "z.json", "{")

		got, err := LoadLocal(root, domain.Subject{
			RepositoryURI: repository,
			HeadRevision:  head,
		})
		if err == nil {
			t.Fatalf("LoadLocal() = %#v, want error", got)
		}
		if got != nil {
			t.Fatalf("LoadLocal() returned partial results %#v", got)
		}
	})

	t.Run("malformed caller fails even when evidence directory is absent", func(t *testing.T) {
		t.Parallel()

		got, err := LoadLocal(t.TempDir(), domain.Subject{
			RepositoryURI: "https://github.com/acme/checkout.git",
			HeadRevision:  head,
		})
		if err == nil {
			t.Fatalf("LoadLocal() = %#v, want error", got)
		}
		if got != nil {
			t.Fatalf("LoadLocal() returned partial results %#v", got)
		}
	})
}

func newLoadLocalTestRoot(t *testing.T) (string, string, string) {
	t.Helper()

	root := t.TempDir()
	evidenceRoot := filepath.Join(root, ".assurectl", "evidence")
	artifactsRoot := filepath.Join(evidenceRoot, "artifacts")
	if err := os.MkdirAll(artifactsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, evidenceRoot, artifactsRoot
}

func writeLoadLocalArtifact(t *testing.T, artifactsRoot, name string, data []byte) inputmeta.Digest {
	t.Helper()

	if err := os.WriteFile(filepath.Join(artifactsRoot, name), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return inputmeta.SHA256(data)
}

func writeLoadLocalEnvelope(t *testing.T, evidenceRoot, name, envelope string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(evidenceRoot, name), []byte(envelope), 0o644); err != nil {
		t.Fatal(err)
	}
}
