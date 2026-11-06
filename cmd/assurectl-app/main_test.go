package main

import (
	"testing"
)

func TestRunAppRejectsMissingRequiredFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "missing app id", args: []string{}},
		{name: "missing private key", args: []string{"--app-id", "1"}},
		{name: "missing webhook secret", args: []string{"--app-id", "1", "--private-key-path", "key.pem"}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if code := run(tt.args); code != 64 {
				t.Fatalf("run() exit = %d, want 64", code)
			}
		})
	}
}
