package evaluation

import (
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/kefyusuf/assurectl/internal/contract"
	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/evidence"
	"github.com/kefyusuf/assurectl/internal/inputmeta"
	"github.com/kefyusuf/assurectl/internal/policy"
)

var (
	canonicalIdentifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	digestValuePattern  = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Input is the complete, closed set of facts one evaluation may consider.
// EvaluatedAt is supplied by the caller; this package never reads the wall
// clock so repeated evaluation of deeply equal input stays deeply equal.
type Input struct {
	Contract    contract.Loaded
	Policy      policy.Loaded
	Evidence    []evidence.Loaded
	EvaluatedAt time.Time
}

// Result carries normalized per-requirement facts, findings, and the
// verdict/decision pair produced by the deterministic kernel.
type Result struct {
	Requirements []domain.RequirementResult
	Findings     []domain.Finding
	Verdict      domain.EvaluationResult
}

// Evaluate is a pure function of Input: no clock, filesystem, network, or
// environment access, and no mutation of the inputs.
func Evaluate(input Input) (Result, error) {
	if err := validateInput(input); err != nil {
		return Result{}, fmt.Errorf("evaluate: %w", err)
	}

	requirements := make([]domain.RequirementResult, 0, len(input.Policy.Policy.Requirements))
	for _, requirement := range input.Policy.Policy.Requirements {
		requirements = append(requirements, normalizeRequirement(requirement, input.Evidence, input.EvaluatedAt))
	}
	findings := sortFindings(universeFindings(
		input.Contract.Contract.AcceptanceCriteria,
		input.Policy.Policy.Requirements,
	))
	return Result{Requirements: requirements, Findings: findings}, nil
}

// universeFindings cross-checks the contract's acceptance criteria against
// policy requirements per the design: an unknown reference is a blocking
// contract finding, an unreferenced requirement is an info finding, and
// neither drops a requirement from evaluation.
func universeFindings(criteria []contract.AcceptanceCriterion, requirements []policy.Requirement) []domain.Finding {
	known := make(map[string]struct{}, len(requirements))
	for _, requirement := range requirements {
		known[requirement.ID] = struct{}{}
	}
	referenced := make(map[string]struct{}, len(criteria))

	findings := make([]domain.Finding, 0)
	seenUnknown := make(map[string]struct{})
	for _, criterion := range criteria {
		for _, reference := range criterion.Requirements {
			if _, isKnown := known[reference]; isKnown {
				referenced[reference] = struct{}{}
				continue
			}
			if _, duplicate := seenUnknown[reference]; duplicate {
				continue
			}
			seenUnknown[reference] = struct{}{}
			findings = append(findings, domain.Finding{
				Code:          "CONTRACT_UNKNOWN_REQUIREMENT",
				Category:      domain.FindingCategoryContract,
				Severity:      domain.SeverityError,
				RequirementID: reference,
				Blocking:      true,
				Message: fmt.Sprintf(
					"acceptance criterion %q references requirement %q that policy does not define",
					criterion.ID, reference,
				),
			})
		}
	}

	for _, requirement := range requirements {
		if _, isReferenced := referenced[requirement.ID]; isReferenced {
			continue
		}
		findings = append(findings, domain.Finding{
			Code:          "POLICY_REQUIREMENT_UNREFERENCED",
			Category:      domain.FindingCategoryPolicy,
			Severity:      domain.SeverityInfo,
			RequirementID: requirement.ID,
			Blocking:      false,
			Message:       fmt.Sprintf("policy requirement %q is referenced by no acceptance criterion", requirement.ID),
		})
	}
	return findings
}

// sortFindings orders findings deterministically by code, requirement ID,
// then joined evidence IDs, so output is stable across runs.
func sortFindings(findings []domain.Finding) []domain.Finding {
	ordered := make([]domain.Finding, len(findings))
	copy(ordered, findings)
	sort.SliceStable(ordered, func(i, j int) bool {
		left, right := ordered[i], ordered[j]
		if left.Code != right.Code {
			return left.Code < right.Code
		}
		if left.RequirementID != right.RequirementID {
			return left.RequirementID < right.RequirementID
		}
		return joinEvidenceIDs(left.EvidenceIDs) < joinEvidenceIDs(right.EvidenceIDs)
	})
	return ordered
}

func joinEvidenceIDs(ids []string) string {
	return strings.Join(ids, ",")
}

// normalizeRequirement folds every envelope matched by exact evidence type
// into one requirement result per the design's normalization table: no
// matches are MISSING, consistent candidates are VALID with the aggregated
// outcome, conflicting candidates collapse to INVALID, and stale facts take
// precedence over untrusted ones.
func normalizeRequirement(requirement policy.Requirement, evidence []evidence.Loaded, evaluatedAt time.Time) domain.RequirementResult {
	matches := matchEnvelopes(requirement.EvidenceType, evidence)
	if len(matches) == 0 {
		return domain.RequirementResult{
			RequirementID: requirement.ID,
			EvidenceState: domain.EvidenceMissing,
			Waivable:      requirement.Waivable,
			WaiverStatus:  domain.WaiverNotApplicable,
		}
	}

	var candidateIDs, staleIDs, untrustedIDs []string
	var candidateOutcomes []domain.ObservedOutcome
	for _, index := range matches {
		item := evidence[index]
		switch deriveFact(requirement, item, evaluatedAt) {
		case factCandidate:
			candidateIDs = append(candidateIDs, item.Envelope.ID)
			candidateOutcomes = append(candidateOutcomes, item.Envelope.Outcome.Status)
		case factStaleRevision, factStaleFuture, factStaleAge:
			staleIDs = append(staleIDs, item.Envelope.ID)
		case factUntrustedProducer:
			untrustedIDs = append(untrustedIDs, item.Envelope.ID)
		}
	}

	if len(candidateIDs) > 0 {
		state, outcome := aggregateCandidates(candidateOutcomes)
		return domain.RequirementResult{
			RequirementID: requirement.ID,
			EvidenceState: state,
			EvidenceIDs:   sortedCopy(candidateIDs),
			Outcome:       outcome,
			Waivable:      requirement.Waivable,
			WaiverStatus:  domain.WaiverNotApplicable,
		}
	}
	if len(staleIDs) > 0 {
		return domain.RequirementResult{
			RequirementID: requirement.ID,
			EvidenceState: domain.EvidenceStale,
			EvidenceIDs:   sortedCopy(staleIDs),
			Waivable:      requirement.Waivable,
			WaiverStatus:  domain.WaiverNotApplicable,
		}
	}
	return domain.RequirementResult{
		RequirementID: requirement.ID,
		EvidenceState: domain.EvidenceUntrusted,
		EvidenceIDs:   sortedCopy(untrustedIDs),
		Waivable:      requirement.Waivable,
		WaiverStatus:  domain.WaiverNotApplicable,
	}
}

// aggregateCandidates folds consistent candidate outcomes: any ERROR
// dominates, otherwise any FAILED establishes failure, and conflicting
// PASSED/FAILED candidates are internally inconsistent evidence (INVALID).
func aggregateCandidates(outcomes []domain.ObservedOutcome) (domain.EvidenceState, domain.ObservedOutcome) {
	anyPassed, anyFailed, anyError := false, false, false
	for _, outcome := range outcomes {
		switch outcome {
		case domain.OutcomePassed:
			anyPassed = true
		case domain.OutcomeFailed:
			anyFailed = true
		case domain.OutcomeError:
			anyError = true
		}
	}
	switch {
	case anyFailed && anyPassed:
		return domain.EvidenceInvalid, ""
	case anyError:
		return domain.EvidenceValid, domain.OutcomeError
	case anyFailed:
		return domain.EvidenceValid, domain.OutcomeFailed
	default:
		return domain.EvidenceValid, domain.OutcomePassed
	}
}

// sortedCopy returns a sorted, deduplicated copy of the evidence IDs.
func sortedCopy(ids []string) []string {
	ordered := make([]string, len(ids))
	copy(ordered, ids)
	sort.Strings(ordered)
	deduplicated := ordered[:0]
	for i, id := range ordered {
		if i == 0 || ordered[i-1] != id {
			deduplicated = append(deduplicated, id)
		}
	}
	return deduplicated
}

func validateInput(input Input) error {
	if input.EvaluatedAt.IsZero() {
		return fmt.Errorf("evaluated_at must be supplied")
	}
	if err := validateContractInput(input.Contract); err != nil {
		return err
	}
	if err := validatePolicyInput(input.Policy); err != nil {
		return err
	}

	seenRequirements := make(map[string]struct{}, len(input.Policy.Policy.Requirements))
	for i, requirement := range input.Policy.Policy.Requirements {
		if !canonicalIdentifier.MatchString(requirement.ID) {
			return fmt.Errorf("policy requirement[%d].id %q: must be a canonical identifier", i, requirement.ID)
		}
		if _, exists := seenRequirements[requirement.ID]; exists {
			return fmt.Errorf("policy requirement[%d].id: duplicate identifier %q", i, requirement.ID)
		}
		seenRequirements[requirement.ID] = struct{}{}
	}

	for i, criterion := range input.Contract.Contract.AcceptanceCriteria {
		for j, reference := range criterion.Requirements {
			if !canonicalIdentifier.MatchString(reference) {
				return fmt.Errorf(
					"contract acceptance criterion[%d] (%q).requirements[%d] %q: must be a canonical identifier",
					i, criterion.ID, j, reference,
				)
			}
		}
	}
	return nil
}

func validateContractInput(loaded contract.Loaded) error {
	if loaded.Contract.SchemaVersion != contract.SchemaVersion {
		return fmt.Errorf("contract schema_version %q is unsupported", loaded.Contract.SchemaVersion)
	}
	if err := validateDigest(loaded.Digest); err != nil {
		return fmt.Errorf("contract digest: %w", err)
	}
	if loaded.TrustStatus != inputmeta.TrustStatusUntrusted {
		return fmt.Errorf("contract trust status %q is not advisory workspace input", loaded.TrustStatus)
	}
	return nil
}

func validatePolicyInput(loaded policy.Loaded) error {
	if loaded.Policy.SchemaVersion != policy.SchemaVersion {
		return fmt.Errorf("policy schema_version %q is unsupported", loaded.Policy.SchemaVersion)
	}
	if err := validateDigest(loaded.Digest); err != nil {
		return fmt.Errorf("policy digest: %w", err)
	}
	if loaded.TrustStatus != inputmeta.TrustStatusUntrusted {
		return fmt.Errorf("policy trust status %q is not advisory workspace input", loaded.TrustStatus)
	}
	if loaded.AuthorityBasis != inputmeta.AuthorityBasisAdvisoryWorkspace {
		return fmt.Errorf("policy authority basis %q is not advisory workspace input", loaded.AuthorityBasis)
	}
	return nil
}

func validateDigest(digest inputmeta.Digest) error {
	if digest.Algorithm != inputmeta.DigestAlgorithmSHA256 {
		return fmt.Errorf("algorithm %q is unsupported", digest.Algorithm)
	}
	if !digestValuePattern.MatchString(digest.Value) {
		return fmt.Errorf("value %q is invalid", digest.Value)
	}
	return nil
}

// matchEnvelopes returns the indexes of evidence entries whose envelope type
// equals evidenceType by exact, case-sensitive string comparison, preserving
// ingestion order.
func matchEnvelopes(evidenceType string, evidence []evidence.Loaded) []int {
	matches := make([]int, 0)
	for i, item := range evidence {
		if item.Envelope.Type == evidenceType {
			matches = append(matches, i)
		}
	}
	return matches
}

// envelopeFact is the per-envelope evaluation fact the design defines for one
// matched evidence entry and one requirement. First match wins; the order of
// the checks below is the documented precedence.
type envelopeFact string

const (
	factCandidate         envelopeFact = "CANDIDATE"
	factStaleRevision     envelopeFact = "STALE_REVISION"
	factStaleFuture       envelopeFact = "STALE_FUTURE"
	factStaleAge          envelopeFact = "STALE_AGE"
	factUntrustedProducer envelopeFact = "UNTRUSTED_PRODUCER"
)

// deriveFact classifies one matched envelope against one requirement using
// only ingestion-established facts and the explicit evaluation instant.
func deriveFact(requirement policy.Requirement, item evidence.Loaded, evaluatedAt time.Time) envelopeFact {
	if item.SubjectBinding == evidence.SubjectBindingOtherRevision {
		return factStaleRevision
	}
	if item.FinishedAt.After(evaluatedAt) {
		return factStaleFuture
	}
	if requirement.MaxAgeSeconds != nil && exceedsMaxAge(requirement.MaxAgeSeconds, evaluatedAt.Sub(item.FinishedAt)) {
		return factStaleAge
	}
	if !allowsProducer(requirement.AllowedProducerTypes, item.Envelope.Producer.Type) {
		return factUntrustedProducer
	}
	return factCandidate
}

// exceedsMaxAge compares the observation age against the policy bound with
// exact nanosecond arithmetic; bounds beyond int64 seconds simply never
// expire within representable durations.
func exceedsMaxAge(limit *big.Int, age time.Duration) bool {
	ageNanos := new(big.Int).SetInt64(age.Nanoseconds())
	limitNanos := new(big.Int).Mul(limit, big.NewInt(int64(time.Second)))
	return ageNanos.Cmp(limitNanos) > 0
}

// allowsProducer requires an explicit allowlist hit; an empty list permits
// no producer type (fail closed).
func allowsProducer(allowed []string, producerType string) bool {
	for _, candidate := range allowed {
		if candidate == producerType {
			return true
		}
	}
	return false
}
