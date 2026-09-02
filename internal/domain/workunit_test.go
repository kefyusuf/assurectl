package domain

import "testing"

func TestWorkUnitTypeValid(t *testing.T) {
	t.Parallel()

	valid := []WorkUnitType{
		WorkUnitPullRequest,
		WorkUnitCommit,
		WorkUnitReleaseCandidate,
		WorkUnitAPIMigration,
		WorkUnitSDKMigration,
		WorkUnitFrameworkUpgrade,
		WorkUnitDependencyUpgrade,
		WorkUnitSecurityRemediation,
	}
	for _, value := range valid {
		if !value.Valid() {
			t.Fatalf("WorkUnitType(%q).Valid() = false, want true", value)
		}
	}

	for _, value := range []WorkUnitType{"", "pull-request", "UNKNOWN"} {
		if value.Valid() {
			t.Fatalf("WorkUnitType(%q).Valid() = true, want false", value)
		}
	}
}
