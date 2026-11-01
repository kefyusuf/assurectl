package evidence

import (
	"math/big"
	"time"

	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/inputmeta"
)

const SchemaVersion = "assurectl/evidence-envelope/v0"

type SubjectBinding string

const (
	SubjectBindingExact         SubjectBinding = "EXACT"
	SubjectBindingOtherRevision SubjectBinding = "OTHER_REVISION"
)

type Loaded struct {
	Envelope       Envelope
	Source         string
	Digest         inputmeta.Digest
	TrustStatus    inputmeta.TrustStatus
	AuthorityBasis inputmeta.AuthorityBasis
	StartedAt      time.Time
	FinishedAt     time.Time
	SubjectBinding SubjectBinding
}

type Envelope struct {
	SchemaVersion string     `json:"schema_version"`
	ID            string     `json:"id"`
	Type          string     `json:"type"`
	Subject       Subject    `json:"subject"`
	Producer      Producer   `json:"producer"`
	Invocation    Invocation `json:"invocation"`
	Outcome       Outcome    `json:"outcome"`
	Artifact      Artifact   `json:"artifact"`
}

type Subject struct {
	RepositoryURI string `json:"repository_uri"`
	Revision      string `json:"revision"`
}

type Producer struct {
	Type             string  `json:"type"`
	Identity         string  `json:"identity"`
	Workflow         *string `json:"workflow,omitempty"`
	WorkflowRevision *string `json:"workflow_revision,omitempty"`
}

type Invocation struct {
	CommandID         string            `json:"command_id"`
	EnvironmentDigest *inputmeta.Digest `json:"environment_digest,omitempty"`
	StartedAt         string            `json:"started_at"`
	FinishedAt        string            `json:"finished_at"`
}

type Outcome struct {
	Status   domain.ObservedOutcome `json:"status"`
	ExitCode *big.Int               `json:"exit_code,omitempty"`
}

type Artifact struct {
	URI       string           `json:"uri"`
	Digest    inputmeta.Digest `json:"digest"`
	MediaType string           `json:"media_type"`
}
