package appserver

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func fixtureJWKS(key *rsa.PublicKey) string {
	return `{"keys":[{"kty":"RSA","kid":"test-kid","alg":"RS256","use":"sig","n":"` +
		base64.RawURLEncoding.EncodeToString(key.N.Bytes()) +
		`","e":"` + base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()) + `"}]}`
}

func TestJWKSKeySourceServesKeys(t *testing.T) {
	t.Parallel()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var fetches int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureJWKS(&key.PublicKey)))
	}))
	defer server.Close()

	source := jwksKeySource{wellKnown: server.URL + "/.well-known/jwks", client: &http.Client{Timeout: 30 * time.Second}}

	if _, err := source.Key("test-kid"); err != nil {
		t.Fatalf("Key() error = %v", err)
	}
	if got := fetches; got != 1 {
		t.Fatalf("JWKS fetches = %d, want 1 (cached)", got)
	}
	if _, err := source.Key("missing-kid"); err == nil {
		t.Fatalf("Key() accepted an unknown kid without error")
	}
}

func TestJWKSKeySourceRefreshesOnUnknownKid(t *testing.T) {
	t.Parallel()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var fetches int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches++
		if fetches == 1 {
			_, _ = w.Write([]byte(fixtureJWKS(&key.PublicKey)))
			return
		}
		_, _ = w.Write([]byte(`{"keys":[{"kty":"RSA","kid":"rotated-kid","alg":"RS256","use":"sig","n":"` +
			base64.RawURLEncoding.EncodeToString(rotated.N.Bytes()) +
			`","e":"` + base64.RawURLEncoding.EncodeToString(big.NewInt(int64(rotated.E)).Bytes()) + `"}]}`))
	}))
	defer server.Close()

	source := jwksKeySource{wellKnown: server.URL + "/.well-known/jwks", client: &http.Client{Timeout: 30 * time.Second}}

	if _, err := source.Key("test-kid"); err != nil {
		t.Fatalf("Key(first) error = %v", err)
	}
	// The issuer rotated to a new key id; the unknown kid forces exactly one
	// refresh, after which the rotated key is served.
	fetched, err := source.Key("rotated-kid")
	if err != nil {
		t.Fatalf("Key(rotated) error = %v", err)
	}
	publicKey, ok := fetched.(*rsa.PublicKey)
	if !ok {
		t.Fatalf("key type = %T", fetched)
	}
	if publicKey.N.Cmp(rotated.N) != 0 {
		t.Fatalf("refreshed key is not the rotated key")
	}
	if fetches != 2 {
		t.Fatalf("JWKS fetches = %d, want exactly one refresh", fetches)
	}
	if _, err := source.Key("never-present-kid"); err == nil {
		t.Fatalf("Key() accepted an unknown kid after the refresh")
	}
}

func TestJWKSKeySourceRejectsMalformedDocument(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"keys": "not-an-array"}`))
	}))
	defer server.Close()

	source := jwksKeySource{wellKnown: server.URL + "/.well-known/jwks", client: &http.Client{Timeout: 30 * time.Second}}
	if _, err := source.Key("test-kid"); err == nil {
		t.Fatalf("Key() accepted a malformed JWKS document")
	}
}
