package waiver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/inputmeta"
	"github.com/kefyusuf/assurectl/internal/localinput"
	"github.com/kefyusuf/assurectl/internal/strictjson"
)

// CanonicalDigest returns the SHA-256 over the canonical JSON of the record
// with the digest field excluded: the digest binds every other supplied
// field.
func CanonicalDigest(record Record) inputmeta.Digest {
	body := record
	body.Digest = inputmeta.Digest{}
	canonical, err := json.Marshal(body)
	if err != nil {
		return inputmeta.Digest{}
	}
	return inputmeta.SHA256(canonical)
}

// LoadLocal reads a waiver file from the workspace. A missing file yields an
// empty set (the M1 behavior); a present file must be a valid waiver
// document or the whole load fails.
func LoadLocal(root, relativePath string) (Loaded, error) {
	if strings.TrimSpace(relativePath) == "" {
		return Loaded{}, nil
	}

	data, err := localinput.ReadWorkspaceFile(root, relativePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Loaded{}, nil
		}
		return Loaded{}, fmt.Errorf("load waivers %s: %w", relativePath, err)
	}

	var file waiversFile
	if err := strictjson.Decode(data, &file); err != nil {
		return Loaded{}, fmt.Errorf("load waivers %s: %w", relativePath, err)
	}
	if file.SchemaVersion != SchemaVersion {
		return Loaded{}, fmt.Errorf("load waivers %s: schema_version %q is unsupported", relativePath, file.SchemaVersion)
	}

	loaded := Loaded{Source: "workspace:" + relativePath}
	seen := make(map[string]struct{}, len(file.Waivers))
	for i, record := range file.Waivers {
		if err := validateRecord(record); err != nil {
			return Loaded{}, fmt.Errorf("load waivers %s: waivers[%d]: %w", relativePath, i, err)
		}
		if _, exists := seen[record.ID]; exists {
			return Loaded{}, fmt.Errorf("load waivers %s: waivers[%d]: duplicate id %q", relativePath, i, record.ID)
		}
		seen[record.ID] = struct{}{}
		if record.Digest != CanonicalDigest(record) {
			return Loaded{}, fmt.Errorf(
				"load waivers %s: waivers[%d] (%q): supplied digest does not match the record contents",
				relativePath, i, record.ID,
			)
		}
		loaded.Records = append(loaded.Records, record)
	}
	return loaded, nil
}

func validateRecord(record Record) error {
	if !identifierPattern.MatchString(record.ID) {
		return fmt.Errorf("id %q is invalid", record.ID)
	}
	if record.Target.Kind != TargetRequirement {
		return fmt.Errorf("target.kind %q is unsupported: only REQUIREMENT waivers are accepted", record.Target.Kind)
	}
	if !identifierPattern.MatchString(record.Target.ID) {
		return fmt.Errorf("target.id %q is invalid", record.Target.ID)
	}
	if strings.TrimSpace(record.Scope.RepositoryURI) == "" {
		return fmt.Errorf("scope.repository_uri must not be empty")
	}
	if !identifierPattern.MatchString(record.Scope.WorkUnitID) {
		return fmt.Errorf("scope.work_unit_id %q is invalid", record.Scope.WorkUnitID)
	}
	if record.Scope.HeadRevision != "" && !objectIDPattern.MatchString(record.Scope.HeadRevision) {
		return fmt.Errorf("scope.head_revision %q is invalid", record.Scope.HeadRevision)
	}
	if err := validateBounded("reason", record.Reason, 1024); err != nil {
		return err
	}
	if err := validateBounded("accepted_risk", record.AcceptedRisk, 1024); err != nil {
		return err
	}
	if strings.TrimSpace(record.Approver.Identity) == "" {
		return fmt.Errorf("approver.identity must not be empty")
	}
	if strings.TrimSpace(record.Approver.Authority) == "" {
		return fmt.Errorf("approver.authority must not be empty")
	}
	issued, err := parseTimestamp("issued_at", record.IssuedAt)
	if err != nil {
		return err
	}
	expires, err := parseTimestamp("expires_at", record.ExpiresAt)
	if err != nil {
		return err
	}
	if expires.Before(issued) {
		return fmt.Errorf("expires_at precedes issued_at")
	}
	if record.Status != domain.WaiverValid && record.Status != domain.WaiverInvalid {
		return fmt.Errorf("status %q is unsupported", record.Status)
	}
	if !digestPattern.MatchString(record.Digest.Value) || record.Digest.Algorithm != inputmeta.DigestAlgorithmSHA256 {
		return fmt.Errorf("digest must be a sha256 value")
	}
	return nil
}

func validateBounded(field, value string, max int) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s must not be empty", field)
	}
	if len(value) > max {
		return fmt.Errorf("%s exceeds maximum length %d", field, max)
	}
	return nil
}

func parseTimestamp(field, value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s %q is not a valid RFC3339 timestamp", field, value)
	}
	return parsed, nil
}
