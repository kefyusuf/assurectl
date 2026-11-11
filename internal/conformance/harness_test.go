package conformance

import (
	"testing"
)

const conformanceDir = "../../conformance"

func TestEvidenceEnvelopeVectors(t *testing.T) {
	t.Parallel()

	vectors := LoadVectors(t, conformanceDir+"/evidence-envelope")
	if len(vectors) == 0 {
		t.Fatal("no evidence-envelope vectors found")
	}
	for _, v := range vectors {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			t.Parallel()

			err := ValidateEvidenceEnvelope(v.Vector.Document)
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
		})
	}
}

func TestUnknownExpectationIsRejected(t *testing.T) {
	t.Parallel()

	vectors := LoadVectors(t, conformanceDir+"/evidence-envelope")
	for _, v := range vectors {
		if v.Vector.Expect != "accept" && v.Vector.Expect != "reject" {
			t.Errorf("vector %s has unknown expectation %q", v.Name, v.Vector.Expect)
		}
	}
}
