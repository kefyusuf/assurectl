package contract

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/inputmeta"
	"github.com/kefyusuf/assurectl/internal/strictjson"
)

const (
	SchemaVersion       = "assurectl/verification-contract/v0"
	localContractPath   = ".assurectl/contract.json"
	localContractSource = "workspace:.assurectl/contract.json"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type AcceptanceCriterion struct {
	ID           string   `json:"id"`
	Statement    string   `json:"statement"`
	Requirements []string `json:"requirements"`
}

type Contract struct {
	SchemaVersion      string                `json:"schema_version"`
	WorkUnit           domain.WorkUnit       `json:"work_unit"`
	AcceptanceCriteria []AcceptanceCriterion `json:"acceptance_criteria"`
}

type Loaded struct {
	Contract    Contract
	Source      string
	Digest      inputmeta.Digest
	TrustStatus inputmeta.TrustStatus
}

func LoadLocal(root string) (Loaded, error) {
	if strings.TrimSpace(root) == "" {
		return Loaded{}, errors.New("load contract: workspace root is empty")
	}

	path := filepath.Join(root, filepath.FromSlash(localContractPath))
	data, err := os.ReadFile(path)
	if err != nil {
		return Loaded{}, fmt.Errorf("load contract %s: %w", localContractPath, err)
	}

	var value Contract
	if err := strictjson.Decode(data, &value); err != nil {
		return Loaded{}, fmt.Errorf("load contract %s: %w", localContractPath, err)
	}
	if err := validate(value); err != nil {
		return Loaded{}, fmt.Errorf("load contract %s: %w", localContractPath, err)
	}

	canonical, err := json.Marshal(value)
	if err != nil {
		return Loaded{}, fmt.Errorf("load contract %s: canonicalize: %w", localContractPath, err)
	}

	return Loaded{
		Contract:    value,
		Source:      localContractSource,
		Digest:      inputmeta.SHA256(canonical),
		TrustStatus: inputmeta.TrustStatusUntrusted,
	}, nil
}

func validate(value Contract) error {
	if value.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version %q is unsupported", value.SchemaVersion)
	}
	if err := validateBoundedString("work_unit.id", value.WorkUnit.ID, 256); err != nil {
		return err
	}
	if !value.WorkUnit.Type.Valid() {
		return fmt.Errorf("work_unit.type %q is unsupported", value.WorkUnit.Type)
	}
	if err := validateBoundedString("work_unit.objective", value.WorkUnit.Objective, 4096); err != nil {
		return err
	}
	if len(value.AcceptanceCriteria) == 0 {
		return errors.New("acceptance_criteria must contain at least one item")
	}

	criterionIDs := make(map[string]struct{}, len(value.AcceptanceCriteria))
	for i, criterion := range value.AcceptanceCriteria {
		if !identifierPattern.MatchString(criterion.ID) {
			return fmt.Errorf("acceptance_criteria[%d].id %q is invalid", i, criterion.ID)
		}
		if _, exists := criterionIDs[criterion.ID]; exists {
			return fmt.Errorf("duplicate acceptance criterion id %q", criterion.ID)
		}
		criterionIDs[criterion.ID] = struct{}{}

		if err := validateBoundedString(fmt.Sprintf("acceptance_criteria[%d].statement", i), criterion.Statement, 4096); err != nil {
			return err
		}
		if len(criterion.Requirements) == 0 {
			return fmt.Errorf("acceptance_criteria[%d].requirements must contain at least one item", i)
		}

		requirementIDs := make(map[string]struct{}, len(criterion.Requirements))
		for j, requirementID := range criterion.Requirements {
			if !identifierPattern.MatchString(requirementID) {
				return fmt.Errorf("acceptance_criteria[%d].requirements[%d] %q is invalid", i, j, requirementID)
			}
			if _, exists := requirementIDs[requirementID]; exists {
				return fmt.Errorf("duplicate requirement id %q in acceptance criterion %q", requirementID, criterion.ID)
			}
			requirementIDs[requirementID] = struct{}{}
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
