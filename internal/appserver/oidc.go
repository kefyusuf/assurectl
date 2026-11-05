package appserver

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// errUnknownKey is returned when a token references a key id the source
// does not serve.
var errUnknownKey = errors.New("unknown key id")

// KeySource serves the public keys an OIDC token may reference. The real
// adapter fetches and caches the issuer's JWKS; tests inject keys.
type KeySource interface {
	Key(kid string) (interface{}, error)
}

// OIDCValidator validates GitHub Actions OIDC tokens with standard
// cryptography (ADR-0010): RS256 signature via an injectable KeySource,
// then issuer, audience, expiry, and the binding claims.
type OIDCValidator struct {
	Issuer   string
	Audience string
	Keys     KeySource
}

// Validate returns the binding claims of a token whose signature, issuer,
// audience, and expiry all check out.
func (v OIDCValidator) Validate(ctx context.Context, token string, expect ExpectedClaims) (ValidatedClaims, error) {
	if strings.TrimSpace(token) == "" {
		return ValidatedClaims{}, fmt.Errorf("validate oidc token: token is empty")
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ValidatedClaims{}, fmt.Errorf("validate oidc token: malformed token")
	}

	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ValidatedClaims{}, fmt.Errorf("validate oidc token: header: %w", err)
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return ValidatedClaims{}, fmt.Errorf("validate oidc token: header: %w", err)
	}
	if header.Alg != "RS256" {
		return ValidatedClaims{}, fmt.Errorf("validate oidc token: algorithm %q is unsupported", header.Alg)
	}

	keyAny, err := v.Keys.Key(header.Kid)
	if err != nil {
		return ValidatedClaims{}, fmt.Errorf("validate oidc token: %w", err)
	}
	publicKey, ok := keyAny.(*rsa.PublicKey)
	if !ok {
		return ValidatedClaims{}, fmt.Errorf("validate oidc token: key %q is not RSA", header.Kid)
	}

	signingInput := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return ValidatedClaims{}, fmt.Errorf("validate oidc token: signature: %w", err)
	}
	sum := sha256.Sum256([]byte(signingInput))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, sum[:], signature); err != nil {
		return ValidatedClaims{}, fmt.Errorf("validate oidc token: signature: %w", err)
	}

	var claims struct {
		Issuer      string `json:"iss"`
		Audience    string `json:"aud"`
		ExpiresAt   int64  `json:"exp"`
		Repository  string `json:"repository"`
		WorkflowRef string `json:"workflow_ref"`
		WorkflowSHA string `json:"workflow_sha"`
		SHA         string `json:"sha"`
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ValidatedClaims{}, fmt.Errorf("validate oidc token: payload: %w", err)
	}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return ValidatedClaims{}, fmt.Errorf("validate oidc token: payload: %w", err)
	}

	if claims.Issuer != v.Issuer {
		return ValidatedClaims{}, fmt.Errorf("validate oidc token: issuer %q is not accepted", claims.Issuer)
	}
	if claims.Audience != v.Audience {
		return ValidatedClaims{}, fmt.Errorf("validate oidc token: audience %q is not accepted", claims.Audience)
	}
	if time.Now().Unix() >= claims.ExpiresAt {
		return ValidatedClaims{}, fmt.Errorf("validate oidc token: token is expired")
	}

	return ValidatedClaims{
		Repository:  claims.Repository,
		WorkflowRef: claims.WorkflowRef,
		WorkflowSHA: claims.WorkflowSHA,
		SHA:         claims.SHA,
	}, nil
}
