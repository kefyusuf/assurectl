package buildinfo

import "testing"

func TestSummary(t *testing.T) {
	oldVersion, oldCommit, oldDate := Version, Commit, Date
	t.Cleanup(func() {
		Version, Commit, Date = oldVersion, oldCommit, oldDate
	})

	Version = "v0.0.0-test"
	Commit = "abc123"
	Date = "2026-09-02T00:00:00Z"

	got := Summary()
	want := "assurectl v0.0.0-test (commit abc123, built 2026-09-02T00:00:00Z)"
	if got != want {
		t.Fatalf("Summary() = %q, want %q", got, want)
	}
}
