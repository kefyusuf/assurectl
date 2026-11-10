package attestation

import (
	"crypto/ed25519"
	"testing"
	"time"
)

// rootKeyState is the verifier's acceptance state: the root keys trusted so
// far and the highest accepted root version.
type rootKeyState struct {
	acceptedVersion int
	rootKeys        map[string]ed25519.PublicKey
}

func rootKeyStateFor(t *testing.T, keys map[string]ed25519.PublicKey, version int) rootKeyState {
	t.Helper()
	// The initial pin: one root key id + material, configured out-of-band.
	copied := make(map[string]ed25519.PublicKey, len(keys))
	for keyID, key := range keys {
		copied[keyID] = key
	}
	return rootKeyState{acceptedVersion: version, rootKeys: copied}
}

func TestAcceptSignedRootHappyPath(t *testing.T) {
	t.Parallel()

	pinnedPublic, pinnedPrivate := fixtureRootKey(t)
	pin := RootPin{KeyID: KeyID(pinnedPublic), PublicKey: pinnedPublic}
	payload := rootPayloadForTest(2, []TrustedIssuer{{
		ID:    "issuer-1",
		Keys:  []TrustedKey{{KeyID: KeyID(pinnedPublic), Ed25519Public: base64StdEncode(pinnedPublic)}},
		Valid: ValidityWindow{From: "2026-01-01T00:00:00Z", Until: "2027-01-01T00:00:00Z"},
	}})
	state := rootKeyStateFor(t, map[string]ed25519.PublicKey{KeyID(pinnedPublic): pinnedPublic}, 1)

	document := buildSignedRootDocument(t, 2, payload, []Signature{signRootForTest(t, pinnedPrivate, KeyID(pinnedPublic), payload)})
	signed, err := ParseSignedRoot(document)
	if err != nil {
		t.Fatal(err)
	}

	accepted, err := AcceptSignedRoot(signed, pin, state.acceptedVersion, state.rootKeys, time.Now())
	if err != nil {
		t.Fatalf("AcceptSignedRoot() error = %v", err)
	}
	if accepted.RootVersion != 2 {
		t.Fatalf("accepted version = %d, want 2", accepted.RootVersion)
	}
}

func TestAcceptSignedRootRejectsRollback(t *testing.T) {
	t.Parallel()

	pinnedPublic, pinnedPrivate := fixtureRootKey(t)
	pin := RootPin{KeyID: KeyID(pinnedPublic), PublicKey: pinnedPublic}
	payload := rootPayloadForTest(1, []TrustedIssuer{{
		ID:    "issuer-1",
		Keys:  []TrustedKey{{KeyID: KeyID(pinnedPublic), Ed25519Public: base64StdEncode(pinnedPublic)}},
		Valid: ValidityWindow{From: "2026-01-01T00:00:00Z", Until: "2027-01-01T00:00:00Z"},
	}})
	state := rootKeyStateFor(t, map[string]ed25519.PublicKey{KeyID(pinnedPublic): pinnedPublic}, 3)

	document := buildSignedRootDocument(t, 1, payload, []Signature{signRootForTest(t, pinnedPrivate, KeyID(pinnedPublic), payload)})
	signed, err := ParseSignedRoot(document)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AcceptSignedRoot(signed, pin, state.acceptedVersion, state.rootKeys, time.Now()); err == nil {
		t.Fatalf("AcceptSignedRoot() accepted a rollback")
	}
}

func TestAcceptSignedRootRejectsSelfIntroduction(t *testing.T) {
	t.Parallel()

	pinnedPublic, _ := fixtureRootKey(t)
	newPublic, newPrivate := fixtureRootKey(t)
	pin := RootPin{KeyID: KeyID(pinnedPublic), PublicKey: pinnedPublic}

	// The root is signed ONLY by the new key: new keys cannot introduce
	// themselves.
	payload := rootPayloadForTest(2, []TrustedIssuer{{
		ID:    "issuer-1",
		Keys:  []TrustedKey{{KeyID: KeyID(newPublic), Ed25519Public: base64StdEncode(newPublic)}},
		Valid: ValidityWindow{From: "2026-01-01T00:00:00Z", Until: "2027-01-01T00:00:00Z"},
	}})
	state := rootKeyStateFor(t, map[string]ed25519.PublicKey{KeyID(pinnedPublic): pinnedPublic}, 1)
	document := buildSignedRootDocument(t, 2, payload, []Signature{signRootForTest(t, newPrivate, KeyID(newPublic), payload)})
	signed, err := ParseSignedRoot(document)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := AcceptSignedRoot(signed, pin, state.acceptedVersion, state.rootKeys, time.Now()); err == nil {
		t.Fatalf("AcceptSignedRoot() accepted a self-introduced root key")
	}
}

func TestAcceptSignedRootRotationNeedsOldAndNew(t *testing.T) {
	t.Parallel()

	oldPublic, oldPrivate := fixtureRootKey(t)
	newPublic, newPrivate := fixtureRootKey(t)
	pin := RootPin{KeyID: KeyID(oldPublic), PublicKey: oldPublic}

	payload := rootPayloadForTest(2, []TrustedIssuer{{
		ID:    "issuer-1",
		Keys:  []TrustedKey{{KeyID: KeyID(newPublic), Ed25519Public: base64StdEncode(newPublic)}},
		Valid: ValidityWindow{From: "2026-01-01T00:00:00Z", Until: "2027-01-01T00:00:00Z"},
	}})
	state := rootKeyStateFor(t, map[string]ed25519.PublicKey{KeyID(oldPublic): oldPublic}, 1)

	// Signed only by the old key: accepted, but the new key does not become
	// trusted for future rotations unless it co-signs.
	oldOnlyDocument := buildSignedRootDocument(t, 2, payload, []Signature{signRootForTest(t, oldPrivate, KeyID(oldPublic), payload)})
	signed, err := ParseSignedRoot(oldOnlyDocument)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := AcceptSignedRoot(signed, pin, state.acceptedVersion, state.rootKeys, time.Now())
	if err != nil {
		t.Fatalf("AcceptSignedRoot(old-only) error = %v", err)
	}
	if _, ok := accepted.RootKeys[KeyID(newPublic)]; ok {
		t.Fatalf("new key became trusted without co-signing")
	}

	// Signed by old AND new: the new root key is now trusted for future
	// rotations.
	bothDocument := buildSignedRootDocument(t, 2, payload, []Signature{
		signRootForTest(t, oldPrivate, KeyID(oldPublic), payload),
		signRootForTest(t, newPrivate, KeyID(newPublic), payload),
	})
	signed, err = ParseSignedRoot(bothDocument)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err = AcceptSignedRoot(signed, pin, state.acceptedVersion, state.rootKeys, time.Now())
	if err != nil {
		t.Fatalf("AcceptSignedRoot(both) error = %v", err)
	}
	if _, ok := accepted.RootKeys[KeyID(newPublic)]; !ok {
		t.Fatalf("co-signed new key did not become trusted")
	}
}

func TestAcceptSignedRootRevocationPropagates(t *testing.T) {
	t.Parallel()

	pinnedPublic, pinnedPrivate := fixtureRootKey(t)
	revokedIssuerKey, _ := fixtureRootKey(t)
	pin := RootPin{KeyID: KeyID(pinnedPublic), PublicKey: pinnedPublic}

	v1Issuers := []TrustedIssuer{{
		ID:    "issuer-1",
		Keys:  []TrustedKey{{KeyID: KeyID(revokedIssuerKey), Ed25519Public: base64StdEncode(revokedIssuerKey)}},
		Valid: ValidityWindow{From: "2026-01-01T00:00:00Z", Until: "2027-01-01T00:00:00Z"},
	}}
	v1Payload := rootPayloadForTest(1, v1Issuers)
	v1State := rootKeyStateFor(t, map[string]ed25519.PublicKey{KeyID(pinnedPublic): pinnedPublic}, 0)

	v1Document := buildSignedRootDocument(t, 1, v1Payload, []Signature{signRootForTest(t, pinnedPrivate, KeyID(pinnedPublic), v1Payload)})
	v1Signed, err := ParseSignedRoot(v1Document)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := AcceptSignedRoot(v1Signed, pin, v1State.acceptedVersion, v1State.rootKeys, time.Now())
	if err != nil {
		t.Fatalf("AcceptSignedRoot(v1) error = %v", err)
	}

	// v2 drops the issuer (revocation).
	v2Payload := rootPayloadForTest(2, nil)
	v2Document := buildSignedRootDocument(t, 2, v2Payload, []Signature{signRootForTest(t, pinnedPrivate, KeyID(pinnedPublic), v2Payload)})
	v2Signed, err := ParseSignedRoot(v2Document)
	if err != nil {
		t.Fatal(err)
	}
	revoked, err := AcceptSignedRoot(v2Signed, pin, accepted.RootVersion, accepted.RootKeys, time.Now())
	if err != nil {
		t.Fatalf("AcceptSignedRoot(v2) error = %v", err)
	}

	// A receipt signed by the revoked issuer key now fails verification:
	// the trusted-root resolution runs against the newest accepted root.
	if _, err := revoked.Root.resolveIssuerKey(KeyID(revokedIssuerKey), time.Now()); err == nil {
		t.Fatalf("revoked issuer key still resolves")
	}
}
