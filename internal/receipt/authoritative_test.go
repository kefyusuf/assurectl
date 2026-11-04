package receipt

import (
	"testing"

	"github.com/kefyusuf/assurectl/internal/contract"
	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/inputmeta"
	"github.com/kefyusuf/assurectl/internal/policy"
)

const (
	trustedOID         = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	basePolicySource   = "base:" + trustedOID + ":.assurectl/policy.json"
	baseContractSource = "base:" + trustedOID + ":.assurectl/contract.json"
)

func trustedPolicy() policy.Loaded {
	loaded := policy.Loaded{
		Policy: policy.Policy{
			SchemaVersion: policy.SchemaVersion,
		},
		Source:         basePolicySource,
		Digest:         seededDigest("base-policy"),
		TrustStatus:    inputmeta.TrustStatusTrusted,
		AuthorityBasis: inputmeta.AuthorityBasisTrustedBase,
	}
	return loaded
}

func trustedContract() contract.Loaded {
	return contract.Loaded{
		Contract: contract.Contract{
			SchemaVersion: contract.SchemaVersion,
			WorkUnit: domain.WorkUnit{
				ID:        "wu-001",
				Type:      domain.WorkUnitCommit,
				Objective: "trusted contract",
			},
		},
		Source:      baseContractSource,
		Digest:      seededDigest("base-contract"),
		TrustStatus: inputmeta.TrustStatusTrusted,
	}
}

func authoritativeInput() BuildInput {
	input := buildInput()
	input.Authority = domain.AuthorityAuthoritative
	input.Contract = trustedContract()
	input.Policy = trustedPolicy()
	return input
}

func TestBuildAcceptsAuthoritativeWithTrustedInputs(t *testing.T) {
	t.Parallel()

	built, err := Build(authoritativeInput())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if built.Authority != domain.AuthorityAuthoritative {
		t.Fatalf("Authority = %q, want AUTHORITATIVE", built.Authority)
	}
	if built.Policy.TrustStatus != inputmeta.TrustStatusTrusted {
		t.Fatalf("policy trust = %q, want TRUSTED", built.Policy.TrustStatus)
	}
	bindingFound := false
	for _, source := range built.Policy.Sources {
		if source.Layer == LayerProject && source.RevisionBinding == "SUBJECT_BASE" {
			bindingFound = true
		}
	}
	if !bindingFound {
		t.Fatalf("project layer lacks the SUBJECT_BASE marker: %#v", built.Policy.Sources)
	}
}

func TestBuildRejectsAuthoritativeWithUntrustedInputs(t *testing.T) {
	t.Parallel()

	input := authoritativeInput()
	input.Policy = buildPolicy() // advisory workspace policy
	if _, err := Build(input); err == nil {
		t.Fatalf("Build() accepted AUTHORITATIVE with an untrusted policy")
	}
}

func TestBuildRejectsHandForgedTrustedSource(t *testing.T) {
	t.Parallel()

	input := authoritativeInput()
	forged := trustedPolicy()
	forged.Source = "workspace:.assurectl/policy.json"
	input.Policy = forged
	if _, err := Build(input); err == nil {
		t.Fatalf("Build() accepted a hand-forged TRUSTED source")
	}
}

func TestBuildRejectsHandForgedTrustedContract(t *testing.T) {
	t.Parallel()

	input := authoritativeInput()
	forged := trustedContract()
	forged.Source = "workspace:.assurectl/contract.json"
	input.Contract = forged
	if _, err := Build(input); err == nil {
		t.Fatalf("Build() accepted a hand-forged trusted contract")
	}
}

func TestBuildRejectsAuthoritativeWithoutSubjectBaseMarker(t *testing.T) {
	t.Parallel()

	input := authoritativeInput()
	input.Policy.AuthorityBasis = inputmeta.AuthorityBasisAdvisoryWorkspace
	if _, err := Build(input); err == nil {
		t.Fatalf("Build() accepted AUTHORITATIVE without the TRUSTED_BASE basis")
	}
}
