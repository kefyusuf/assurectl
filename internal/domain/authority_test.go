package domain

import "testing"

func TestAuthorityTierValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value AuthorityTier
		want  bool
	}{
		{name: "advisory", value: AuthorityAdvisory, want: true},
		{name: "ci asserted", value: AuthorityCIAsserted, want: true},
		{name: "authoritative", value: AuthorityAuthoritative, want: true},
		{name: "empty", value: AuthorityTier(""), want: false},
		{name: "unknown", value: AuthorityTier("TRUSTED"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.value.Valid(); got != tt.want {
				t.Fatalf("AuthorityTier(%q).Valid() = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
