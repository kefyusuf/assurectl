package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/kefyusuf/assurectl/internal/inputmeta"
	"github.com/kefyusuf/assurectl/internal/localinput"
)

func validatePortableArtifactURI(uri string) error {
	if uri == "" {
		return fmt.Errorf("artifact uri is empty")
	}
	if !utf8.ValidString(uri) {
		return fmt.Errorf("artifact uri is not valid UTF-8")
	}
	if strings.HasPrefix(uri, "/") {
		return fmt.Errorf("artifact uri must be relative")
	}
	if strings.Contains(uri, "\\") {
		return fmt.Errorf("artifact uri contains a backslash")
	}
	for _, char := range uri {
		if char < 0x20 || char == 0x7f {
			return fmt.Errorf("artifact uri contains a control character")
		}
	}
	if path.Clean(uri) != uri {
		return fmt.Errorf("artifact uri is not in canonical slash-separated form")
	}

	for _, segment := range strings.Split(uri, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("artifact uri contains an ambiguous segment")
		}
		if strings.Contains(segment, ":") {
			return fmt.Errorf("artifact uri segment contains a colon")
		}
		if strings.HasSuffix(segment, ".") || strings.HasSuffix(segment, " ") {
			return fmt.Errorf("artifact uri segment has a trailing dot or space")
		}
		if isWindowsDeviceName(segment) {
			return fmt.Errorf("artifact uri segment %q is a Windows device alias", segment)
		}
	}
	return nil
}

func isWindowsDeviceName(segment string) bool {
	base := segment
	if dot := strings.IndexByte(base, '.'); dot >= 0 {
		base = base[:dot]
	}
	base = strings.ToUpper(base)

	switch base {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) {
		return base[3] >= '1' && base[3] <= '9'
	}
	return false
}

func verifyArtifact(evidenceRoot, uri string, expected inputmeta.Digest) error {
	if err := validatePortableArtifactURI(uri); err != nil {
		return err
	}
	if err := validateDigest("artifact.digest", expected); err != nil {
		return err
	}

	file, err := localinput.OpenWorkspaceRegularFile(evidenceRoot, uri)
	if err != nil {
		return fmt.Errorf("open artifact %q: %w", uri, err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("hash artifact %q: %w", uri, err)
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != expected.Value {
		return fmt.Errorf("artifact %q digest mismatch", uri)
	}
	return nil
}
