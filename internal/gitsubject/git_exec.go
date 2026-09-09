package gitsubject

import (
	"fmt"
	"os"
	"runtime"
)

func trustedGitExecutable() (string, error) {
	candidates := []string{
		"/usr/bin/git",
		"/usr/local/bin/git",
		"/opt/homebrew/bin/git",
		"/opt/local/bin/git",
	}
	if runtime.GOOS == "windows" {
		candidates = []string{
			`C:\Program Files\Git\cmd\git.exe`,
			`C:\Program Files\Git\bin\git.exe`,
		}
	}

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
			continue
		}
		return candidate, nil
	}
	return "", fmt.Errorf("trusted Git executable not found in the supported absolute-path allowlist")
}
