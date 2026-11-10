package attestation

import (
	"encoding/json"
	"fmt"
)

// RootPayloadTypeV1 is the DSSE payload type for a signed trusted root.
const RootPayloadTypeV1 = "https://assurectl.dev/trusted-root/v1"

// SignedRoot is a trusted-root document wrapped in a DSSE-style envelope:
// the payload is the slice-1 trusted-root shape (root_version + issuers),
// signed by root keys over the PAE.
type SignedRoot struct {
	RootVersion int         `json:"root_version"`
	PayloadType string      `json:"payloadType"`
	Payload     string      `json:"payload"`
	Signatures  []Signature `json:"signatures"`
}

// ParseSignedRoot parses a signed-root document, validating the envelope
// structure fail-closed.
func ParseSignedRoot(document []byte) (SignedRoot, error) {
	var signed SignedRoot
	if err := json.Unmarshal(document, &signed); err != nil {
		return SignedRoot{}, fmt.Errorf("signed root: parse: %w", err)
	}
	if signed.RootVersion <= 0 {
		return SignedRoot{}, fmt.Errorf("signed root: root_version must be positive")
	}
	if signed.PayloadType != RootPayloadTypeV1 {
		return SignedRoot{}, fmt.Errorf("signed root: payloadType %q is unsupported", signed.PayloadType)
	}
	if signed.Payload == "" || len(signed.Signatures) == 0 {
		return SignedRoot{}, fmt.Errorf("signed root: payload and at least one signature are required")
	}
	payload, err := signed.DecodePayload()
	if err != nil {
		return SignedRoot{}, fmt.Errorf("signed root: %w", err)
	}
	var probe struct {
		RootVersion int `json:"root_version"`
	}
	if err := json.Unmarshal(payload, &probe); err != nil {
		return SignedRoot{}, fmt.Errorf("signed root: payload: %w", err)
	}
	if probe.RootVersion != signed.RootVersion {
		return SignedRoot{}, fmt.Errorf(
			"signed root: envelope version %d does not match payload version %d",
			signed.RootVersion, probe.RootVersion,
		)
	}
	return signed, nil
}

// DecodePayload returns the signed root's payload bytes.
func (s SignedRoot) DecodePayload() ([]byte, error) {
	payload, err := base64Decode(s.Payload)
	if err != nil {
		return nil, fmt.Errorf("payload: %w", err)
	}
	return payload, nil
}

// ParseRootPayload parses a signed root's payload into the slice-1 trusted
// root, reusing its validation.
func (s SignedRoot) ParseRootPayload() (TrustedRoot, error) {
	payload, err := s.DecodePayload()
	if err != nil {
		return TrustedRoot{}, fmt.Errorf("signed root: %w", err)
	}
	return ParseTrustedRoot(payload)
}

func base64Decode(value string) ([]byte, error) {
	if decoded, err := base64Std.DecodeString(value); err == nil {
		return decoded, nil
	}
	return base64Raw.DecodeString(value)
}
