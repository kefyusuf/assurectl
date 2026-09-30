package evidence

import (
	"testing"

	"github.com/kefyusuf/assurectl/internal/domain"
)

const (
	testHeadRevision  = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	testOtherRevision = "cccccccccccccccccccccccccccccccccccccccc"
	testLocalRepo      = "local://sha256/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
)

func TestBindSubject(t *testing.T) {
	t.Parallel()

	t.Run("normalizes evidence URI and binds exact hosted revision", func(t *testing.T) {
		t.Parallel()

		resolved := domain.Subject{
			RepositoryURI: "github.com/acme/checkout",
			HeadRevision:  testHeadRevision,
		}
		got, err := bindSubject(Subject{
			RepositoryURI: "https://github.com/acme/checkout.git",
			Revision:      testHeadRevision,
		}, resolved)
		if err != nil {
			t.Fatalf("bindSubject() error = %v", err)
		}
		if got != SubjectBindingExact {
			t.Fatalf("bindSubject() = %q, want %q", got, SubjectBindingExact)
		}
	})

	t.Run("binds exact local advisory identity", func(t *testing.T) {
		t.Parallel()

		resolved := domain.Subject{
			RepositoryURI: testLocalRepo,
			HeadRevision:  testHeadRevision,
		}
		got, err := bindSubject(Subject{
			RepositoryURI: testLocalRepo,
			Revision:      testHeadRevision,
		}, resolved)
		if err != nil {
			t.Fatalf("bindSubject() error = %v", err)
		}
		if got != SubjectBindingExact {
			t.Fatalf("bindSubject() = %q, want %q", got, SubjectBindingExact)
		}
	})

	t.Run("same repository at another revision is a successful fact", func(t *testing.T) {
		t.Parallel()

		resolved := domain.Subject{
			RepositoryURI: "github.com/acme/checkout",
			HeadRevision:  testHeadRevision,
		}
		got, err := bindSubject(Subject{
			RepositoryURI: "git@github.com:acme/checkout.git",
			Revision:      testOtherRevision,
		}, resolved)
		if err != nil {
			t.Fatalf("bindSubject() error = %v", err)
		}
		if got != SubjectBindingOtherRevision {
			t.Fatalf("bindSubject() = %q, want %q", got, SubjectBindingOtherRevision)
		}
	})

	t.Run("different repository fails", func(t *testing.T) {
		t.Parallel()

		resolved := domain.Subject{
			RepositoryURI: "github.com/acme/checkout",
			HeadRevision:  testHeadRevision,
		}
		if got, err := bindSubject(Subject{
			RepositoryURI: "https://github.com/acme/other.git",
			Revision:      testHeadRevision,
		}, resolved); err == nil {
			t.Fatalf("bindSubject() = %q, want error", got)
		}
	})

	t.Run("malformed evidence repository fails", func(t *testing.T) {
		t.Parallel()

		resolved := domain.Subject{
			RepositoryURI: "github.com/acme/checkout",
			HeadRevision:  testHeadRevision,
		}
		if got, err := bindSubject(Subject{
			RepositoryURI: "https://github.com/acme/%2e%2e/checkout",
			Revision:      testHeadRevision,
		}, resolved); err == nil {
			t.Fatalf("bindSubject() = %q, want error", got)
		}
	})

	t.Run("does not repair malformed resolved subject", func(t *testing.T) {
		t.Parallel()

		resolved := domain.Subject{
			RepositoryURI: "https://github.com/acme/checkout.git",
			HeadRevision:  testHeadRevision,
		}
		if got, err := bindSubject(Subject{
			RepositoryURI: "github.com/acme/checkout",
			Revision:      testHeadRevision,
		}, resolved); err == nil {
			t.Fatalf("bindSubject() = %q, want error", got)
		}
	})
}

func TestValidateResolvedSubject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		subject domain.Subject
		wantErr bool
	}{
		{
			name: "canonical hosted subject",
			subject: domain.Subject{
				RepositoryURI: "github.com/acme/checkout",
				HeadRevision:  testHeadRevision,
			},
		},
		{
			name: "canonical local subject",
			subject: domain.Subject{
				RepositoryURI: testLocalRepo,
				HeadRevision:  testHeadRevision,
			},
		},
		{
			name: "non canonical repository identity",
			subject: domain.Subject{
				RepositoryURI: "https://github.com/acme/checkout.git",
				HeadRevision:  testHeadRevision,
			},
			wantErr: true,
		},
		{
			name: "malformed repository identity",
			subject: domain.Subject{
				RepositoryURI: "not-a-repository",
				HeadRevision:  testHeadRevision,
			},
			wantErr: true,
		},
		{
			name: "empty repository identity",
			subject: domain.Subject{
				HeadRevision: testHeadRevision,
			},
			wantErr: true,
		},
		{
			name: "invalid head revision",
			subject: domain.Subject{
				RepositoryURI: "github.com/acme/checkout",
				HeadRevision:  "ABC",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateResolvedSubject(tt.subject)
			if tt.wantErr && err == nil {
				t.Fatal("validateResolvedSubject() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("validateResolvedSubject() error = %v", err)
			}
		})
	}
}
