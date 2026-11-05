package inputmeta

import (
	"crypto/sha256"
	"encoding/hex"
)

const DigestAlgorithmSHA256 = "sha256"

type Digest struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}

type TrustStatus string

const (
	TrustStatusUntrusted TrustStatus = "UNTRUSTED"
	// TrustStatusTrusted marks inputs whose authority comes from their
	// resolved source context (ADR-0009: the subject's base revision),
	// never from a self-asserted field.
	TrustStatusTrusted TrustStatus = "TRUSTED"
)

type AuthorityBasis string

const (
	AuthorityBasisAdvisoryWorkspace AuthorityBasis = "ADVISORY_WORKSPACE"
	// AuthorityBasisTrustedBase marks inputs read from the subject's base
	// revision through the vetted Git path.
	AuthorityBasisTrustedBase AuthorityBasis = "TRUSTED_BASE"
)

func SHA256(data []byte) Digest {
	sum := sha256.Sum256(data)
	return Digest{
		Algorithm: DigestAlgorithmSHA256,
		Value:     hex.EncodeToString(sum[:]),
	}
}
