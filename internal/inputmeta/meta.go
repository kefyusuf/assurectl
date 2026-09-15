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

const TrustStatusUntrusted TrustStatus = "UNTRUSTED"

type AuthorityBasis string

const AuthorityBasisAdvisoryWorkspace AuthorityBasis = "ADVISORY_WORKSPACE"

func SHA256(data []byte) Digest {
	sum := sha256.Sum256(data)
	return Digest{
		Algorithm: DigestAlgorithmSHA256,
		Value:     hex.EncodeToString(sum[:]),
	}
}
