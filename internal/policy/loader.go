package policy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/kefyusuf/assurectl/internal/inputmeta"
	"github.com/kefyusuf/assurectl/internal/strictjson"
)

const (
	SchemaVersion     = "assurectl/policy/v0"
	localPolicyPath   = ".assurectl/policy.json"
	localPolicySource = "workspace:.assurectl/policy.json"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type Requirement struct {
	ID                   string   `json:"id"`
	EvidenceType         string   `json:"evidence_type"`
	Waivable             bool     `json:"waivable"`
	AllowedProducerTypes []string `json:"allowed_producer_types"`
	MaxAgeSeconds        *uint64  `json:"max_age_seconds,omitempty"`
}

type Policy struct {
	SchemaVersion string        `json:"schema_version"`
	Requirements  []Requirement `json:"requirements"`
}

type Loaded struct {
	Policy         Policy
	Source         string
	Digest         inputmeta.Digest
	TrustStatus    inputmeta.TrustStatus
	AuthorityBasis inputmeta.AuthorityBasis
}

type rawPolicy struct {
	SchemaVersion string           `json:"schema_version"`
	Requirements  []rawRequirement `json:"requirements"`
}

type rawRequirement struct {
	ID                   string          `json:"id"`
	EvidenceType         string          `json:"evidence_type"`
	Waivable             json.RawMessage `json:"waivable"`
	AllowedProducerTypes []string        `json:"allowed_producer_types"`
	MaxAgeSeconds        json.RawMessage `json:"max_age_seconds,omitempty"`
}

func LoadLocal(root string) (Loaded, error) {
	if strings.TrimSpace(root) == "" {
		return Loaded{}, errors.New("load policy: workspace root is empty")
	}

	path := filepath.Join(root, filepath.FromSlash(localPolicyPath))
	data, err := os.ReadFile(path)
	if err != nil {
		return Loaded{}, fmt.Errorf("load policy %s: %w", localPolicyPath, err)
	}

	var raw rawPolicy
	if err := strictjson.Decode(data, &raw); err != nil {
		return Loaded{}, fmt.Errorf("load policy %s: %w", localPolicyPath, err)
	}

	value, err := normalize(raw)
	if err != nil {
		return Loaded{}, fmt.Errorf("load policy %s: %w", localPolicyPath, err)
	}
	if err := validate(value); err != nil {
		return Loaded{}, fmt.Errorf("load policy %s: %w", localPolicyPath, err)
	}

	canonical, err := json.Marshal(value)
	if err != nil {
		return Loaded{}, fmt.Errorf("load policy %s: canonicalize: %w", localPolicyPath, err)
	}

	return Loaded{
		Policy:         value,
		Source:         localPolicySource,
		Digest:         inputmeta.SHA256(canonical),
		TrustStatus:    inputmeta.TrustStatusUntrusted,
		AuthorityBasis: inputmeta.AuthorityBasisAdvisoryWorkspace,
	}, nil
}

func normalize(raw rawPolicy) (Policy, error) {
	value := Policy{
		SchemaVersion: raw.SchemaVersion,
		Requirements:  make([]Requirement, 0, len(raw.Requirements)),
	}

	for i, item := range raw.Requirements {
		waivable, err := requiredBool(item.Waivable)
		if err != nil {
			return Policy{}, fmt.Errorf("requirements[%d].waivable %w", i, err)
		}

		maxAgeSeconds, err := optionalNonNegativeInteger(item.MaxAgeSeconds)
		if err != nil {
			return Policy{}, fmt.Errorf("requirements[%d].max_age_seconds %w", i, err)
		}

		value.Requirements = append(value.Requirements, Requirement{
			ID:                   item.ID,
			EvidenceType:         item.EvidenceType,
			Waivable:             waivable,
			AllowedProducerTypes: item.AllowedProducerTypes,
			MaxAgeSeconds:        maxAgeSeconds,
		})
	}

	return value, nil
}

func requiredBool(raw json.RawMessage) (bool, error) {
	if len(raw) == 0 {
		return false, errors.New("is required")
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return false, errors.New("must be a boolean")
	}

	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, errors.New("must be a boolean")
	}
	return value, nil
}

func optionalNonNegativeInteger(raw json.RawMessage) (*uint64, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, errors.New("must be a non-negative integer")
	}

	var value uint64
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, errors.New("must be a non-negative integer")
	}
	return &value, nil
}

func validate(value Policy) error {
	if value.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version %q is unsupported", value.SchemaVersion)
	}
	if len(value.Requirements) == 0 {
		return errors.New("requirements must contain at least one item")
	}

	requirementIDs := make(map[string]struct{}, len(value.Requirements))
	for i, requirement := range value.Requirements {
		if !identifierPattern.MatchString(requirement.ID) {
			return fmt.Errorf("requirements[%d].id %q is invalid", i, requirement.ID)
		}
		if _, exists := requirementIDs[requirement.ID]; exists {
			return fmt.Errorf("duplicate requirement id %q", requirement.ID)
		}
		requirementIDs[requirement.ID] = struct{}{}

		if err := validateBoundedString(fmt.Sprintf("requirements[%d].evidence_type", i), requirement.EvidenceType, 128); err != nil {
			return err
		}
		if len(requirement.AllowedProducerTypes) == 0 {
			return fmt.Errorf("requirements[%d].allowed_producer_types must contain at least one item", i)
		}

		producerTypes := make(map[string]struct{}, len(requirement.AllowedProducerTypes))
		for j, producerType := range requirement.AllowedProducerTypes {
			if err := validateBoundedString(fmt.Sprintf("requirements[%d].allowed_producer_types[%d]", i, j), producerType, 128); err != nil {
				return err
			}
			if _, exists := producerTypes[producerType]; exists {
				return fmt.Errorf("duplicate allowed producer type %q in requirement %q", producerType, requirement.ID)
			}
			producerTypes[producerType] = struct{}{}
		}
	}

	return nil
}

func validateBoundedString(field, value string, max int) error {
	length := utf8.RuneCountInString(value)
	if length == 0 {
		return fmt.Errorf("%s must not be empty", field)
	}
	if length > max {
		return fmt.Errorf("%s exceeds maximum length %d", field, max)
	}
	return nil
}
