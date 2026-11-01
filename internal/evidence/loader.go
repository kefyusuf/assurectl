package evidence

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/inputmeta"
	"github.com/kefyusuf/assurectl/internal/localinput"
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

const localEvidenceRelativeRoot = ".assurectl/evidence"

type localCandidate struct {
	RelativePath string
	Source       string
	Data         []byte
}

type localDiscovery struct {
	EvidenceRoot string
	Candidates   []localCandidate
}

func discoverLocal(root string) (localDiscovery, error) {
	if strings.TrimSpace(root) == "" {
		return localDiscovery{}, errors.New("workspace root is empty")
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return localDiscovery{}, fmt.Errorf("resolve workspace root: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return localDiscovery{}, fmt.Errorf("resolve workspace root: %w", err)
	}
	rootInfo, err := os.Stat(resolvedRoot)
	if err != nil {
		return localDiscovery{}, fmt.Errorf("stat workspace root: %w", err)
	}
	if !rootInfo.IsDir() {
		return localDiscovery{}, errors.New("workspace root is not a directory")
	}

	controlDir := filepath.Join(resolvedRoot, ".assurectl")
	if _, err := os.Lstat(controlDir); err != nil {
		if os.IsNotExist(err) {
			return localDiscovery{}, nil
		}
		return localDiscovery{}, fmt.Errorf("inspect evidence control directory: %w", err)
	}
	resolvedControlDir, err := filepath.EvalSymlinks(controlDir)
	if err != nil {
		return localDiscovery{}, fmt.Errorf("resolve evidence control directory: %w", err)
	}
	inside, err := discoveryPathWithin(resolvedRoot, resolvedControlDir)
	if err != nil {
		return localDiscovery{}, fmt.Errorf("resolve evidence control directory: %w", err)
	}
	if !inside {
		return localDiscovery{}, errors.New("evidence control directory resolves outside workspace")
	}
	controlInfo, err := os.Stat(resolvedControlDir)
	if err != nil {
		return localDiscovery{}, fmt.Errorf("stat evidence control directory: %w", err)
	}
	if !controlInfo.IsDir() {
		return localDiscovery{}, errors.New("evidence control directory is not a directory")
	}

	logicalEvidenceRoot := filepath.Join(resolvedControlDir, "evidence")
	if _, err := os.Lstat(logicalEvidenceRoot); err != nil {
		if os.IsNotExist(err) {
			return localDiscovery{}, nil
		}
		return localDiscovery{}, fmt.Errorf("inspect evidence root: %w", err)
	}
	resolvedEvidenceRoot, err := filepath.EvalSymlinks(logicalEvidenceRoot)
	if err != nil {
		return localDiscovery{}, fmt.Errorf("resolve evidence root: %w", err)
	}
	inside, err = discoveryPathWithin(resolvedRoot, resolvedEvidenceRoot)
	if err != nil {
		return localDiscovery{}, fmt.Errorf("resolve evidence root: %w", err)
	}
	if !inside {
		return localDiscovery{}, errors.New("evidence root resolves outside workspace")
	}
	evidenceInfo, err := os.Stat(resolvedEvidenceRoot)
	if err != nil {
		return localDiscovery{}, fmt.Errorf("stat evidence root: %w", err)
	}
	if !evidenceInfo.IsDir() {
		return localDiscovery{}, errors.New("evidence root is not a directory")
	}

	entries, err := os.ReadDir(resolvedEvidenceRoot)
	if err != nil {
		return localDiscovery{}, fmt.Errorf("read evidence root: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.Type().IsRegular() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	candidates := make([]localCandidate, 0, len(names))
	for _, name := range names {
		relativePath := localEvidenceRelativeRoot + "/" + name
		data, err := localinput.ReadWorkspaceFile(root, relativePath)
		if err != nil {
			return localDiscovery{}, fmt.Errorf("read evidence candidate %q: %w", relativePath, err)
		}
		candidates = append(candidates, localCandidate{
			RelativePath: relativePath,
			Source:       "workspace:" + relativePath,
			Data:         data,
		})
	}

	return localDiscovery{
		EvidenceRoot: resolvedEvidenceRoot,
		Candidates:   candidates,
	}, nil
}

func discoveryPathWithin(root, candidate string) (bool, error) {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false, err
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)), nil
}

func LoadLocal(root string, subject domain.Subject) ([]Loaded, error) {
	if err := validateResolvedSubject(subject); err != nil {
		return nil, err
	}

	discovery, err := discoverLocal(root)
	if err != nil {
		return nil, err
	}

	type decodedCandidate struct {
		candidate localCandidate
		decoded   decodedEnvelope
	}
	decodedCandidates := make([]decodedCandidate, 0, len(discovery.Candidates))
	seenIDs := make(map[string]struct{}, len(discovery.Candidates))
	seenArtifactURIs := make(map[string]struct{}, len(discovery.Candidates))

	for _, candidate := range discovery.Candidates {
		decoded, err := decodeEnvelope(candidate.Data)
		if err != nil {
			return nil, fmt.Errorf("load evidence %q: %w", candidate.Source, err)
		}
		if _, exists := seenIDs[decoded.Envelope.ID]; exists {
			return nil, fmt.Errorf("duplicate evidence id %q", decoded.Envelope.ID)
		}
		seenIDs[decoded.Envelope.ID] = struct{}{}

		artifactURI := decoded.Envelope.Artifact.URI
		if err := validatePortableArtifactURI(artifactURI); err != nil {
			return nil, fmt.Errorf("load evidence %q artifact uri: %w", candidate.Source, err)
		}
		if _, exists := seenArtifactURIs[artifactURI]; exists {
			return nil, fmt.Errorf("duplicate artifact uri %q", artifactURI)
		}
		seenArtifactURIs[artifactURI] = struct{}{}

		decodedCandidates = append(decodedCandidates, decodedCandidate{
			candidate: candidate,
			decoded:   decoded,
		})
	}

	loaded := make([]Loaded, 0, len(decodedCandidates))
	for _, item := range decodedCandidates {
		binding, err := bindSubject(item.decoded.Envelope.Subject, subject)
		if err != nil {
			return nil, fmt.Errorf("bind evidence %q: %w", item.candidate.Source, err)
		}
		if err := verifyArtifact(
			discovery.EvidenceRoot,
			item.decoded.Envelope.Artifact.URI,
			item.decoded.Envelope.Artifact.Digest,
		); err != nil {
			return nil, fmt.Errorf("verify evidence %q artifact: %w", item.candidate.Source, err)
		}

		loaded = append(loaded, Loaded{
			Envelope:       item.decoded.Envelope,
			Source:         item.candidate.Source,
			Digest:         item.decoded.Digest,
			TrustStatus:    inputmeta.TrustStatusUntrusted,
			AuthorityBasis: inputmeta.AuthorityBasisAdvisoryWorkspace,
			StartedAt:      item.decoded.StartedAt,
			FinishedAt:     item.decoded.FinishedAt,
			SubjectBinding: binding,
		})
	}
	return loaded, nil
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
