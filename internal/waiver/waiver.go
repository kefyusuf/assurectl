package waiver

import (
	"regexp"

	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/inputmeta"
)

const (
	// SchemaVersion identifies the waiver input instance format.
	SchemaVersion = "assurectl/waivers/v0"
	// TargetRequirement is the only supported waiver target kind; the
	// FINDING kind stays reserved.
	TargetRequirement = "REQUIREMENT"
)

var (
	identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	objectIDPattern   = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)
	digestPattern     = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Target names what a waiver accepts risk for.
type Target struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// Scope bounds where a waiver applies.
type Scope struct {
	RepositoryURI string `json:"repository_uri"`
	WorkUnitID    string `json:"work_unit_id"`
	HeadRevision  string `json:"head_revision,omitempty"`
}

// Approver records who accepted the risk.
type Approver struct {
	Identity  string `json:"identity"`
	Authority string `json:"authority"`
}

// Record is one explicit risk-acceptance claim. The loader recomputes the
// canonical digest and rejects records whose supplied digest does not match,
// so an edited file is tamper-evident.
type Record struct {
	ID           string              `json:"id"`
	Target       Target              `json:"target"`
	Scope        Scope               `json:"scope"`
	Reason       string              `json:"reason"`
	AcceptedRisk string              `json:"accepted_risk"`
	Approver     Approver            `json:"approver"`
	IssuedAt     string              `json:"issued_at"`
	ExpiresAt    string              `json:"expires_at"`
	Status       domain.WaiverStatus `json:"status"`
	Digest       inputmeta.Digest    `json:"digest"`
}

// Loaded carries the parsed records and their source.
type Loaded struct {
	Records []Record
	Source  string
}

type waiversFile struct {
	SchemaVersion string   `json:"schema_version"`
	Waivers       []Record `json:"waivers"`
}
