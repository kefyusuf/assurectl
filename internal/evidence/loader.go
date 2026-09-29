package evidence

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"time"
	"unicode/utf8"

	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/inputmeta"
	"github.com/kefyusuf/assurectl/internal/strictjson"
)

var (
	evidenceIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	objectIDPattern   = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)
	digestPattern     = regexp.MustCompile(`^[0-9a-f]{64}$`)
	timestampPattern  = regexp.MustCompile(`^[0-9]{4}-(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])T([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9](.[0-9]+)?(Z|[+-]([01][0-9]|2[0-3]):[0-5][0-9])$`)
)

type decodedEnvelope struct {
	Envelope   Envelope
	StartedAt  time.Time
	FinishedAt time.Time
	Digest     inputmeta.Digest
}

type rawEnvelope struct {
	SchemaVersion string        `json:"schema_version"`
	ID            string        `json:"id"`
	Type          string        `json:"type"`
	Subject       Subject       `json:"subject"`
	Producer      rawProducer   `json:"producer"`
	Invocation    rawInvocation `json:"invocation"`
	Outcome       rawOutcome    `json:"outcome"`
	Artifact      Artifact      `json:"artifact"`
}

type rawProducer struct {
	Type             string          `json:"type"`
	Identity         string          `json:"identity"`
	Workflow         json.RawMessage `json:"workflow,omitempty"`
	WorkflowRevision json.RawMessage `json:"workflow_revision,omitempty"`
}

type rawInvocation struct {
	CommandID         string          `json:"command_id"`
	EnvironmentDigest json.RawMessage `json:"environment_digest,omitempty"`
	StartedAt         string          `json:"started_at"`
	FinishedAt        string          `json:"finished_at"`
}

type rawOutcome struct {
	Status   domain.ObservedOutcome `json:"status"`
	ExitCode json.RawMessage        `json:"exit_code,omitempty"`
}

func decodeEnvelope(data []byte) (decodedEnvelope, error) {
	var raw rawEnvelope
	if err := strictjson.Decode(data, &raw); err != nil {
		return decodedEnvelope{}, fmt.Errorf("decode evidence envelope: %w", err)
	}

	envelope, err := normalizeEnvelope(raw)
	if err != nil {
		return decodedEnvelope{}, err
	}
	if err := validateEnvelope(envelope); err != nil {
		return decodedEnvelope{}, err
	}

	startedAt, err := parseTimestamp("invocation.started_at", envelope.Invocation.StartedAt)
	if err != nil {
		return decodedEnvelope{}, err
	}
	finishedAt, err := parseTimestamp("invocation.finished_at", envelope.Invocation.FinishedAt)
	if err != nil {
		return decodedEnvelope{}, err
	}
	if finishedAt.Before(startedAt) {
		return decodedEnvelope{}, errors.New("invocation.finished_at precedes started_at")
	}

	canonical, err := json.Marshal(envelope)
	if err != nil {
		return decodedEnvelope{}, fmt.Errorf("canonicalize evidence envelope: %w", err)
	}

	return decodedEnvelope{
		Envelope:   envelope,
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		Digest:     inputmeta.SHA256(canonical),
	}, nil
}

func normalizeEnvelope(raw rawEnvelope) (Envelope, error) {
	workflow, err := optionalString("producer.workflow", raw.Producer.Workflow, 1024)
	if err != nil {
		return Envelope{}, err
	}
	workflowRevision, err := optionalObjectID("producer.workflow_revision", raw.Producer.WorkflowRevision)
	if err != nil {
		return Envelope{}, err
	}
	environmentDigest, err := optionalDigest("invocation.environment_digest", raw.Invocation.EnvironmentDigest)
	if err != nil {
		return Envelope{}, err
	}
	exitCode, err := optionalInteger("outcome.exit_code", raw.Outcome.ExitCode)
	if err != nil {
		return Envelope{}, err
	}

	return Envelope{
		SchemaVersion: raw.SchemaVersion,
		ID:            raw.ID,
		Type:          raw.Type,
		Subject:       raw.Subject,
		Producer: Producer{
			Type:             raw.Producer.Type,
			Identity:         raw.Producer.Identity,
			Workflow:         workflow,
			WorkflowRevision: workflowRevision,
		},
		Invocation: Invocation{
			CommandID:         raw.Invocation.CommandID,
			EnvironmentDigest: environmentDigest,
			StartedAt:         raw.Invocation.StartedAt,
			FinishedAt:        raw.Invocation.FinishedAt,
		},
		Outcome: Outcome{
			Status:   raw.Outcome.Status,
			ExitCode: exitCode,
		},
		Artifact: raw.Artifact,
	}, nil
}

func validateEnvelope(envelope Envelope) error {
	if envelope.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version %q is unsupported", envelope.SchemaVersion)
	}
	if !evidenceIDPattern.MatchString(envelope.ID) {
		return fmt.Errorf("id %q is invalid", envelope.ID)
	}
	if err := validateBoundedString("type", envelope.Type, 128); err != nil {
		return err
	}
	if err := validateBoundedString("subject.repository_uri", envelope.Subject.RepositoryURI, 1024); err != nil {
		return err
	}
	if !objectIDPattern.MatchString(envelope.Subject.Revision) {
		return fmt.Errorf("subject.revision %q is invalid", envelope.Subject.Revision)
	}
	if err := validateBoundedString("producer.type", envelope.Producer.Type, 128); err != nil {
		return err
	}
	if err := validateBoundedString("producer.identity", envelope.Producer.Identity, 1024); err != nil {
		return err
	}
	if err := validateBoundedString("invocation.command_id", envelope.Invocation.CommandID, 256); err != nil {
		return err
	}
	if !envelope.Outcome.Status.Valid() {
		return fmt.Errorf("outcome.status %q is invalid", envelope.Outcome.Status)
	}
	if err := validateBoundedString("artifact.uri", envelope.Artifact.URI, 2048); err != nil {
		return err
	}
	if err := validateDigest("artifact.digest", envelope.Artifact.Digest); err != nil {
		return err
	}
	if err := validateBoundedString("artifact.media_type", envelope.Artifact.MediaType, 256); err != nil {
		return err
	}
	return nil
}

func optionalString(field string, raw json.RawMessage, max int) (*string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, fmt.Errorf("%s must be a string when present", field)
	}

	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("%s must be a string when present", field)
	}
	if err := validateBoundedString(field, value, max); err != nil {
		return nil, err
	}
	return &value, nil
}

func optionalObjectID(field string, raw json.RawMessage) (*string, error) {
	value, err := optionalString(field, raw, 64)
	if err != nil || value == nil {
		return value, err
	}
	if !objectIDPattern.MatchString(*value) {
		return nil, fmt.Errorf("%s %q is invalid", field, *value)
	}
	return value, nil
}

func optionalDigest(field string, raw json.RawMessage) (*inputmeta.Digest, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, fmt.Errorf("%s must be a digest object when present", field)
	}

	var digest inputmeta.Digest
	if err := strictjson.Decode(raw, &digest); err != nil {
		return nil, fmt.Errorf("%s: %w", field, err)
	}
	if err := validateDigest(field, digest); err != nil {
		return nil, err
	}
	return &digest, nil
}

func optionalInteger(field string, raw json.RawMessage) (*big.Int, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	trimmed := bytes.TrimSpace(raw)
	if bytes.Equal(trimmed, []byte("null")) {
		return nil, fmt.Errorf("%s must be an integer when present", field)
	}

	value, ok := new(big.Rat).SetString(string(trimmed))
	if !ok || !value.IsInt() {
		return nil, fmt.Errorf("%s must be an integer when present", field)
	}
	return new(big.Int).Set(value.Num()), nil
}

func validateDigest(field string, digest inputmeta.Digest) error {
	if digest.Algorithm != inputmeta.DigestAlgorithmSHA256 {
		return fmt.Errorf("%s.algorithm %q is unsupported", field, digest.Algorithm)
	}
	if !digestPattern.MatchString(digest.Value) {
		return fmt.Errorf("%s.value %q is invalid", field, digest.Value)
	}
	return nil
}

func validateBoundedString(field, value string, max int) error {
	length := utf8.RuneCountInString(value)
	if length == 0 {
		return fmt.Errorf("%s must not be empty", field)
	}
	if length > max {
		return fmt.Errorf("%s exceeds maximum length %d", field, max)
	}
	return nil
}

func parseTimestamp(field, value string) (time.Time, error) {
	if !timestampPattern.MatchString(value) {
		return time.Time{}, fmt.Errorf("%s is not a valid RFC3339 timestamp", field)
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s is not a valid RFC3339 timestamp: %w", field, err)
	}
	return parsed, nil
}
