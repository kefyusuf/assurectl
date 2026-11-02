package receipt

import (
	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/inputmeta"
)

// WaiverRecord mirrors the v0 draft's waiver_record definition. M1 emits an
// empty waiver array; the type exists so the receipt builder's semantic
// checks (exact target matching, authority, scope, timing) are already
// fail-closed when M2 introduces waiver inputs.
type WaiverRecord struct {
	ID           string              `json:"id"`
	Target       WaiverTarget        `json:"target"`
	Scope        WaiverScope         `json:"scope"`
	Reason       string              `json:"reason"`
	AcceptedRisk string              `json:"accepted_risk"`
	Approver     WaiverApprover      `json:"approver"`
	IssuedAt     string              `json:"issued_at"`
	ExpiresAt    string              `json:"expires_at"`
	Status       domain.WaiverStatus `json:"status"`
	Digest       inputmeta.Digest    `json:"digest"`
}

// WaiverTarget names what the waiver accepts risk for.
type WaiverTarget struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// WaiverScope bounds where the waiver applies.
type WaiverScope struct {
	RepositoryURI string `json:"repository_uri"`
	WorkUnitID    string `json:"work_unit_id"`
	HeadRevision  string `json:"head_revision,omitempty"`
}

// WaiverApprover records who authorized the risk acceptance.
type WaiverApprover struct {
	Identity  string `json:"identity"`
	Authority string `json:"authority"`
}
