package inputmeta_test

import (
	"encoding/json"
	"testing"

	"github.com/kefyusuf/assurectl/internal/inputmeta"
)

func TestTrustStatusVocabulary(t *testing.T) {
	t.Parallel()

	if got := string(inputmeta.TrustStatusUntrusted); got != "UNTRUSTED" {
		t.Fatalf("TrustStatusUntrusted = %q", got)
	}
	if got := string(inputmeta.TrustStatusTrusted); got != "TRUSTED" {
		t.Fatalf("TrustStatusTrusted = %q, want TRUSTED (ADR-0009)", got)
	}
}

func TestAuthorityBasisVocabulary(t *testing.T) {
	t.Parallel()

	if got := string(inputmeta.AuthorityBasisAdvisoryWorkspace); got != "ADVISORY_WORKSPACE" {
		t.Fatalf("AuthorityBasisAdvisoryWorkspace = %q", got)
	}
	if got := string(inputmeta.AuthorityBasisTrustedBase); got != "TRUSTED_BASE" {
		t.Fatalf("AuthorityBasisTrustedBase = %q, want TRUSTED_BASE (ADR-0009)", got)
	}
}

func TestDigestJSONShape(t *testing.T) {
	t.Parallel()

	digest := inputmeta.SHA256([]byte("payload"))
	data, err := json.Marshal(digest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"algorithm":"sha256","value":"`+digest.Value+`"}` {
		t.Fatalf("digest JSON = %s", data)
	}
}
