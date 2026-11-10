package attestation

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"
)

const rootPayloadTypeV1 = "https://assurectl.dev/trusted-root/v1"

func rootPayloadForTest(rootVersion int, issuers []TrustedIssuer) []byte {
	payload, err := json.Marshal(struct {
		RootVersion int             `json:"root_version"`
		Issuers     []TrustedIssuer `json:"issuers"`
	}{RootVersion: rootVersion, Issuers: issuers})
	if err != nil {
		panic(err)
	}
	return payload
}

func signRootForTest(t *testing.T, key ed25519.PrivateKey, keyID string, payload []byte) Signature {
	t.Helper()
	pae := PAE([]byte(rootPayloadTypeV1), payload)
	return Signature{KeyID: keyID, Sig: base64RawEncode(ed25519.Sign(key, pae))}
}

func TestParseSignedRootAcceptsWellFormedDocument(t *testing.T) {
	t.Parallel()

	public, private := fixtureRootKey(t)
	payload := rootPayloadForTest(2, []TrustedIssuer{{
		ID:    "issuer-1",
		Keys:  []TrustedKey{{KeyID: "issuer-key", Ed25519Public: mustIssuerKey(t, public)}},
		Valid: ValidityWindow{From: "2026-01-01T00:00:00Z", Until: "2027-01-01T00:00:00Z"},
	}})
	document := buildSignedRootDocument(t, 2, payload, []Signature{signRootForTest(t, private, "root-key", payload)})

	signed, err := ParseSignedRoot(document)
	if err != nil {
		t.Fatalf("ParseSignedRoot() error = %v", err)
	}
	if signed.RootVersion != 2 {
		t.Fatalf("RootVersion = %d, want 2", signed.RootVersion)
	}
	if len(signed.Signatures) != 1 || signed.Signatures[0].KeyID != "root-key" {
		t.Fatalf("Signatures = %#v", signed.Signatures)
	}
	if signed.PayloadType != rootPayloadTypeV1 {
		t.Fatalf("PayloadType = %q", signed.PayloadType)
	}
}

func mustIssuerKey(t *testing.T, public ed25519.PublicKey) string {
	t.Helper()
	return base64StdEncode(public)
}

func base64RawEncode(data []byte) string {
	return base64.RawStdEncoding.EncodeToString(data)
}

func base64StdEncode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func buildSignedRootDocument(t *testing.T, rootVersion int, payload []byte, signatures []Signature) []byte {
	t.Helper()
	document := signedRootDocument{
		RootVersion: rootVersion,
		PayloadType: rootPayloadTypeV1,
		Payload:     base64RawEncode(payload),
		Signatures:  signatures,
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

type signedRootDocument struct {
	RootVersion int         `json:"root_version"`
	PayloadType string      `json:"payloadType"`
	Payload     string      `json:"payload"`
	Signatures  []Signature `json:"signatures"`
}

func fixtureRootKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return public, private
}
