package attestation

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// SignEnvelope returns a copy of the envelope carrying one Ed25519
// signature over the DSSE PAE under the given key id.
func SignEnvelope(envelope Envelope, keyID string, private ed25519.PrivateKey) (Envelope, error) {
	payload, err := envelope.DecodePayload()
	if err != nil {
		return Envelope{}, fmt.Errorf("sign: %w", err)
	}
	pae := PAE([]byte(envelope.PayloadType), payload)
	signature := ed25519.Sign(private, pae)

	signed := envelope
	signed.Signatures = append([]Signature(nil), envelope.Signatures...)
	signed.Signatures = append(signed.Signatures, Signature{
		KeyID: keyID,
		Sig:   base64.StdEncoding.EncodeToString(signature),
	})
	return signed, nil
}

// TrustedKey is one Ed25519 public key in the trusted root.
type TrustedKey struct {
	KeyID         string `json:"keyid"`
	Ed25519Public string `json:"ed25519_public"`
}

// ValidityWindow bounds when an issuer's keys may be used.
type ValidityWindow struct {
	From  string `json:"from"`
	Until string `json:"until"`
}

// TrustedIssuer is one accepted receipt issuer and its keys.
type TrustedIssuer struct {
	ID    string         `json:"id"`
	Keys  []TrustedKey   `json:"keys"`
	Valid ValidityWindow `json:"valid"`
}

// TrustedRoot is the caller-supplied trust anchor: the signed receipt is
// worth exactly the root the verifier was configured with.
type TrustedRoot struct {
	RootVersion int             `json:"root_version"`
	Issuers     []TrustedIssuer `json:"issuers"`
}

// ParseTrustedRoot parses and validates a trusted-root document.
func ParseTrustedRoot(document []byte) (TrustedRoot, error) {
	var root TrustedRoot
	if err := json.Unmarshal(document, &root); err != nil {
		return TrustedRoot{}, fmt.Errorf("trusted root: parse: %w", err)
	}
	if root.RootVersion != 1 {
		return TrustedRoot{}, fmt.Errorf("trusted root: root_version %d is unsupported", root.RootVersion)
	}
	if len(root.Issuers) == 0 {
		return TrustedRoot{}, fmt.Errorf("trusted root: at least one issuer is required")
	}
	return root, nil
}

// resolveKey finds the one valid Ed25519 public key matching the key id at
// the given instant, fail-closed.
func (r TrustedRoot) resolveKey(keyID string, at time.Time) (ed25519.PublicKey, error) {
	matches := 0
	var resolved ed25519.PublicKey
	for _, issuer := range r.Issuers {
		from, err := time.Parse(time.RFC3339, issuer.Valid.From)
		if err != nil {
			return nil, fmt.Errorf("trusted root: issuer %q valid.from: %w", issuer.ID, err)
		}
		until, err := time.Parse(time.RFC3339, issuer.Valid.Until)
		if err != nil {
			return nil, fmt.Errorf("trusted root: issuer %q valid.until: %w", issuer.ID, err)
		}
		if at.Before(from) || at.After(until) {
			continue
		}
		for _, key := range issuer.Keys {
			if key.KeyID != keyID {
				continue
			}
			raw, err := base64.StdEncoding.DecodeString(key.Ed25519Public)
			if err != nil {
				return nil, fmt.Errorf("trusted root: issuer %q key %q: %w", issuer.ID, key.KeyID, err)
			}
			if len(raw) != ed25519.PublicKeySize {
				return nil, fmt.Errorf("trusted root: issuer %q key %q: wrong key size", issuer.ID, key.KeyID)
			}
			matches++
			resolved = ed25519.PublicKey(raw)
		}
	}
	if matches == 0 {
		return nil, fmt.Errorf("trusted root: no valid issuer key for keyid %q at %s", keyID, at.Format(time.RFC3339))
	}
	if matches > 1 {
		return nil, fmt.Errorf("trusted root: %d issuer keys match keyid %q", matches, keyID)
	}
	return resolved, nil
}

// VerifyEnvelope verifies exactly one Ed25519 signature over the DSSE PAE
// against the trusted root at the given instant, fail-closed.
func VerifyEnvelope(envelope Envelope, root TrustedRoot, at time.Time) error {
	payload, err := envelope.DecodePayload()
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}

	verified := 0
	var lastErr error
	for _, signature := range envelope.Signatures {
		public, err := root.resolveKey(signature.KeyID, at)
		if err != nil {
			lastErr = err
			continue
		}
		sig, err := base64.StdEncoding.DecodeString(signature.Sig)
		if err != nil {
			lastErr = fmt.Errorf("verify: signature decode: %w", err)
			continue
		}
		pae := PAE([]byte(envelope.PayloadType), payload)
		if ed25519.Verify(public, pae, sig) {
			verified++
		} else {
			lastErr = fmt.Errorf("verify: signature for keyid %q does not verify", signature.KeyID)
		}
	}
	if verified != 1 || len(envelope.Signatures) != 1 {
		if lastErr != nil {
			return fmt.Errorf("verify: %w", lastErr)
		}
		return fmt.Errorf("verify: exactly one valid signature is required, got %d of %d", verified, len(envelope.Signatures))
	}
	return nil
}
