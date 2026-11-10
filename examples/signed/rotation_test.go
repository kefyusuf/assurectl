package signed_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/kefyusuf/assurectl/internal/attestation"
)

const (
	rotV1 = 1
	rotV2 = 2
	rotV3 = 3
)

type rootKeyPair struct {
	Public  ed25519.PublicKey
	Private ed25519.PrivateKey
	KeyID   string
}

func mustGenerateKey(t *testing.T) rootKeyPair {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return rootKeyPair{Public: public, Private: private, KeyID: attestation.KeyID(public)}
}

func mustMarshalRoot(t *testing.T, root attestation.TrustedRoot) []byte {
	t.Helper()
	data, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func mustSignRoot(t *testing.T, key rootKeyPair, payload []byte) attestation.Signature {
	t.Helper()
	envelope := attestation.Envelope{
		PayloadType: attestation.RootPayloadTypeV1,
		Payload:     base64Of(payload),
	}
	signed, err := attestation.SignEnvelope(envelope, key.KeyID, key.Private)
	if err != nil {
		t.Fatal(err)
	}
	if len(signed.Signatures) != 1 {
		t.Fatal("fixture bug: expected exactly one signature")
	}
	return signed.Signatures[0]
}

func mustSignedRootDocument(t *testing.T, version int, payload []byte, signatures []attestation.Signature) []byte {
	t.Helper()
	document := map[string]any{
		"root_version": version,
		"payloadType":  attestation.RootPayloadTypeV1,
		"payload":      base64Of(payload),
		"signatures":   signatures,
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func mustParseSignedRoot(t *testing.T, document []byte) attestation.SignedRoot {
	t.Helper()
	signed, err := attestation.ParseSignedRoot(document)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func base64Of(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// rotationChainFixture holds the three-step chain: initial trust, rotation,
// revocation.
type rotationChainFixture struct {
	Pinned    rootKeyPair
	NewRoot   rootKeyPair
	Surviving rootKeyPair
	Revoked   rootKeyPair
	Versions  map[int][]byte
}

func buildRotationChain(t *testing.T) *rotationChainFixture {
	t.Helper()
	fixture := &rotationChainFixture{Versions: map[int][]byte{}}
	fixture.Pinned = mustGenerateKey(t)
	fixture.NewRoot = mustGenerateKey(t)
	fixture.Surviving = mustGenerateKey(t)
	fixture.Revoked = mustGenerateKey(t)

	validity := func() attestation.ValidityWindow {
		return attestation.ValidityWindow{From: "2026-01-01T00:00:00Z", Until: "2027-01-01T00:00:00Z"}
	}
	bothIssuers := []attestation.TrustedIssuer{
		{ID: "issuer-surviving", Keys: []attestation.TrustedKey{{KeyID: fixture.Surviving.KeyID, Ed25519Public: base64Of(fixture.Surviving.Public)}}, Valid: validity()},
		{ID: "issuer-revoked", Keys: []attestation.TrustedKey{{KeyID: fixture.Revoked.KeyID, Ed25519Public: base64Of(fixture.Revoked.Public)}}, Valid: validity()},
	}
	survivingOnly := []attestation.TrustedIssuer{
		{ID: "issuer-surviving", Keys: []attestation.TrustedKey{{KeyID: fixture.Surviving.KeyID, Ed25519Public: base64Of(fixture.Surviving.Public)}}, Valid: validity()},
	}

	v1Payload := mustMarshalRoot(t, attestation.TrustedRoot{RootVersion: rotV1, Issuers: bothIssuers})
	fixture.Versions[rotV1] = mustSignedRootDocument(t, rotV1, v1Payload, []attestation.Signature{mustSignRoot(t, fixture.Pinned, v1Payload)})

	v2Payload := mustMarshalRoot(t, attestation.TrustedRoot{RootVersion: rotV2, Issuers: bothIssuers})
	fixture.Versions[rotV2] = mustSignedRootDocument(t, rotV2, v2Payload, []attestation.Signature{
		mustSignRoot(t, fixture.Pinned, v2Payload),
		mustSignRoot(t, fixture.NewRoot, v2Payload),
	})

	v3Payload := mustMarshalRoot(t, attestation.TrustedRoot{RootVersion: rotV3, Issuers: survivingOnly})
	fixture.Versions[rotV3] = mustSignedRootDocument(t, rotV3, v3Payload, []attestation.Signature{
		mustSignRoot(t, fixture.Pinned, v3Payload),
		mustSignRoot(t, fixture.NewRoot, v3Payload),
	})
	return fixture
}

func TestRotationChainVerifiesStepByStep(t *testing.T) {
	t.Parallel()

	fixture := buildRotationChain(t)
	pin := attestation.RootPin{KeyID: fixture.Pinned.KeyID, PublicKey: fixture.Pinned.Public}
	acceptedKeys := map[string]ed25519.PublicKey{fixture.Pinned.KeyID: fixture.Pinned.Public}

	accepted, err := attestation.AcceptSignedRoot(
		mustParseSignedRoot(t, fixture.Versions[rotV1]), pin, 0, acceptedKeys, time.Now())
	if err != nil {
		t.Fatalf("AcceptSignedRoot(v1) error = %v", err)
	}

	// v2 rotation: the rotation announcement carries the new root key's
	// public material (out-of-band, like the initial pin).
	rotationCandidates := map[string]ed25519.PublicKey{
		fixture.Pinned.KeyID:  fixture.Pinned.Public,
		fixture.NewRoot.KeyID: fixture.NewRoot.Public,
	}
	accepted, err = attestation.AcceptSignedRoot(
		mustParseSignedRoot(t, fixture.Versions[rotV2]), pin, accepted.RootVersion, rotationCandidates, time.Now())
	if err != nil {
		t.Fatalf("AcceptSignedRoot(v2) error = %v", err)
	}
	if _, ok := accepted.RootKeys[fixture.NewRoot.KeyID]; !ok {
		t.Fatalf("rotated root key did not become trusted")
	}

	accepted, err = attestation.AcceptSignedRoot(
		mustParseSignedRoot(t, fixture.Versions[rotV3]), pin, accepted.RootVersion, accepted.RootKeys, time.Now())
	if err != nil {
		t.Fatalf("AcceptSignedRoot(v3) error = %v", err)
	}
	if _, err := accepted.Root.ResolveIssuerKey(fixture.Revoked.KeyID, time.Now()); err == nil {
		t.Fatalf("revoked issuer key still resolves after v3")
	}
	if _, err := accepted.Root.ResolveIssuerKey(fixture.Surviving.KeyID, time.Now()); err != nil {
		t.Fatalf("surviving issuer key failed to resolve: %v", err)
	}
}

func TestRotationChainRejectsRollbackAndSelfIntroduction(t *testing.T) {
	t.Parallel()

	fixture := buildRotationChain(t)
	pin := attestation.RootPin{KeyID: fixture.Pinned.KeyID, PublicKey: fixture.Pinned.Public}
	acceptedKeys := map[string]ed25519.PublicKey{fixture.Pinned.KeyID: fixture.Pinned.Public}

	accepted, err := attestation.AcceptSignedRoot(
		mustParseSignedRoot(t, fixture.Versions[rotV1]), pin, 0, acceptedKeys, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	accepted, err = attestation.AcceptSignedRoot(
		mustParseSignedRoot(t, fixture.Versions[rotV2]), pin, accepted.RootVersion, accepted.RootKeys, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	// Rollback: re-presenting v1 after v2 is rejected.
	if _, err := attestation.AcceptSignedRoot(
		mustParseSignedRoot(t, fixture.Versions[rotV1]), pin, accepted.RootVersion, accepted.RootKeys, time.Now()); err == nil {
		t.Fatalf("rollback was accepted")
	}

	// Self-introduction: a v4 root signed only by a brand-new key is
	// rejected.
	stranger := mustGenerateKey(t)
	v4Issuers := []attestation.TrustedIssuer{
		{ID: "issuer-stranger", Keys: []attestation.TrustedKey{{KeyID: stranger.KeyID, Ed25519Public: base64Of(stranger.Public)}}, Valid: attestation.ValidityWindow{From: "2026-01-01T00:00:00Z", Until: "2027-01-01T00:00:00Z"}},
	}
	v4Payload := mustMarshalRoot(t, attestation.TrustedRoot{RootVersion: 4, Issuers: v4Issuers})
	strangerDocument := mustSignedRootDocument(t, 4, v4Payload, []attestation.Signature{mustSignRoot(t, stranger, v4Payload)})
	if _, err := attestation.AcceptSignedRoot(
		mustParseSignedRoot(t, strangerDocument), pin, accepted.RootVersion, accepted.RootKeys, time.Now()); err == nil {
		t.Fatalf("self-introduction was accepted")
	}
}
