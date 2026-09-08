package domain

type WaiverStatus string

const (
	WaiverNotApplicable WaiverStatus = "NOT_APPLICABLE"
	WaiverValid         WaiverStatus = "VALID"
	WaiverInvalid       WaiverStatus = "INVALID"
)

func (v WaiverStatus) Valid() bool {
	switch v {
	case WaiverNotApplicable, WaiverValid, WaiverInvalid:
		return true
	default:
		return false
	}
}
