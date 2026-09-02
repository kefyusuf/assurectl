package domain

import "testing"

func TestEvidenceStateValid(t *testing.T) {
	t.Parallel()

	valid := []EvidenceState{
		EvidenceValid,
		EvidenceInvalid,
		EvidenceMissing,
		EvidenceStale,
		EvidenceUntrusted,
	}
	for _, value := range valid {
		value := value
		t.Run(string(value), func(t *testing.T) {
			t.Parallel()
			if !value.Valid() {
				t.Fatalf("EvidenceState(%q).Valid() = false, want true", value)
			}
		})
	}

	for _, value := range []EvidenceState{"", "valid", "UNKNOWN"} {
		if value.Valid() {
			t.Fatalf("EvidenceState(%q).Valid() = true, want false", value)
		}
	}
}

func TestObservedOutcomeValid(t *testing.T) {
	t.Parallel()

	for _, value := range []ObservedOutcome{OutcomePassed, OutcomeFailed, OutcomeError} {
		if !value.Valid() {
			t.Fatalf("ObservedOutcome(%q).Valid() = false, want true", value)
		}
	}

	for _, value := range []ObservedOutcome{"", "PASS", "UNKNOWN"} {
		if value.Valid() {
			t.Fatalf("ObservedOutcome(%q).Valid() = true, want false", value)
		}
	}
}
