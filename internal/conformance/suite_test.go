package conformance

import (
	"testing"
)

func TestReceiptVectors(t *testing.T) {
	t.Parallel()

	vectors := LoadVectors(t, conformanceDir+"/receipt")
	if len(vectors) == 0 {
		t.Fatal("no receipt vectors found")
	}
	for _, v := range vectors {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			t.Parallel()

			err := ValidateReceipt(v.Vector.Document)
			assertVectorExpectation(t, v, err)
		})
	}
}

func TestTrustedRootVectors(t *testing.T) {
	t.Parallel()

	vectors := LoadVectors(t, conformanceDir+"/trusted-root")
	if len(vectors) == 0 {
		t.Fatal("no trusted-root vectors found")
	}
	for _, v := range vectors {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			t.Parallel()

			err := ValidateTrustedRoot(v.Vector.Document)
			assertVectorExpectation(t, v, err)
		})
	}
}

func TestCheckPayloadVectors(t *testing.T) {
	t.Parallel()

	vectors := LoadVectors(t, conformanceDir+"/check-payload")
	if len(vectors) == 0 {
		t.Fatal("no check-payload vectors found")
	}
	for _, v := range vectors {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			t.Parallel()

			err := ValidateCheckPayload(v.Vector.Document)
			assertVectorExpectation(t, v, err)
		})
	}
}

func assertVectorExpectation(t *testing.T, v NamedVector, err error) {
	t.Helper()
	switch v.Vector.Expect {
	case "accept":
		if err != nil {
			t.Fatalf("expected accept, got reject: %v", err)
		}
	case "reject":
		if err == nil {
			t.Fatalf("expected reject, got accept")
		}
	default:
		t.Fatalf("unknown expectation %q", v.Vector.Expect)
	}
}
