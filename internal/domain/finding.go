package domain

type FindingSeverity string

const (
	SeverityInfo    FindingSeverity = "INFO"
	SeverityWarning FindingSeverity = "WARNING"
	SeverityError   FindingSeverity = "ERROR"
)

func (v FindingSeverity) Valid() bool {
	switch v {
	case SeverityInfo, SeverityWarning, SeverityError:
		return true
	default:
		return false
	}
}

type Finding struct {
	Code          string          `json:"code"`
	Severity      FindingSeverity `json:"severity"`
	RequirementID string          `json:"requirement_id,omitempty"`
	EvidenceIDs   []string        `json:"evidence_ids,omitempty"`
	Message       string          `json:"message"`
	Blocking      bool            `json:"blocking"`
}
