package contract

import "testing"

func TestLocalContractPathIsFixed(t *testing.T) {
	if localContractPath != ".assurectl/contract.json" {
		t.Fatalf("localContractPath = %q", localContractPath)
	}
}
