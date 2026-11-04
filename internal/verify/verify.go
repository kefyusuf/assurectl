package verify

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kefyusuf/assurectl/internal/cicontext"
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
// CI carries what the GitHub Actions runner asserts (M2 ci-context design):
// assertion, not validation. TrustedBase reads the gate documents from the
// resolved base revision (ADR-0009) and dominates the authority tier.
type Options struct {
	RepositoryURI string
	BaseRef       string
	HeadRef       string
	WaiversPath   string
	TrustedBase   bool
	CI            *cicontext.Context
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

	// The CI context is an assertion, not a validation: it only survives
	// when it agrees with the resolved subject, fail-closed (no receipt
	// otherwise), per the M2 ci-context design section 4. Trusted-base mode
	// dominates the tier: the receipt claims the strongest authority the
	// caller actually earned (ADR-0009).
	authority := domain.AuthorityAdvisory
	switch {
	case options.TrustedBase:
		authority = domain.AuthorityAuthoritative
	case options.CI != nil:
		authority = domain.AuthorityCIAsserted
	}
	if options.CI != nil {
		canonical, err := options.CI.CanonicalRepository()
		if err != nil {
			return Result{}, fmt.Errorf("verify: %w", err)
		}
		if canonical != resolution.Subject.RepositoryURI {
			return Result{}, fmt.Errorf("verify: CI repository does not match the resolved subject repository")
		}
		if options.CI.SHA != resolution.Subject.HeadRevision {
			return Result{}, fmt.Errorf("verify: CI SHA does not match the resolved head revision")
		}
	}

	var contractLoaded contract.Loaded
	var policyLoaded policy.Loaded
	if options.TrustedBase {
		contractLoaded, err = contract.LoadBase(ctx, dir, resolution.Subject.BaseRevision)
		if err != nil {
			return Result{}, fmt.Errorf("verify: %w", err)
		}
		policyLoaded, err = policy.LoadBase(ctx, dir, resolution.Subject.BaseRevision)
		if err != nil {
			return Result{}, fmt.Errorf("verify: %w", err)
		}
	} else {
		contractLoaded, err = contract.LoadLocal(dir)
		if err != nil {
			return Result{}, fmt.Errorf("verify: %w", err)
		}
		policyLoaded, err = policy.LoadLocal(dir)
		if err != nil {
			return Result{}, fmt.Errorf("verify: %w", err)
		}
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
		Authority:   authority,
		Subject:     resolution.Subject,
		Contract:    contractLoaded,
		Policy:      policyLoaded,
		Evidence:    evidenceLoaded,
		Evaluation:  evaluationResult,
		CI:          options.CI,
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
