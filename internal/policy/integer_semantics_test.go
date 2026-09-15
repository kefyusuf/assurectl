package policy

import "testing"

func TestLoadLocalAcceptsJSONSchemaIntegerFormsAndNormalizesDigest(t *testing.T) {
	rootDecimal := t.TempDir()
	rootExponent := t.TempDir()

	writePolicyFile(t, rootDecimal, `{"schema_version":"assurectl/policy/v0","requirements":[{"id":"unit-tests","evidence_type":"test-result","waivable":false,"allowed_producer_types":["local-user"],"max_age_seconds":1000}]}`)
	writePolicyFile(t, rootExponent, `{"schema_version":"assurectl/policy/v0","requirements":[{"id":"unit-tests","evidence_type":"test-result","waivable":false,"allowed_producer_types":["local-user"],"max_age_seconds":1e3}]}`)

	decimal, err := LoadLocal(rootDecimal)
	if err != nil {
		t.Fatalf("LoadLocal(decimal) error = %v", err)
	}
	exponent, err := LoadLocal(rootExponent)
	if err != nil {
		t.Fatalf("LoadLocal(exponent) error = %v", err)
	}
	if decimal.Digest != exponent.Digest {
		t.Fatalf("equivalent integer digests differ: %#v != %#v", decimal.Digest, exponent.Digest)
	}
}

func TestLoadLocalAcceptsJSONSchemaIntegerBeyondUint64(t *testing.T) {
	root := t.TempDir()
	writePolicyFile(t, root, `{"schema_version":"assurectl/policy/v0","requirements":[{"id":"unit-tests","evidence_type":"test-result","waivable":false,"allowed_producer_types":["local-user"],"max_age_seconds":18446744073709551616}]}`)

	if _, err := LoadLocal(root); err != nil {
		t.Fatalf("LoadLocal() error = %v", err)
	}
}
