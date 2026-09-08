package gitsubject

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

const changeSetAlgorithm = "assurectl.git-change-set/v0"

func canonicalizeRepositoryURI(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("repository URI is empty")
	}
	if strings.TrimSpace(raw) != raw {
		return "", fmt.Errorf("repository URI has surrounding whitespace")
	}
	if strings.ContainsAny(raw, "\x00\r\n") {
		return "", fmt.Errorf("repository URI contains a control character")
	}
	if strings.Contains(raw, "%") {
		return "", fmt.Errorf("repository URI percent-encoding is unsupported")
	}

	if !strings.Contains(raw, "://") {
		return canonicalizeSCPLikeURI(raw)
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse repository URI: %w", err)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("repository URI query and fragment are unsupported")
	}
	if parsed.Port() != "" {
		return "", fmt.Errorf("repository URI ports are unsupported")
	}

	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		if parsed.User != nil {
			return "", fmt.Errorf("repository URI credentials are unsupported")
		}
	case "ssh":
		if parsed.User == nil {
			return "", fmt.Errorf("repository URI SSH username is required")
		}
		if _, hasPassword := parsed.User.Password(); hasPassword {
			return "", fmt.Errorf("repository URI passwords are unsupported")
		}
		if parsed.User.Username() != "git" {
			return "", fmt.Errorf("repository URI SSH username is unsupported")
		}
	default:
		return "", fmt.Errorf("repository URI scheme %q is unsupported", parsed.Scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		return "", fmt.Errorf("repository URI host is empty")
	}
	return canonicalHostPath(host, parsed.Path)
}

func canonicalizeSCPLikeURI(raw string) (string, error) {
	at := strings.IndexByte(raw, '@')
	if at <= 0 {
		return "", fmt.Errorf("repository URI is neither a supported URL nor SCP-like SSH")
	}
	colonOffset := strings.IndexByte(raw[at+1:], ':')
	if colonOffset < 0 {
		return "", fmt.Errorf("SCP-like repository URI has no path separator")
	}
	colon := at + 1 + colonOffset
	if colon == len(raw)-1 {
		return "", fmt.Errorf("SCP-like repository URI path is empty")
	}

	user := raw[:at]
	host := raw[at+1 : colon]
	path := raw[colon+1:]
	if user == "" || host == "" {
		return "", fmt.Errorf("SCP-like repository URI user or host is empty")
	}
	if user != "git" {
		return "", fmt.Errorf("SCP-like repository URI SSH username is unsupported")
	}
	if strings.ContainsAny(user+host, "/\\?#") {
		return "", fmt.Errorf("SCP-like repository URI user or host is malformed")
	}
	if strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("SCP-like repository URI absolute paths are unsupported")
	}
	return canonicalHostPath(host, path)
}

func canonicalHostPath(host, rawPath string) (string, error) {
	if strings.TrimSpace(host) != host {
		return "", fmt.Errorf("repository URI host has surrounding whitespace")
	}
	host = strings.ToLower(host)
	if host == "" || strings.ContainsAny(host, " \t/@?#\\") {
		return "", fmt.Errorf("repository URI host is malformed")
	}

	path := strings.TrimPrefix(rawPath, "/")
	path = strings.TrimSuffix(path, "/")
	path = strings.TrimSuffix(path, ".git")
	if path == "" {
		return "", fmt.Errorf("repository URI path is empty")
	}
	if strings.Contains(path, "\\") {
		return "", fmt.Errorf("repository URI path contains a backslash")
	}

	segments := strings.Split(path, "/")
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return "", fmt.Errorf("repository URI path contains an ambiguous segment")
		}
		if strings.ContainsAny(segment, "\x00\r\n?#") {
			return "", fmt.Errorf("repository URI path contains an unsupported character")
		}
	}

	return host + "/" + strings.Join(segments, "/"), nil
}

func changeSetDigest(repositoryURI, baseRevision, headRevision string) (string, error) {
	if repositoryURI == "" || strings.TrimSpace(repositoryURI) != repositoryURI || strings.ContainsAny(repositoryURI, "\x00\r\n") {
		return "", fmt.Errorf("canonical repository URI is invalid")
	}
	if err := validateObjectID(baseRevision); err != nil {
		return "", fmt.Errorf("base revision: %w", err)
	}
	if err := validateObjectID(headRevision); err != nil {
		return "", fmt.Errorf("head revision: %w", err)
	}

	preimage := changeSetAlgorithm + "\n" + repositoryURI + "\n" + baseRevision + "\n" + headRevision + "\n"
	digest := sha256.Sum256([]byte(preimage))
	return hex.EncodeToString(digest[:]), nil
}

func validateObjectID(value string) error {
	if len(value) != 40 && len(value) != 64 {
		return fmt.Errorf("object ID length %d is not 40 or 64", len(value))
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return fmt.Errorf("object ID must be lowercase hexadecimal")
		}
	}
	return nil
}
