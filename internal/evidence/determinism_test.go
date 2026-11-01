package evidence

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/gitsubject"
	"github.com/kefyusuf/assurectl/internal/inputmeta"
)

// TestEnvelopeDigestDeterminism pins the distinction between the typed
// supplied-input digest and the comparison facts derived from the same
// envelope: equivalent JSON spellings collapse to one digest, while supplied
// spellings that are semantically equivalent (timestamps, repository URIs)
// intentionally produce different digests without changing the parsed facts.
func TestEnvelopeDigestDeterminism(t *testing.T) {
	t.Parallel()

	base, err := decodeEnvelope([]byte(validEnvelopeJSON))
	if err != nil {
		t.Fatalf("decodeEnvelope(base) error = %v", err)
	}

	t.Run("whitespace and object-key order do not change the digest", func(t *testing.T) {
		t.Parallel()

		var compactBuffer bytes.Buffer
		if err := json.Compact(&compactBuffer, []byte(validEnvelopeJSON)); err != nil {
			t.Fatal(err)
		}
		compacted, err := decodeEnvelope(compactBuffer.Bytes())
		if err != nil {
			t.Fatalf("decodeEnvelope(compacted) error = %v", err)
		}

		var reorderedAny map[string]any
		if err := json.Unmarshal([]byte(validEnvelopeJSON), &reorderedAny); err != nil {
			t.Fatal(err)
		}
		reordered, err := decodeEnvelope(mustMarshal(t, reorderedAny))
		if err != nil {
			t.Fatalf("decodeEnvelope(reordered) error = %v", err)
		}

		if base.Digest != compacted.Digest || base.Digest != reordered.Digest {
			t.Fatalf("equivalent spellings produced different digests: base=%#v compacted=%#v reordered=%#v", base.Digest, compacted.Digest, reordered.Digest)
		}
	})

	t.Run("timestamp spelling changes the digest but not the parsed instant", func(t *testing.T) {
		t.Parallel()

		respelled := strings.Replace(
			validEnvelopeJSON,
			"\"started_at\": \"2026-09-02T09:10:00Z\"",
			"\"started_at\": \"2026-09-02T12:10:00+03:00\"",
			1,
		)
		decoded, err := decodeEnvelope([]byte(respelled))
		if err != nil {
			t.Fatalf("decodeEnvelope(respelled) error = %v", err)
		}

		if base.Digest == decoded.Digest {
			t.Fatalf("different supplied timestamp spellings must produce different digests, both were %#v", base.Digest)
		}
		if !base.StartedAt.Equal(decoded.StartedAt) {
			t.Fatalf("parsed instants differ: base=%s respelled=%s", base.StartedAt, decoded.StartedAt)
		}
		if want := (time.Time{}); decoded.StartedAt.Equal(want) {
			t.Fatalf("parsed instant is zero: %s", decoded.StartedAt)
		}
	})

	t.Run("repository uri spelling changes the digest but not the canonical identity", func(t *testing.T) {
		t.Parallel()

		respelled := strings.Replace(
			validEnvelopeJSON,
			"\"repository_uri\": \"github.com/acme/checkout\"",
			"\"repository_uri\": \"https://github.com/acme/checkout.git\"",
			1,
		)
		decoded, err := decodeEnvelope([]byte(respelled))
		if err != nil {
			t.Fatalf("decodeEnvelope(respelled) error = %v", err)
		}

		if base.Digest == decoded.Digest {
			t.Fatalf("different supplied repository spellings must produce different digests, both were %#v", base.Digest)
		}
		baseCanonical, err := gitsubject.CanonicalizeRepositoryIdentity(base.Envelope.Subject.RepositoryURI)
		if err != nil {
			t.Fatalf("CanonicalizeRepositoryIdentity(base) error = %v", err)
		}
		respelledCanonical, err := gitsubject.CanonicalizeRepositoryIdentity(decoded.Envelope.Subject.RepositoryURI)
		if err != nil {
			t.Fatalf("CanonicalizeRepositoryIdentity(respelled) error = %v", err)
		}
		if baseCanonical != respelledCanonical {
			t.Fatalf("canonical identities differ: %q != %q", baseCanonical, respelledCanonical)
		}
	})
}

// TestLoadLocalDeterministic pins that local ingestion is a pure function of
// the filesystem state and the resolved subject: repeated loads are deeply
// equal and ordered, and repository spellings that canonicalize identically
// bind exactly without changing the digest of their own envelope.
func TestLoadLocalDeterministic(t *testing.T) {
	t.Parallel()

	head := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	repository := "github.com/acme/checkout"

	t.Run("repeated loads of identical filesystem state are deeply equal", func(t *testing.T) {
		t.Parallel()

		root := writeDeterminismWorkspace(t, map[string]determinismEnvelope{
			"ev-a": {
				RepositoryURI: repository,
				Revision:      head,
				Outcome:       domain.OutcomeFailed,
				ArtifactData:  []byte("artifact-a"),
			},
			"ev-z": {
				RepositoryURI: repository,
				Revision:      head,
				Outcome:       domain.OutcomePassed,
				ArtifactData:  []byte("artifact-z"),
			},
		})

		first, err := LoadLocal(root, domain.Subject{RepositoryURI: repository, HeadRevision: head})
		if err != nil {
			t.Fatalf("LoadLocal(first) error = %v", err)
		}
		second, err := LoadLocal(root, domain.Subject{RepositoryURI: repository, HeadRevision: head})
		if err != nil {
			t.Fatalf("LoadLocal(second) error = %v", err)
		}

		if len(first) != 2 {
			t.Fatalf("LoadLocal() returned %d entries, want 2", len(first))
		}
		if first[0].Source != "workspace:.assurectl/evidence/ev-a.json" || first[1].Source != "workspace:.assurectl/evidence/ev-z.json" {
			t.Fatalf("load order is not deterministic: %q, %q", first[0].Source, first[1].Source)
		}
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("repeated loads differ:\nfirst:  %#v\nsecond: %#v", first, second)
		}
	})

	t.Run("repository uri spelling binds exact", func(t *testing.T) {
		t.Parallel()

		root := writeDeterminismWorkspace(t, map[string]determinismEnvelope{
			"ev-canonical": {
				RepositoryURI: repository,
				Revision:      head,
				Outcome:       domain.OutcomePassed,
				ArtifactData:  []byte("artifact-canonical"),
			},
			"ev-spelled": {
				RepositoryURI: "https://github.com/acme/checkout.git",
				Revision:      head,
				Outcome:       domain.OutcomePassed,
				ArtifactData:  []byte("artifact-spelled"),
			},
		})

		loaded, err := LoadLocal(root, domain.Subject{RepositoryURI: repository, HeadRevision: head})
		if err != nil {
			t.Fatalf("LoadLocal() error = %v", err)
		}
		if len(loaded) != 2 {
			t.Fatalf("LoadLocal() returned %d entries, want 2", len(loaded))
		}

		for _, item := range loaded {
			if item.SubjectBinding != SubjectBindingExact {
				t.Fatalf("%s binding = %q, want EXACT", item.Envelope.ID, item.SubjectBinding)
			}
			if item.Digest == (inputmeta.Digest{}) {
				t.Fatalf("%s digest is empty", item.Envelope.ID)
			}
		}
		if loaded[0].Digest == loaded[1].Digest {
			t.Fatalf("envelopes with different supplied repository spellings must have different digests, both were %#v", loaded[0].Digest)
		}
	})
}

type determinismEnvelope struct {
	RepositoryURI string
	Revision      string
	Outcome       domain.ObservedOutcome
	ArtifactData  []byte
}

// writeDeterminismWorkspace materializes one evidence file per envelope,
// ordered by id so the derived file names (a.json, z.json, ...) and artifact
// paths stay unique and deterministic across subtests.
func writeDeterminismWorkspace(t *testing.T, envelopes map[string]determinismEnvelope) string {
	t.Helper()

	root := t.TempDir()
	evidenceRoot := filepath.Join(root, ".assurectl", "evidence")
	if err := os.MkdirAll(evidenceRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, id := range sortedKeys(envelopes) {
		envelope := envelopes[id]
		artifactName := "artifacts/" + id + ".bin"
		if err := os.MkdirAll(filepath.Join(evidenceRoot, "artifacts"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(evidenceRoot, filepath.FromSlash(artifactName)), envelope.ArtifactData, 0o644); err != nil {
			t.Fatal(err)
		}
		envelopeJSON := loadLocalEnvelopeJSON(
			id,
			envelope.RepositoryURI,
			envelope.Revision,
			envelope.Outcome,
			artifactName,
			inputmeta.SHA256(envelope.ArtifactData),
			"2026-09-02T09:10:00Z",
			"2026-09-02T09:10:01Z",
		)
		if err := os.WriteFile(filepath.Join(evidenceRoot, id+".json"), []byte(envelopeJSON), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func sortedKeys(envelopes map[string]determinismEnvelope) []string {
	keys := make([]string, 0, len(envelopes))
	for key := range envelopes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func mustMarshal(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
