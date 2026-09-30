package evidence

import "testing"

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
