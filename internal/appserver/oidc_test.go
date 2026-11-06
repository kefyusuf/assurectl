package appserver

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

// fixtureKeySource serves one in-process RSA public key.
type fixtureKeySource struct{ key *rsa.PublicKey }

func (f fixtureKeySource) Key(kid string) (interface{}, error) {
	if kid != "test-key" {
		return nil, errUnknownKey
	}
	return f.key, nil
}

func signFixtureToken(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	header := mustBase64URL(t, map[string]string{"alg": "RS256", "typ": "JWT", "kid": "test-key"})
	payload := mustBase64URL(t, claims)
	signingInput := header + "." + payload
	sum := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func mustBase64URL(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

func TestOIDCValidatorAcceptsBindingClaims(t *testing.T) {
	t.Parallel()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	validator := OIDCValidator{
		Issuer:   "https://token.actions.githubusercontent.com",
		Audience: "assurectl-app",
		Keys:     fixtureKeySource{key: &key.PublicKey},
	}
	token := signFixtureToken(t, key, map[string]any{
		"iss":          "https://token.actions.githubusercontent.com",
		"aud":          "assurectl-app",
		"repository":   "acme/checkout",
		"sha":          "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"workflow_ref": "acme/checkout/.github/workflows/ci.yml@refs/pull/184/merge",
		"workflow_sha": "2a5165d0f3f9bbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"exp":          time.Now().Add(time.Hour).Unix(),
	})

	claims, err := validator.Validate(context.Background(), token, ExpectedClaims{
		Repository: "github.com/acme/checkout",
		SHA:        "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	})
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if claims.SHA != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" || claims.Repository != "acme/checkout" {
		t.Fatalf("claims = %#v", claims)
	}
}

func TestOIDCValidatorRejectsBadTokens(t *testing.T) {
	t.Parallel()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	validator := OIDCValidator{
		Issuer:   "https://token.actions.githubusercontent.com",
		Audience: "assurectl-app",
		Keys:     fixtureKeySource{key: &key.PublicKey},
	}

	validClaims := func(overrides map[string]any) map[string]any {
		claims := map[string]any{
			"iss":        "https://token.actions.githubusercontent.com",
			"aud":        "assurectl-app",
			"repository": "acme/checkout",
			"sha":        "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			"exp":        time.Now().Add(time.Hour).Unix(),
		}
		for name, value := range overrides {
			claims[name] = value
		}
		return claims
	}

	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "wrong signature key",
			token: signFixtureToken(t, otherKey, validClaims(nil)),
		},
		{
			name:  "expired",
			token: signFixtureToken(t, key, validClaims(map[string]any{"exp": time.Now().Add(-time.Minute).Unix()})),
		},
		{
			name:  "wrong issuer",
			token: signFixtureToken(t, key, validClaims(map[string]any{"iss": "https://evil.example"})),
		},
		{
			name:  "wrong audience",
			token: signFixtureToken(t, key, validClaims(map[string]any{"aud": "other-app"})),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := validator.Validate(context.Background(), tt.token, ExpectedClaims{
				Repository: "github.com/acme/checkout",
				SHA:        "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			}); err == nil {
				t.Fatalf("Validate() accepted a bad token")
			}
		})
	}

	t.Run("empty token is rejected", func(t *testing.T) {
		t.Parallel()

		if _, err := validator.Validate(context.Background(), "", ExpectedClaims{}); err == nil {
			t.Fatalf("Validate() accepted an empty token")
		}
	})
}
