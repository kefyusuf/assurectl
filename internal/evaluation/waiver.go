package evaluation

import (
	"fmt"
	"time"

	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/waiver"
)

// waiverResolution drives the per-record application decision; the order of
// the classification below is the documented precedence.
type waiverResolution string

const (
	waiverApply         waiverResolution = "APPLY"
	waiverSuperseded    waiverResolution = "SUPERSEDED"
	waiverRejectTarget  waiverResolution = "REJECT_TARGET"
	waiverRejectScope   waiverResolution = "REJECT_SCOPE"
	waiverRejectExpired waiverResolution = "REJECT_EXPIRED"
	waiverRejectTiming  waiverResolution = "REJECT_TIMING"
)

// applyWaivers applies explicit waiver records to established, waivable
// failures. Application sets RequirementResult.WaiverStatus and nothing
// else: evidence states, outcomes, and other findings are immutable to it
// (ADR-0008). Every supplied record resolves to a status and reappears on
// Result.Waivers for receipt emission.
func applyWaivers(requirements []domain.RequirementResult, waivers []waiver.Record, input Input) ([]domain.RequirementResult, []waiver.Record, []domain.Finding) {
	results := make([]domain.RequirementResult, len(requirements))
	copy(results, requirements)
	for i := range results {
		ids := make([]string, len(results[i].EvidenceIDs))
		copy(ids, results[i].EvidenceIDs)
		results[i].EvidenceIDs = ids
	}

	index := make(map[string]int, len(results))
	for i, result := range results {
		index[result.RequirementID] = i
	}

	resolved := make([]waiver.Record, 0, len(waivers))
	findings := make([]domain.Finding, 0)
	for _, record := range waivers {
		entry := record
		switch resolution := classifyWaiver(record, results, index, input); resolution {
		case waiverApply:
			entry.Status = domain.WaiverValid
			results[index[record.Target.ID]].WaiverStatus = domain.WaiverValid
			findings = append(findings, waiverFinding("WAIVER_APPLIED",
				record.Target.ID, domain.SeverityInfo,
				fmt.Sprintf("waiver %q applied to requirement %q", record.ID, record.Target.ID)))
		case waiverSuperseded:
			entry.Status = domain.WaiverInvalid
			findings = append(findings, waiverFinding("WAIVER_SUPERSEDED",
				record.Target.ID, domain.SeverityInfo,
				fmt.Sprintf("waiver %q is superseded for requirement %q", record.ID, record.Target.ID)))
		default:
			entry.Status = domain.WaiverInvalid
			code := "WAIVER_REJECTED_" + string(resolution[len("REJECT_"):])
			findings = append(findings, waiverFinding(code,
				record.Target.ID, domain.SeverityWarning,
				fmt.Sprintf("waiver %q rejected for requirement %q: %s", record.ID, record.Target.ID, rejectionReason(resolution))))
		}
		resolved = append(resolved, entry)
	}
	return results, resolved, findings
}

func classifyWaiver(record waiver.Record, results []domain.RequirementResult, index map[string]int, input Input) waiverResolution {
	position, exists := index[record.Target.ID]
	if !exists {
		return waiverRejectTarget
	}
	result := results[position]
	if result.EvidenceState != domain.EvidenceValid || result.Outcome != domain.OutcomeFailed || !result.Waivable {
		return waiverRejectTarget
	}
	if !scopeMatches(record, input) {
		return waiverRejectScope
	}
	issued, err := time.Parse(time.RFC3339Nano, record.IssuedAt)
	if err != nil {
		return waiverRejectTiming
	}
	expires, err := time.Parse(time.RFC3339Nano, record.ExpiresAt)
	if err != nil {
		return waiverRejectTiming
	}
	if !expires.After(input.EvaluatedAt) {
		return waiverRejectExpired
	}
	if issued.After(input.EvaluatedAt) {
		return waiverRejectTiming
	}
	if result.WaiverStatus == domain.WaiverValid {
		return waiverSuperseded
	}
	return waiverApply
}

// scopeMatches requires the record's scope to name the resolved subject and
// the contract's work unit exactly; an optional head revision must equal the
// subject's head revision.
func scopeMatches(record waiver.Record, input Input) bool {
	if record.Scope.RepositoryURI != input.Subject.RepositoryURI {
		return false
	}
	if record.Scope.HeadRevision != "" && record.Scope.HeadRevision != input.Subject.HeadRevision {
		return false
	}
	return record.Scope.WorkUnitID == input.Contract.Contract.WorkUnit.ID
}

func waiverFinding(code, requirementID string, severity domain.FindingSeverity, message string) domain.Finding {
	return domain.Finding{
		Code:          code,
		Category:      domain.FindingCategoryWaiver,
		Severity:      severity,
		RequirementID: requirementID,
		Message:       message,
		Blocking:      false,
	}
}

func rejectionReason(resolution waiverResolution) string {
	switch resolution {
	case waiverRejectTarget:
		return "target is not an established waivable failure"
	case waiverRejectScope:
		return "scope does not match the resolved subject and work unit"
	case waiverRejectExpired:
		return "the waiver expired before the evaluation instant"
	default:
		return "the waiver was issued after the evaluation instant"
	}
}
