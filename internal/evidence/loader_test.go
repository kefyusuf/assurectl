package evidence

import (
	"os"
	"path/filepath"
	"strings"
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

func TestDecodeEnvelopeRejectsMalformedV0Input(t *testing.T) {
	t.Parallel()

	replace := func(old, replacement string) string {
		t.Helper()
		if !strings.Contains(validEnvelopeJSON, old) {
			t.Fatalf("fixture does not contain %q", old)
		}
		return strings.Replace(validEnvelopeJSON, old, replacement, 1)
	}

	tests := []struct {
		name string
		json string
	}{
		{name: "unsupported schema version", json: replace("\"schema_version\": \"assurectl/evidence-envelope/v0\"", "\"schema_version\": \"assurectl/evidence-envelope/v9\"")},
		{name: "self asserted trust field", json: replace("\"id\": \"ev-unit-tests-001\"", "\"trust_status\": \"TRUSTED\", \"id\": \"ev-unit-tests-001\"")},
		{name: "duplicate json key", json: replace("\"id\": \"ev-unit-tests-001\"", "\"id\": \"ev-unit-tests-001\", \"id\": \"ev-unit-tests-001\"")},
		{name: "case alias field", json: replace("\"id\": \"ev-unit-tests-001\"", "\"Id\": \"ev-unit-tests-001\"")},
		{name: "invalid evidence id", json: replace("\"id\": \"ev-unit-tests-001\"", "\"id\": \"../bad\"")},
		{name: "empty evidence type", json: replace("\"type\": \"test-result\"", "\"type\": \"\"")},
		{name: "overlong producer identity", json: replace("\"identity\": \"example-developer\"", "\"identity\": \""+strings.Repeat("x", 1025)+"\"")},
		{name: "invalid subject revision", json: replace("\"revision\": \"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\"", "\"revision\": \"ABC\"")},
		{name: "invalid environment digest algorithm", json: replace("\"algorithm\": \"sha256\"", "\"algorithm\": \"sha512\"")},
		{name: "invalid artifact digest hex", json: replace("\"value\": \"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee\"", "\"value\": \"xyz\"")},
		{name: "unsupported outcome", json: replace("\"status\": \"PASSED\"", "\"status\": \"SKIPPED\"")},
		{name: "impossible timestamp", json: replace("\"started_at\": \"2026-09-02T09:10:00Z\"", "\"started_at\": \"2026-02-30T09:10:00Z\"")},
		{name: "finished before started", json: replace("\"finished_at\": \"2026-09-02T09:10:01Z\"", "\"finished_at\": \"2026-09-02T09:09:59Z\"")},
		{name: "empty optional workflow", json: replace("\"identity\": \"example-developer\"", "\"identity\": \"example-developer\", \"workflow\": \"\"")},
		{name: "null optional workflow", json: replace("\"identity\": \"example-developer\"", "\"identity\": \"example-developer\", \"workflow\": null")},
		{name: "empty optional workflow revision", json: replace("\"identity\": \"example-developer\"", "\"identity\": \"example-developer\", \"workflow_revision\": \"\"")},
		{name: "null optional workflow revision", json: replace("\"identity\": \"example-developer\"", "\"identity\": \"example-developer\", \"workflow_revision\": null")},
		{
			name: "null optional environment digest",
			json: replace(
				"\"environment_digest\": {\n      \"algorithm\": \"sha256\",\n      \"value\": \"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd\"\n    }",
				"\"environment_digest\": null",
			),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got, err := decodeEnvelope([]byte(tt.json)); err == nil {
				t.Fatalf("decodeEnvelope() = %#v, want error", got)
			}
		})
	}
}

func TestDecodeEnvelopeAcceptsEqualInvocationTimestamps(t *testing.T) {
	t.Parallel()

	json := strings.Replace(
		validEnvelopeJSON,
		"\"finished_at\": \"2026-09-02T09:10:01Z\"",
		"\"finished_at\": \"2026-09-02T09:10:00Z\"",
		1,
	)
	if _, err := decodeEnvelope([]byte(json)); err != nil {
		t.Fatalf("decodeEnvelope() error = %v", err)
	}
}

func TestDecodeEnvelopeNormalizesJSONSchemaIntegerExitCode(t *testing.T) {
	t.Parallel()

	withExitCode := func(value string) string {
		t.Helper()
		return strings.Replace(validEnvelopeJSON, "\"exit_code\": 0", "\"exit_code\": "+value, 1)
	}

	tests := []struct {
		name string
		json string
		want string
	}{
		{name: "plain integer", json: withExitCode("1000"), want: "1000"},
		{name: "decimal integer", json: withExitCode("1000.0"), want: "1000"},
		{name: "exponent integer", json: withExitCode("1e3"), want: "1000"},
		{name: "negative integer", json: withExitCode("-1"), want: "-1"},
		{name: "beyond int64", json: withExitCode("9223372036854775808"), want: "9223372036854775808"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := decodeEnvelope([]byte(tt.json))
			if err != nil {
				t.Fatalf("decodeEnvelope() error = %v", err)
			}
			if got.Envelope.Outcome.ExitCode == nil || got.Envelope.Outcome.ExitCode.String() != tt.want {
				t.Fatalf("ExitCode = %#v, want %s", got.Envelope.Outcome.ExitCode, tt.want)
			}
		})
	}

	t.Run("fractional number rejected", func(t *testing.T) {
		t.Parallel()
		if got, err := decodeEnvelope([]byte(withExitCode("1.5"))); err == nil {
			t.Fatalf("decodeEnvelope() = %#v, want error", got)
		}
	})

	t.Run("explicit null rejected", func(t *testing.T) {
		t.Parallel()
		if got, err := decodeEnvelope([]byte(withExitCode("null"))); err == nil {
			t.Fatalf("decodeEnvelope() = %#v, want error", got)
		}
	})

	t.Run("omitted field remains nil", func(t *testing.T) {
		t.Parallel()
		json := strings.Replace(validEnvelopeJSON, ",\n    \"exit_code\": 0", "", 1)
		got, err := decodeEnvelope([]byte(json))
		if err != nil {
			t.Fatalf("decodeEnvelope() error = %v", err)
		}
		if got.Envelope.Outcome.ExitCode != nil {
			t.Fatalf("ExitCode = %#v, want nil", got.Envelope.Outcome.ExitCode)
		}
	})
}

func TestEnvelopeDigestNormalizesEquivalentIntegerForms(t *testing.T) {
	t.Parallel()

	plainJSON := strings.Replace(validEnvelopeJSON, "\"exit_code\": 0", "\"exit_code\": 1000", 1)
	exponentJSON := strings.Replace(validEnvelopeJSON, "\"exit_code\": 0", "\"exit_code\": 1e3", 1)

	plain, err := decodeEnvelope([]byte(plainJSON))
	if err != nil {
		t.Fatalf("decodeEnvelope(plain) error = %v", err)
	}
	exponent, err := decodeEnvelope([]byte(exponentJSON))
	if err != nil {
		t.Fatalf("decodeEnvelope(exponent) error = %v", err)
	}
	if plain.Digest != exponent.Digest {
		t.Fatalf("equivalent integer digests differ: %#v != %#v", plain.Digest, exponent.Digest)
	}
}


func TestDiscoverLocal(t *testing.T) {
	t.Parallel()

	t.Run("missing evidence directory is empty", func(t *testing.T) {
		t.Parallel()

		got, err := discoverLocal(t.TempDir())
		if err != nil {
			t.Fatalf("discoverLocal() error = %v", err)
		}
		if len(got.Candidates) != 0 {
			t.Fatalf("Candidates = %#v, want empty", got.Candidates)
		}
	})

	t.Run("discovers only direct regular json files in canonical order", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		evidenceRoot := filepath.Join(root, ".assurectl", "evidence")
		if err := os.MkdirAll(filepath.Join(evidenceRoot, "artifacts"), 0o755); err != nil {
			t.Fatal(err)
		}
		for name, data := range map[string]string{
			"z.json":                    "{}",
			"a.json":                    "{}",
			"ignore.txt":                "ignored",
			"artifacts/nested.json":     "{}",
		} {
			path := filepath.Join(evidenceRoot, filepath.FromSlash(name))
			if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Mkdir(filepath.Join(evidenceRoot, "directory.json"), 0o755); err != nil {
			t.Fatal(err)
		}

		got, err := discoverLocal(root)
		if err != nil {
			t.Fatalf("discoverLocal() error = %v", err)
		}
		resolvedEvidenceRoot, err := filepath.EvalSymlinks(evidenceRoot)
		if err != nil {
			t.Fatal(err)
		}
		if got.EvidenceRoot != resolvedEvidenceRoot {
			t.Fatalf("EvidenceRoot = %q, want %q", got.EvidenceRoot, resolvedEvidenceRoot)
		}
		if len(got.Candidates) != 2 {
			t.Fatalf("Candidates = %#v, want 2 entries", got.Candidates)
		}

		wantPaths := []string{
			".assurectl/evidence/a.json",
			".assurectl/evidence/z.json",
		}
		wantSources := []string{
			"workspace:.assurectl/evidence/a.json",
			"workspace:.assurectl/evidence/z.json",
		}
		for i := range wantPaths {
			if got.Candidates[i].RelativePath != wantPaths[i] {
				t.Fatalf("Candidates[%d].RelativePath = %q, want %q", i, got.Candidates[i].RelativePath, wantPaths[i])
			}
			if got.Candidates[i].Source != wantSources[i] {
				t.Fatalf("Candidates[%d].Source = %q, want %q", i, got.Candidates[i].Source, wantSources[i])
			}
			if string(got.Candidates[i].Data) != "{}" {
				t.Fatalf("Candidates[%d].Data = %q, want {}", i, got.Candidates[i].Data)
			}
		}
	})
}
