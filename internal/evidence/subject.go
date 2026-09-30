package evidence

import (
	"fmt"

	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/gitsubject"
)

func validateResolvedSubject(resolved domain.Subject) error {
	canonical, err := gitsubject.CanonicalizeRepositoryIdentity(resolved.RepositoryURI)
	if err != nil {
		return fmt.Errorf("resolved subject repository_uri: %w", err)
	}
	if canonical != resolved.RepositoryURI {
		return fmt.Errorf("resolved subject repository_uri %q is not canonical", resolved.RepositoryURI)
	}
	if !objectIDPattern.MatchString(resolved.HeadRevision) {
		return fmt.Errorf("resolved subject head_revision %q is invalid", resolved.HeadRevision)
	}
	return nil
}

func bindSubject(evidenceSubject Subject, resolved domain.Subject) (SubjectBinding, error) {
	if err := validateResolvedSubject(resolved); err != nil {
		return "", err
	}

	canonicalRepository, err := gitsubject.CanonicalizeRepositoryIdentity(evidenceSubject.RepositoryURI)
	if err != nil {
		return "", fmt.Errorf("evidence subject repository_uri: %w", err)
	}
	if canonicalRepository != resolved.RepositoryURI {
		return "", fmt.Errorf(
			"evidence subject repository_uri %q does not match resolved repository %q",
			canonicalRepository,
			resolved.RepositoryURI,
		)
	}
	if !objectIDPattern.MatchString(evidenceSubject.Revision) {
		return "", fmt.Errorf("evidence subject revision %q is invalid", evidenceSubject.Revision)
	}
	if evidenceSubject.Revision == resolved.HeadRevision {
		return SubjectBindingExact, nil
	}
	return SubjectBindingOtherRevision, nil
}
