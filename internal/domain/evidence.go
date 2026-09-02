package domain

type EvidenceState string

const (
	EvidenceValid     EvidenceState = "VALID"
	EvidenceInvalid   EvidenceState = "INVALID"
	EvidenceMissing   EvidenceState = "MISSING"
	EvidenceStale     EvidenceState = "STALE"
	EvidenceUntrusted EvidenceState = "UNTRUSTED"
)

func (v EvidenceState) Valid() bool {
	switch v {
	case EvidenceValid, EvidenceInvalid, EvidenceMissing, EvidenceStale, EvidenceUntrusted:
		return true
	default:
		return false
	}
}

type ObservedOutcome string

const (
	OutcomePassed ObservedOutcome = "PASSED"
	OutcomeFailed ObservedOutcome = "FAILED"
	OutcomeError  ObservedOutcome = "ERROR"
)

func (v ObservedOutcome) Valid() bool {
	switch v {
	case OutcomePassed, OutcomeFailed, OutcomeError:
		return true
	default:
		return false
	}
}
