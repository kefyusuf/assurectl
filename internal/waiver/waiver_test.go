package waiver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/inputmeta"
	"github.com/kefyusuf/assurectl/internal/localinput"
)

func validRecord() Record {
	return Record{
		ID:           "wv-flake-001",
		Target:       Target{Kind: "REQUIREMENT", ID: "req-a"},
		Scope:        Scope{RepositoryURI: "github.com/acme/checkout", WorkUnitID: "wu-001"},
		Reason:       "known flaky test",
		AcceptedRisk: "retry before merge",
		Approver:     Approver{Identity: "release-engineer", Authority: "RELEASE_MANAGER"},
		IssuedAt:     "2026-11-01T09:00:00Z",
		ExpiresAt:    "2026-12-01T09:00:00Z",
		Status:       domain.WaiverValid,
	}
}

func writeFile(t *testing.T, root, relative, content string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return relative
}

func recordFile(t *testing.T, root string, records ...Record) string {
	t.Helper()
	set := waiversFile{SchemaVersion: SchemaVersion}
	for _, record := range records {
		if record.Digest == (inputmeta.Digest{}) {
			record.Digest = CanonicalDigest(record)
		}
		set.Waivers = append(set.Waivers, record)
	}
	data, err := json.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	return writeFile(t, root, "waivers.json", string(data))
}

func TestLoadLocalReadsValidRecords(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	relative := recordFile(t, root, validRecord())

	loaded, err := LoadLocal(root, relative)
	if err != nil {
		t.Fatalf("LoadLocal() error = %v", err)
	}
	if len(loaded.Records) != 1 {
		t.Fatalf("Records = %d entries, want 1", len(loaded.Records))
	}
	record := loaded.Records[0]
	if record.ID != "wv-flake-001" || record.Target.ID != "req-a" {
		t.Fatalf("Record = %#v", record)
	}
	if record.Digest.Algorithm != inputmeta.DigestAlgorithmSHA256 || len(record.Digest.Value) != 64 {
		t.Fatalf("Digest = %#v", record.Digest)
	}
	if record.Digest != CanonicalDigest(validRecord()) {
		t.Fatalf("Digest is not the canonical record digest")
	}
	if !strings.Contains(loaded.Source, "waivers.json") {
		t.Fatalf("Source = %q", loaded.Source)
	}
}

func TestLoadLocalMissingFileIsEmptySet(t *testing.T) {
	t.Parallel()

	loaded, err := LoadLocal(t.TempDir(), "waivers.json")
	if err != nil {
		t.Fatalf("LoadLocal() error = %v", err)
	}
	if len(loaded.Records) != 0 {
		t.Fatalf("Records = %#v, want empty", loaded.Records)
	}
}

func TestLoadLocalRejectsMalformedRecords(t *testing.T) {
	t.Parallel()

	mutate := func(mutations ...func(*Record)) Record {
		record := validRecord()
		for _, apply := range mutations {
			apply(&record)
		}
		return record
	}

	tests := []struct {
		name    string
		records []Record
	}{
		{
			name:    "finding target kind reserved",
			records: []Record{mutate(func(r *Record) { r.Target.Kind = "FINDING" })},
		},
		{
			name:    "duplicate ids",
			records: []Record{validRecord(), validRecord()},
		},
		{
			name:    "non canonical id",
			records: []Record{mutate(func(r *Record) { r.ID = "bad id" })},
		},
		{
			name:    "expiry before issuance",
			records: []Record{mutate(func(r *Record) { r.IssuedAt, r.ExpiresAt = r.ExpiresAt, r.IssuedAt })},
		},
		{
			name:    "unparseable issued at",
			records: []Record{mutate(func(r *Record) { r.IssuedAt = "not-a-time" })},
		},
		{
			name:    "unsupported status",
			records: []Record{mutate(func(r *Record) { r.Status = domain.WaiverNotApplicable })},
		},
		{
			name: "digest mismatch",
			records: []Record{mutate(func(r *Record) {
				r.Digest = CanonicalDigest(validRecord())
				r.AcceptedRisk = "tampered after digesting"
			})},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			relative := recordFile(t, root, tt.records...)
			if _, err := LoadLocal(root, relative); err == nil {
				t.Fatalf("LoadLocal() accepted malformed records")
			}
		})
	}
}

func TestLoadLocalOversizeFileRejected(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	huge := `{"schema_version": "assurectl/waivers/v0", "waivers": []}` + strings.Repeat(" ", int(localinput.MaxWorkspaceInputBytes))
	relative := writeFile(t, root, "waivers.json", huge)
	if _, err := LoadLocal(root, relative); err == nil {
		t.Fatalf("LoadLocal() accepted an oversized file")
	}
}

func TestCanonicalDigestIsFormattingIndependent(t *testing.T) {
	t.Parallel()

	compact, err := json.Marshal(validRecord())
	if err != nil {
		t.Fatal(err)
	}
	spread := strings.Replace(string(compact), "{", "{ ", 1)
	spread = strings.Replace(spread, ",", ", ", 1)

	var parsed Record
	if err := json.Unmarshal([]byte(spread), &parsed); err != nil {
		t.Fatal(err)
	}
	if CanonicalDigest(parsed) != CanonicalDigest(validRecord()) {
		t.Fatalf("canonical digest changed with input formatting")
	}
}
