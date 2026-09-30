package evidence

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/kefyusuf/assurectl/internal/inputmeta"
)

func TestValidatePortableArtifactURI(t *testing.T) {
	t.Parallel()

	if err := validatePortableArtifactURI("artifacts/unit-tests.json"); err != nil {
		t.Fatalf("validatePortableArtifactURI(valid) error = %v", err)
	}

	rejected := []string{
		"/artifact.json",
		"../artifact.json",
		"artifacts/../artifact.json",
		"./artifacts/x.json",
		"artifacts//x.json",
		"artifacts\\x.json",
		"C:\\x.json",
		"\\\\server\\share\\x.json",
		"file:///x.json",
		"https://example.com/x",
		"artifacts/name:variant.json",
		"artifacts/control\n.json",
		"artifacts/trailing.",
		"artifacts/trailing ",
		"artifacts/CON",
		"artifacts/NUL.txt",
		"artifacts/com1.log",
		"artifacts/Lpt9",
	}
	for _, uri := range rejected {
		uri := uri
		t.Run(uri, func(t *testing.T) {
			t.Parallel()

			if err := validatePortableArtifactURI(uri); err == nil {
				t.Fatalf("validatePortableArtifactURI(%q) error = nil, want error", uri)
			}
		})
	}
}

func TestVerifyArtifact(t *testing.T) {
	t.Parallel()

	evidenceRoot := t.TempDir()
	artifactsRoot := filepath.Join(evidenceRoot, "artifacts")
	if err := os.MkdirAll(artifactsRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	data := []byte("verified artifact")
	if err := os.WriteFile(filepath.Join(artifactsRoot, "unit-tests.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	expected := inputmeta.SHA256(data)

	if err := verifyArtifact(evidenceRoot, "artifacts/unit-tests.json", expected); err != nil {
		t.Fatalf("verifyArtifact(valid) error = %v", err)
	}

	if err := verifyArtifact(evidenceRoot, "artifacts/missing.json", expected); err == nil {
		t.Fatal("verifyArtifact(missing) error = nil, want error")
	}

	if err := os.Mkdir(filepath.Join(artifactsRoot, "directory.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := verifyArtifact(evidenceRoot, "artifacts/directory.json", expected); err == nil {
		t.Fatal("verifyArtifact(directory) error = nil, want error")
	}

	mismatch := inputmeta.SHA256([]byte("different artifact"))
	if err := verifyArtifact(evidenceRoot, "artifacts/unit-tests.json", mismatch); err == nil {
		t.Fatal("verifyArtifact(digest mismatch) error = nil, want error")
	}

	unsupported := expected
	unsupported.Algorithm = "sha512"
	if err := verifyArtifact(evidenceRoot, "artifacts/unit-tests.json", unsupported); err == nil {
		t.Fatal("verifyArtifact(unsupported algorithm) error = nil, want error")
	}

	large := bytes.Repeat([]byte("x"), 3<<20)
	if err := os.WriteFile(filepath.Join(artifactsRoot, "large.bin"), large, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyArtifact(evidenceRoot, "artifacts/large.bin", inputmeta.SHA256(large)); err != nil {
		t.Fatalf("verifyArtifact(large) error = %v", err)
	}
}
