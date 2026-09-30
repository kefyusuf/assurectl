package evidence

import (
	"fmt"
	"path"
	"strings"
	"unicode/utf8"
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
