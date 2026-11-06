package attestation

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"
)

func fixtureKey() (ed25519.PublicKey, ed25519.PrivateKey) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	return public, private
}

func TestSignVerifyRoundTrip(t *testing.T) {
	t.Parallel()

	public, private := fixtureKey()
	payload := []byte(`{"k":"v"}`)
	envelope := NewEnvelope(PayloadTypeReceiptV0, payload)

	signed, err := SignEnvelope(envelope, KeyID(public), private)
	if err != nil {
		t.Fatalf("SignEnvelope() error = %v", err)
	}

	root := TrustedRoot{
		RootVersion: 1,
		Issuers: []TrustedIssuer{{
			ID: "test-issuer",
			Keys: []TrustedKey{{
				KeyID:         KeyID(public),
				Ed25519Public: base64.StdEncoding.EncodeToString(public),
			}},
			Valid: ValidityWindow{From: "2026-01-01T00:00:00Z", Until: "2027-01-01T00:00:00Z"},
		}},
	}

	if err := VerifyEnvelope(signed, root, time.Now()); err != nil {
		t.Fatalf("VerifyEnvelope() error = %v", err)
	}
}

func TestSignEnvelopeSetsKeyIDAndSignature(t *testing.T) {
	t.Parallel()

	public, private := fixtureKey()
	envelope := NewEnvelope(PayloadTypeReceiptV0, []byte(`{}`))

	signed, err := SignEnvelope(envelope, KeyID(public), private)
	if err != nil {
		t.Fatal(err)
	}
	if len(signed.Signatures) != 1 || signed.Signatures[0].KeyID != KeyID(public) {
		t.Fatalf("signatures = %#v", signed.Signatures)
	}
	if len(signed.Signatures[0].Sig) == 0 {
		t.Fatalf("signature is empty")
	}
}

func TestVerifyEnvelopeRejectsTampering(t *testing.T) {
	t.Parallel()

	public, private := fixtureKey()
	envelope := NewEnvelope(PayloadTypeReceiptV0, []byte(`{"k":"v"}`))
	signed, err := SignEnvelope(envelope, KeyID(public), private)
	if err != nil {
		t.Fatal(err)
	}
	root := trustedRootFor(public)

	tests := []struct {
		name   string
		mutate func(Envelope) Envelope
	}{
		{
			name: "payload tampered",
			mutate: func(e Envelope) Envelope {
				tampered := []byte(`{"k":"changed"}`)
				e.Payload = base64.StdEncoding.EncodeToString(tampered)
				return e
			},
		},
		{
			name: "payload type tampered",
			mutate: func(e Envelope) Envelope {
				e.PayloadType = "https://evil.example/attestation"
				return e
			},
		},
		{
			name: "signature replaced",
			mutate: func(e Envelope) Envelope {
				e.Signatures[0].Sig = base64.StdEncoding.EncodeToString(make([]byte, ed25519.SignatureSize))
				return e
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := VerifyEnvelope(tt.mutate(signed), root, time.Now()); err == nil {
				t.Fatalf("VerifyEnvelope() accepted tampering")
			}
		})
	}
}

func TestVerifyEnvelopeTrustRootTable(t *testing.T) {
	t.Parallel()

	public, private := fixtureKey()
	envelope := SignEnvelopeForTest(t, public, private)
	rootFor := func(issuers []TrustedIssuer) TrustedRoot {
		return TrustedRoot{RootVersion: 1, Issuers: issuers}
	}
	keyEntry := TrustedKey{KeyID: KeyID(public), Ed25519Public: base64.StdEncoding.EncodeToString(public)}

	tests := []struct {
		name    string
		issuers []TrustedIssuer
	}{
		{name: "unknown keyid", issuers: []TrustedIssuer{{ID: "i", Keys: []TrustedKey{{KeyID: "unknown", Ed25519Public: base64.StdEncoding.EncodeToString(public)}}, Valid: ValidityWindow{From: "2026-01-01T00:00:00Z", Until: "2027-01-01T00:00:00Z"}}}},
		{name: "expired issuer", issuers: []TrustedIssuer{{ID: "i", Keys: []TrustedKey{keyEntry}, Valid: ValidityWindow{From: "2025-01-01T00:00:00Z", Until: "2025-06-01T00:00:00Z"}}}},
		{name: "not yet valid", issuers: []TrustedIssuer{{ID: "i", Keys: []TrustedKey{keyEntry}, Valid: ValidityWindow{From: "2027-01-01T00:00:00Z", Until: "2028-01-01T00:00:00Z"}}}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := VerifyEnvelope(envelope, rootFor(tt.issuers), time.Now()); err == nil {
				t.Fatalf("VerifyEnvelope() accepted an untrusted configuration")
			}
		})
	}

	t.Run("malformed public key", func(t *testing.T) {
		t.Parallel()

		if err := VerifyEnvelope(envelope, rootFor([]TrustedIssuer{{ID: "i", Keys: []TrustedKey{{KeyID: KeyID(public), Ed25519Public: "not-base64!"}}, Valid: ValidityWindow{From: "2026-01-01T00:00:00Z", Until: "2027-01-01T00:00:00Z"}}}), time.Now()); err == nil {
			t.Fatalf("VerifyEnvelope() accepted a malformed key")
		}
	})
}

func SignEnvelopeForTest(t *testing.T, public ed25519.PublicKey, private ed25519.PrivateKey) Envelope {
	t.Helper()
	signed, err := SignEnvelope(NewEnvelope(PayloadTypeReceiptV0, []byte(`{}`)), KeyID(public), private)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func trustedRootFor(public ed25519.PublicKey) TrustedRoot {
	return TrustedRoot{
		RootVersion: 1,
		Issuers: []TrustedIssuer{{
			ID:    "test-issuer",
			Keys:  []TrustedKey{{KeyID: KeyID(public), Ed25519Public: base64.StdEncoding.EncodeToString(public)}},
			Valid: ValidityWindow{From: "2026-01-01T00:00:00Z", Until: "2027-01-01T00:00:00Z"},
		}},
	}
}
