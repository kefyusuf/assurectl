package evidence

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/kefyusuf/assurectl/internal/inputmeta"
	"github.com/kefyusuf/assurectl/internal/strictjson"
)

type decodedEnvelope struct {
	Envelope   Envelope
	StartedAt  time.Time
	FinishedAt time.Time
	Digest     inputmeta.Digest
}

func decodeEnvelope(data []byte) (decodedEnvelope, error) {
	var envelope Envelope
	if err := strictjson.Decode(data, &envelope); err != nil {
		return decodedEnvelope{}, fmt.Errorf("decode evidence envelope: %w", err)
	}
	if envelope.SchemaVersion != SchemaVersion {
		return decodedEnvelope{}, fmt.Errorf("schema_version %q is unsupported", envelope.SchemaVersion)
	}

	startedAt, err := time.Parse(time.RFC3339Nano, envelope.Invocation.StartedAt)
	if err != nil {
		return decodedEnvelope{}, fmt.Errorf("invocation.started_at: %w", err)
	}
	finishedAt, err := time.Parse(time.RFC3339Nano, envelope.Invocation.FinishedAt)
	if err != nil {
		return decodedEnvelope{}, fmt.Errorf("invocation.finished_at: %w", err)
	}
	if finishedAt.Before(startedAt) {
		return decodedEnvelope{}, fmt.Errorf("invocation.finished_at precedes started_at")
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
