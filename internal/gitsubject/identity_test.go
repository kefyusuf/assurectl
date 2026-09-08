package gitsubject

import "testing"

func TestCanonicalizeRepositoryURI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "https",
			raw:  "https://github.com/acme/checkout.git",
			want: "github.com/acme/checkout",
		},
		{
			name: "scp like ssh",
			raw:  "git@github.com:acme/checkout.git",
			want: "github.com/acme/checkout",
		},
		{
			name: "ssh url",
			raw:  "ssh://git@GITHUB.COM/acme/checkout.git",
			want: "github.com/acme/checkout",
		},
		{
			name: "host lowercased path preserved",
			raw:  "https://GITHUB.COM/Acme/Checkout.git",
			want: "github.com/Acme/Checkout",
		},
		{
			name: "http and https share identity",
			raw:  "http://github.com/acme/checkout",
			want: "github.com/acme/checkout",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := canonicalizeRepositoryURI(tt.raw)
			if err != nil {
				t.Fatalf("canonicalizeRepositoryURI(%q): %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("canonicalizeRepositoryURI(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestCanonicalizeRepositoryURIRejectsAmbiguousOrUnsafeInputs(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"",
		"https://github.com/",
		"https://placeholder-user:placeholder-token@github.com/acme/checkout.git",
		"https://github.com/acme/checkout.git?ref=main",
		"https://github.com/acme/checkout.git#fragment",
		"https://github.com/acme/../checkout.git",
		"https://github.com/acme/./checkout.git",
		"git@github.com:",
		"file:///tmp/checkout",
	}

	for _, raw := range inputs {
		raw := raw
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			if got, err := canonicalizeRepositoryURI(raw); err == nil {
				t.Fatalf("canonicalizeRepositoryURI(%q) = %q, want error", raw, got)
			}
		})
	}
}

func TestCanonicalizeRepositoryURIRejectsExplicitPorts(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		"ssh://git@git.example.com:7999/proj/repo.git",
		"https://git.example.com:8443/proj/repo.git",
	} {
		raw := raw
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			if got, err := canonicalizeRepositoryURI(raw); err == nil {
				t.Fatalf("canonicalizeRepositoryURI(%q) = %q, want unsupported-port error", raw, got)
			}
		})
	}
}

func TestChangeSetDigest(t *testing.T) {
	t.Parallel()

	const (
		repository = "github.com/acme/checkout"
		base       = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		head       = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		want       = "9ec24ca4f653b7952eb4afdf9c18a4c5a13b5cb6e313b24b7d768609eb289814"
	)

	got, err := changeSetDigest(repository, base, head)
	if err != nil {
		t.Fatalf("changeSetDigest: %v", err)
	}
	if got != want {
		t.Fatalf("changeSetDigest = %q, want %q", got, want)
	}

	again, err := changeSetDigest(repository, base, head)
	if err != nil {
		t.Fatalf("changeSetDigest repeat: %v", err)
	}
	if again != got {
		t.Fatalf("changeSetDigest repeat = %q, want %q", again, got)
	}
}

func TestChangeSetDigestRejectsMalformedObjectIDs(t *testing.T) {
	t.Parallel()

	valid40 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	valid64 := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	tests := []struct {
		name string
		base string
		head string
	}{
		{name: "short base", base: "abc", head: valid40},
		{name: "uppercase base", base: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", head: valid40},
		{name: "non hex head", base: valid40, head: "gggggggggggggggggggggggggggggggggggggggg"},
		{name: "wrong length head", base: valid40, head: valid40 + "a"},
		{name: "mixed object formats allowed only when individually valid", base: valid64, head: "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got, err := changeSetDigest("github.com/acme/checkout", tt.base, tt.head); err == nil {
				t.Fatalf("changeSetDigest(%q, %q) = %q, want error", tt.base, tt.head, got)
			}
		})
	}

	if _, err := changeSetDigest("github.com/acme/checkout", valid64, valid64); err != nil {
		t.Fatalf("changeSetDigest with 64-char object IDs: %v", err)
	}
}
