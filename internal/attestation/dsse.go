package attestation

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// PayloadTypeReceiptV0 is the DSSE payload type and in-toto predicate type
// for an AssureCTL receipt v0 attestation.
const PayloadTypeReceiptV0 = "https://assurectl.dev/attestations/receipt/v0"

// PAE returns the DSSE pre-authentication encoding, byte-exact per the
// DSSE specification: "DSSEv1" SP len(type) SP type SP len(payload) SP
// payload, with decimal ASCII lengths and single ASCII spaces. Every
// signature covers exactly these bytes.
func PAE(payloadType, payload []byte) []byte {
	header := fmt.Sprintf("DSSEv1 %d ", len(payloadType))
	separator := fmt.Sprintf(" %d ", len(payload))
	encoded := make([]byte, 0, len(header)+len(payloadType)+len(separator)+len(payload))
	encoded = append(encoded, header...)
	encoded = append(encoded, payloadType...)
	encoded = append(encoded, separator...)
	encoded = append(encoded, payload...)
	return encoded
}

// Signature is one DSSE signature entry.
type Signature struct {
	KeyID string `json:"keyid"`
	Sig   string `json:"sig"`
}

// Envelope is a DSSE envelope (v1) holding the base64 payload and its
// signatures.
type Envelope struct {
	PayloadType string      `json:"payloadType"`
	Payload     string      `json:"payload"`
	Signatures  []Signature `json:"signatures"`
}

// NewEnvelope builds an envelope carrying the given payload bytes.
func NewEnvelope(payloadType string, payload []byte) Envelope {
	return Envelope{
		PayloadType: payloadType,
		Payload:     base64.StdEncoding.EncodeToString(payload),
	}
}

// DecodePayload returns the decoded payload bytes of the envelope.
func (e Envelope) DecodePayload() ([]byte, error) {
	payload, err := base64.StdEncoding.DecodeString(e.Payload)
	if err != nil {
		return nil, fmt.Errorf("dsse: payload: %w", err)
	}
	return payload, nil
}

// MarshalDocument renders the envelope as compact JSON.
func (e Envelope) MarshalDocument() ([]byte, error) {
	return json.Marshal(e)
}

// ParseEnvelope parses an envelope document, validating the DSSE structure
// fail-closed.
func ParseEnvelope(document []byte) (Envelope, error) {
	var envelope Envelope
	if err := json.Unmarshal(document, &envelope); err != nil {
		return Envelope{}, fmt.Errorf("dsse: parse: %w", err)
	}
	if envelope.PayloadType == "" || envelope.Payload == "" || len(envelope.Signatures) != 1 {
		return Envelope{}, fmt.Errorf("dsse: envelope must carry a payload type, a payload, and exactly one signature")
	}
	return envelope, nil
}

// KeyID derives the deterministic key id: the SHA-256 of the raw Ed25519
// public key bytes, hex-encoded.
func KeyID(public ed25519.PublicKey) string {
	sum := sha256.Sum256(public)
	return hex.EncodeToString(sum[:])
}
