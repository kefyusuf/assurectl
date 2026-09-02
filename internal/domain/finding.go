package domain

type FindingCategory string

const (
	FindingCategorySubject      FindingCategory = "SUBJECT"
	FindingCategoryContract     FindingCategory = "CONTRACT"
	FindingCategoryPolicy       FindingCategory = "POLICY"
	FindingCategoryEvidence     FindingCategory = "EVIDENCE"
	FindingCategoryVerification FindingCategory = "VERIFICATION"
	FindingCategoryWaiver       FindingCategory = "WAIVER"
	FindingCategoryAuthority    FindingCategory = "AUTHORITY"
	FindingCategoryProtocol     FindingCategory = "PROTOCOL"
	FindingCategoryInternal     FindingCategory = "INTERNAL"
)

func (v FindingCategory) Valid() bool {
	switch v {
	case FindingCategorySubject,
		FindingCategoryContract,
		FindingCategoryPolicy,
		FindingCategoryEvidence,
		FindingCategoryVerification,
		FindingCategoryWaiver,
		FindingCategoryAuthority,
		FindingCategoryProtocol,
		FindingCategoryInternal:
		return true
	default:
		return false
	}
}

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
	Category      FindingCategory `json:"category"`
	Severity      FindingSeverity `json:"severity"`
	RequirementID string          `json:"requirement_id,omitempty"`
	EvidenceIDs   []string        `json:"evidence_ids,omitempty"`
	Message       string          `json:"message"`
	Blocking      bool            `json:"blocking"`
}
