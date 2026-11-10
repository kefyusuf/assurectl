package attestation

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"time"
)

// RootPin is the verifier's out-of-band initial trust: one root key id and
// its material, configured like the reference action pins a commit SHA.
type RootPin struct {
	KeyID     string
	PublicKey ed25519.PublicKey
}

// AcceptedRoot is the verifier's state after accepting a signed root: the
// parsed root plus the root keys trusted for future rotations.
type AcceptedRoot struct {
	Root        TrustedRoot
	RootVersion int
	RootKeys    map[string]ed25519.PublicKey
}

// AcceptSignedRoot applies the acceptance rules (ADR: issuer lifecycle):
//
//  1. version monotonicity — a root with version <= the accepted version is
//     a rollback and is rejected;
//  2. at least one signature by a root key already trusted (the pin, or a
//     root key a previously accepted root introduced);
//  3. if the root's key set changed, at least one additional signature by a
//     key that the new root itself declares (new keys cannot self-introduce;
//     co-signed new keys become trusted for future rotations);
//  4. the payload must parse as a trusted root.
//
// The returned AcceptedRoot carries the parsed root (which then governs
// receipt-issuer resolution) and the updated root-key set.
func AcceptSignedRoot(signed SignedRoot, pin RootPin, acceptedVersion int, acceptedRootKeys map[string]ed25519.PublicKey, at time.Time) (AcceptedRoot, error) {
	if signed.RootVersion <= acceptedVersion {
		return AcceptedRoot{}, fmt.Errorf(
			"signed root: version %d does not exceed the accepted version %d (rollback)",
			signed.RootVersion, acceptedVersion,
		)
	}

	root, err := signed.ParseRootPayload()
	if err != nil {
		return AcceptedRoot{}, fmt.Errorf("signed root: %w", err)
	}

	// Collect the candidate signer keys: the pin, the root keys trusted so
	// far, and the keys the new root itself declares.
	candidates := map[string]ed25519.PublicKey{}
	for keyID, key := range acceptedRootKeys {
		candidates[keyID] = key
	}
	if pin.KeyID != "" {
		candidates[pin.KeyID] = pin.PublicKey
	}
	newPayloadKeys := map[string]ed25519.PublicKey{}
	for _, issuer := range root.Issuers {
		for _, key := range issuer.Keys {
			if raw, err := base64StdDecode(key.Ed25519Public); err == nil && len(raw) == ed25519.PublicKeySize {
				newPayloadKeys[key.KeyID] = ed25519.PublicKey(raw)
			}
		}
	}
	for keyID, key := range newPayloadKeys {
		candidates[keyID] = key
	}

	verifiedBy := map[string]bool{}
	for _, signature := range signed.Signatures {
		public, ok := candidates[signature.KeyID]
		if !ok {
			continue
		}
		payload, err := signed.DecodePayload()
		if err != nil {
			return AcceptedRoot{}, fmt.Errorf("signed root: %w", err)
		}
		signatureBytes, err := base64RawDecode(signature.Sig)
		if err != nil {
			return AcceptedRoot{}, fmt.Errorf("signed root: signature %q: %w", signature.KeyID, err)
		}
		if ed25519.Verify(public, PAE([]byte(RootPayloadTypeV1), payload), signatureBytes) {
			verifiedBy[signature.KeyID] = true
		}
	}

	oldTrustedHit := false
	for keyID := range verifiedBy {
		if _, trusted := acceptedRootKeys[keyID]; trusted || keyID == pin.KeyID {
			oldTrustedHit = true
		}
	}
	if !oldTrustedHit {
		return AcceptedRoot{}, fmt.Errorf("signed root: no signature by a root key trusted in version %d", acceptedVersion)
	}

	// The verified signers become the root keys trusted for future
	// rotations: only keys that co-signed this accepted root. A root signed
	// only by keys that were never trusted fails the oldTrustedHit check
	// above (self-introduction).
	newVersionKeys := map[string]ed25519.PublicKey{}
	for keyID := range verifiedBy {
		newVersionKeys[keyID] = candidates[keyID]
	}

	return AcceptedRoot{Root: root, RootVersion: signed.RootVersion, RootKeys: newVersionKeys}, nil
}

func base64StdDecode(value string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(value)
}

func base64RawDecode(value string) ([]byte, error) {
	return base64.RawStdEncoding.DecodeString(value)
}

// resolveIssuerKey resolves a receipt-signing issuer key against this root
// at the given instant, fail-closed (slice-1 logic, reused).
func (r TrustedRoot) resolveIssuerKey(keyID string, at time.Time) (ed25519.PublicKey, error) {
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
			raw, err := base64StdDecode(key.Ed25519Public)
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
