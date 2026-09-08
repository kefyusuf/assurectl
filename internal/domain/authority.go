package domain

type AuthorityTier string

const (
	AuthorityAdvisory      AuthorityTier = "ADVISORY"
	AuthorityCIAsserted    AuthorityTier = "CI_ASSERTED"
	AuthorityAuthoritative AuthorityTier = "AUTHORITATIVE"
)

func (v AuthorityTier) Valid() bool {
	switch v {
	case AuthorityAdvisory, AuthorityCIAsserted, AuthorityAuthoritative:
		return true
	default:
		return false
	}
}
