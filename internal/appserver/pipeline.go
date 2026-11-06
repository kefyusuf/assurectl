package appserver

import (
	"context"
	"fmt"
	"time"

	"github.com/kefyusuf/assurectl/internal/contract"
	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/evaluation"
	"github.com/kefyusuf/assurectl/internal/evidence"
	"github.com/kefyusuf/assurectl/internal/gitsubject"
	"github.com/kefyusuf/assurectl/internal/policy"
	"github.com/kefyusuf/assurectl/internal/receipt"
)

// EvaluatePullRequest composes the exact evaluator pipeline that the CLI
// uses - resolve, load the base gate, ingest evidence, evaluate, build the
// receipt - over the injected ports (ADR-0010: the App reuses the evaluator
// as a library). The producer context caps the authority tier: a validated
// CI token earns AUTHORITATIVE; anything else stays advisory (ADR-0010's
// failure posture).
func EvaluatePullRequest(ctx context.Context, event PullRequestEvent, deps Dependencies) (Outcome, error) {
	if deps.WorktreeSource == nil || deps.EvidenceSource == nil || deps.CheckPoster == nil {
		return Outcome{}, fmt.Errorf("app: ports are not wired (ADR-0010: real adapters arrive in a later slice)")
	}
	now := deps.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC().Truncate(time.Second) }
	}
	evaluatedAt := now()

	worktree, err := deps.WorktreeSource.Worktree(ctx, event.InstallationID, event.Repository, event.HeadSHA)
	if err != nil {
		return Outcome{}, fmt.Errorf("app: materialize worktree: %w", err)
	}

	resolution, err := gitsubject.Resolve(ctx, worktree, gitsubject.Options{
		BaseRef: event.BaseSHA,
		HeadRef: event.HeadSHA,
	})
	if err != nil {
		return Outcome{}, fmt.Errorf("app: resolve subject: %w", err)
	}

	bundle, ciToken, err := deps.EvidenceSource.EvidenceBundle(ctx, event.InstallationID, event.Repository, event.PRNumber, event.HeadSHA)
	if err != nil {
		return Outcome{}, fmt.Errorf("app: materialize evidence: %w", err)
	}
	evidenceLoaded, err := evidence.LoadLocal(bundle, resolution.Subject)
	if err != nil {
		return Outcome{}, fmt.Errorf("app: %w", err)
	}

	// A validated producer context is earned, never assumed: the token must
	// bind the exact repository and head revision (ADR-0010 section 4).
	authority := domain.AuthorityAdvisory
	if ciToken != "" {
		claims, validateErr := deps.TokenValidator.Validate(ctx, ciToken, ExpectedClaims{
			Repository: resolution.Subject.RepositoryURI,
			SHA:        event.HeadSHA,
		})
		if validateErr == nil && claims.SHA == event.HeadSHA && claims.Repository == resolution.Subject.RepositoryURI {
			authority = domain.AuthorityAuthoritative
		}
	}

	contractLoaded, err := contract.LoadBase(ctx, worktree, resolution.Subject.BaseRevision)
	if err != nil {
		return Outcome{}, fmt.Errorf("app: %w", err)
	}
	policyLoaded, err := policy.LoadBase(ctx, worktree, resolution.Subject.BaseRevision)
	if err != nil {
		return Outcome{}, fmt.Errorf("app: %w", err)
	}

	evaluationResult, err := evaluation.Evaluate(evaluation.Input{
		Contract:    contractLoaded,
		Policy:      policyLoaded,
		Evidence:    evidenceLoaded,
		Subject:     resolution.Subject,
		EvaluatedAt: evaluatedAt,
	})
	if err != nil {
		return Outcome{}, fmt.Errorf("app: %w", err)
	}

	built, err := receipt.Build(receipt.BuildInput{
		Authority:   authority,
		Subject:     resolution.Subject,
		Contract:    contractLoaded,
		Policy:      policyLoaded,
		Evidence:    evidenceLoaded,
		Evaluation:  evaluationResult,
		EvaluatedAt: evaluatedAt,
	})
	if err != nil {
		return Outcome{}, fmt.Errorf("app: %w", err)
	}

	payload := buildCheckPayload(built)
	if err := deps.CheckPoster.PostCheck(ctx, event.InstallationID, event.Repository, event.PRNumber, payload); err != nil {
		return Outcome{}, fmt.Errorf("app: post check: %w", err)
	}
	return Outcome{Payload: payload}, nil
}

// buildCheckPayload maps the receipt onto the check-payload.v0 protocol:
// the conclusion is derived exactly like the CLI exit codes and never
// overrides the receipt.
func buildCheckPayload(built receipt.Receipt) CheckPayload {
	conclusion := "neutral"
	switch built.CompletionDecision {
	case domain.DecisionApproved, domain.DecisionAcceptedWithRisk:
		conclusion = "success"
	case domain.DecisionRejected:
		conclusion = "failure"
	}
	digest, err := built.Digest()
	receiptDigest := ""
	if err == nil {
		receiptDigest = digest.Value
	}
	return CheckPayload{
		Name:       "assurectl",
		Conclusion: conclusion,
		Title:      fmt.Sprintf("AssureCTL %s (%s)", built.VerificationVerdict, built.Authority),
		Summary: fmt.Sprintf(
			"%d requirement(s) evaluated; %d finding(s); verdict %s, decision %s.",
			len(built.RequirementResults), len(built.Findings), built.VerificationVerdict, built.CompletionDecision,
		),
		ReceiptReference: ReceiptReference{
			ChangeSetDigest: built.Subject.ChangeSetHash,
			ReceiptSHA256:   receiptDigest,
			Authority:       string(built.Authority),
		},
	}
}
