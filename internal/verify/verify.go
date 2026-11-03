package verify

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kefyusuf/assurectl/internal/contract"
	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/evaluation"
	"github.com/kefyusuf/assurectl/internal/evidence"
	"github.com/kefyusuf/assurectl/internal/gitsubject"
	"github.com/kefyusuf/assurectl/internal/policy"
	"github.com/kefyusuf/assurectl/internal/receipt"
	"github.com/kefyusuf/assurectl/internal/waiver"
)

// Options carries the caller's verification intent. BaseRef is required: the
// change-set boundary is caller intent and is never guessed. WaiversPath is
// an explicit opt-in (ADR-0008): empty means no waivers, never discovery.
type Options struct {
	RepositoryURI string
	BaseRef       string
	HeadRef       string
	WaiversPath   string
	EvaluatedAt   time.Time
}

// Result carries the emitted advisory receipt.
type Result struct {
	Receipt receipt.Receipt
}

// Run composes the local advisory pipeline — resolve, load, ingest,
// evaluate, build — and fails closed at the first error.
func Run(ctx context.Context, dir string, options Options) (Result, error) {
	if strings.TrimSpace(options.BaseRef) == "" {
		return Result{}, fmt.Errorf("verify: base ref is required")
	}
	if options.EvaluatedAt.IsZero() {
		return Result{}, fmt.Errorf("verify: evaluated_at must be supplied")
	}

	resolution, err := gitsubject.Resolve(ctx, dir, gitsubject.Options{
		RepositoryURI: options.RepositoryURI,
		BaseRef:       options.BaseRef,
		HeadRef:       options.HeadRef,
	})
	if err != nil {
		return Result{}, fmt.Errorf("verify: resolve subject: %w", err)
	}

	contractLoaded, err := contract.LoadLocal(dir)
	if err != nil {
		return Result{}, fmt.Errorf("verify: %w", err)
	}
	policyLoaded, err := policy.LoadLocal(dir)
	if err != nil {
		return Result{}, fmt.Errorf("verify: %w", err)
	}
	if len(policyLoaded.Policy.Requirements) == 0 {
		return Result{}, fmt.Errorf("verify: policy defines no requirements")
	}

	evidenceLoaded, err := evidence.LoadLocal(dir, resolution.Subject)
	if err != nil {
		return Result{}, fmt.Errorf("verify: %w", err)
	}

	waiverRecords := []waiver.Record(nil)
	if options.WaiversPath != "" {
		loaded, err := waiver.LoadLocal(dir, options.WaiversPath)
		if err != nil {
			return Result{}, fmt.Errorf("verify: %w", err)
		}
		waiverRecords = loaded.Records
	}

	evaluationResult, err := evaluation.Evaluate(evaluation.Input{
		Contract:    contractLoaded,
		Policy:      policyLoaded,
		Evidence:    evidenceLoaded,
		Subject:     resolution.Subject,
		Waivers:     waiverRecords,
		EvaluatedAt: options.EvaluatedAt,
	})
	if err != nil {
		return Result{}, fmt.Errorf("verify: %w", err)
	}

	// Resolver facts become diagnostic findings: visible, non-blocking, and
	// still covered by the receipt builder's re-derivation check because
	// non-blocking findings never change the kernel's output pair.
	if resolution.Dirty {
		evaluationResult.Findings = append(evaluationResult.Findings, dirtyWorktreeFinding())
	}
	if resolution.LocalOnly {
		evaluationResult.Findings = append(evaluationResult.Findings, localOnlyFinding())
	}

	built, err := receipt.Build(receipt.BuildInput{
		Authority:   domain.AuthorityAdvisory,
		Subject:     resolution.Subject,
		Contract:    contractLoaded,
		Policy:      policyLoaded,
		Evidence:    evidenceLoaded,
		Evaluation:  evaluationResult,
		EvaluatedAt: options.EvaluatedAt,
	})
	if err != nil {
		return Result{}, fmt.Errorf("verify: %w", err)
	}
	return Result{Receipt: built}, nil
}

func dirtyWorktreeFinding() domain.Finding {
	return domain.Finding{
		Code:     "SUBJECT_DIRTY_WORKTREE",
		Category: domain.FindingCategorySubject,
		Severity: domain.SeverityWarning,
		Blocking: false,
		Message:  "the worktree has uncommitted changes relative to the resolved subject",
	}
}

func localOnlyFinding() domain.Finding {
	return domain.Finding{
		Code:     "SUBJECT_LOCAL_ONLY",
		Category: domain.FindingCategorySubject,
		Severity: domain.SeverityWarning,
		Blocking: false,
		Message:  "the repository identity is local-only; no remote origin is configured",
	}
}
