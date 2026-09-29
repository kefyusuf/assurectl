package evidence

import (
	"testing"
	"time"

	"github.com/kefyusuf/assurectl/internal/domain"
)

const validEnvelopeJSON = `{
  "schema_version": "assurectl/evidence-envelope/v0",
  "id": "ev-unit-tests-001",
  "type": "test-result",
  "subject": {
    "repository_uri": "github.com/acme/checkout",
    "revision": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
  },
  "producer": {
    "type": "local-user",
    "identity": "example-developer"
  },
  "invocation": {
    "command_id": "test.unit",
    "environment_digest": {
      "algorithm": "sha256",
      "value": "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
    },
    "started_at": "2026-09-02T09:10:00Z",
    "finished_at": "2026-09-02T09:10:01Z"
  },
  "outcome": {
    "status": "PASSED",
    "exit_code": 0
  },
  "artifact": {
    "uri": "artifacts/unit-tests.json",
    "digest": {
      "algorithm": "sha256",
      "value": "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
    },
    "media_type": "application/json"
  }
}`

func TestDecodeEnvelopeReturnsTypedValueTimesAndCanonicalDigest(t *testing.T) {
	t.Parallel()

	first, err := decodeEnvelope([]byte(validEnvelopeJSON))
	if err != nil {
		t.Fatalf("decodeEnvelope() error = %v", err)
	}

	secondJSON := `{"artifact":{"media_type":"application/json","digest":{"value":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee","algorithm":"sha256"},"uri":"artifacts/unit-tests.json"},"outcome":{"exit_code":0,"status":"PASSED"},"invocation":{"finished_at":"2026-09-02T09:10:01Z","started_at":"2026-09-02T09:10:00Z","environment_digest":{"value":"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd","algorithm":"sha256"},"command_id":"test.unit"},"producer":{"identity":"example-developer","type":"local-user"},"subject":{"revision":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","repository_uri":"github.com/acme/checkout"},"type":"test-result","id":"ev-unit-tests-001","schema_version":"assurectl/evidence-envelope/v0"}`
	second, err := decodeEnvelope([]byte(secondJSON))
	if err != nil {
		t.Fatalf("decodeEnvelope(reordered) error = %v", err)
	}

	if first.Envelope.SchemaVersion != SchemaVersion {
		t.Fatalf("SchemaVersion = %q", first.Envelope.SchemaVersion)
	}
	if first.Envelope.ID != "ev-unit-tests-001" || first.Envelope.Type != "test-result" {
		t.Fatalf("Envelope identity = %#v", first.Envelope)
	}
	if first.Envelope.Subject.RepositoryURI != "github.com/acme/checkout" {
		t.Fatalf("Subject.RepositoryURI = %q", first.Envelope.Subject.RepositoryURI)
	}
	if first.Envelope.Subject.Revision != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Fatalf("Subject.Revision = %q", first.Envelope.Subject.Revision)
	}
	if first.Envelope.Producer.Type != "local-user" || first.Envelope.Producer.Identity != "example-developer" {
		t.Fatalf("Producer = %#v", first.Envelope.Producer)
	}
	if first.Envelope.Invocation.EnvironmentDigest == nil || first.Envelope.Invocation.EnvironmentDigest.Algorithm != "sha256" {
		t.Fatalf("EnvironmentDigest = %#v", first.Envelope.Invocation.EnvironmentDigest)
	}
	if first.Envelope.Outcome.Status != domain.OutcomePassed {
		t.Fatalf("Outcome.Status = %q", first.Envelope.Outcome.Status)
	}
	if first.Envelope.Outcome.ExitCode == nil || first.Envelope.Outcome.ExitCode.String() != "0" {
		t.Fatalf("Outcome.ExitCode = %#v", first.Envelope.Outcome.ExitCode)
	}
	if first.Envelope.Artifact.Digest.Algorithm != "sha256" {
		t.Fatalf("Artifact.Digest = %#v", first.Envelope.Artifact.Digest)
	}

	wantStarted := time.Date(2026, 9, 2, 9, 10, 0, 0, time.UTC)
	wantFinished := time.Date(2026, 9, 2, 9, 10, 1, 0, time.UTC)
	if !first.StartedAt.Equal(wantStarted) || !first.FinishedAt.Equal(wantFinished) {
		t.Fatalf("parsed times = %s..%s", first.StartedAt, first.FinishedAt)
	}
	if first.Digest.Algorithm != "sha256" || first.Digest.Value == "" {
		t.Fatalf("Digest = %#v", first.Digest)
	}
	if first.Digest != second.Digest {
		t.Fatalf("canonical digests differ: %#v != %#v", first.Digest, second.Digest)
	}
}
