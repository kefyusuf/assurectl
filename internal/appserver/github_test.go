package appserver

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fixturePEM(t *testing.T) ([]byte, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der := x509.MarshalPKCS1PrivateKey(key)
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}
	return pem.EncodeToMemory(block), key
}

func TestAppJWTShape(t *testing.T) {
	t.Parallel()

	pemBytes, key := fixturePEM(t)
	jwt, err := appJWT(time.Now, 42, pemBytes)
	if err != nil {
		t.Fatalf("appJWT() error = %v", err)
	}

	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Fatalf("jwt has %d parts, want 3", len(parts))
	}
	var header struct {
		Alg string `json:"alg"`
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		t.Fatal(err)
	}
	if header.Alg != "RS256" {
		t.Fatalf("alg = %q, want RS256", header.Alg)
	}

	var claims struct {
		Iss string `json:"iss"`
		Iat int64  `json:"iat"`
		Exp int64  `json:"exp"`
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		t.Fatal(err)
	}
	if claims.Iss != "42" {
		t.Fatalf("iss = %q, want the app id", claims.Iss)
	}
	if claims.Exp-claims.Iat < 500 || claims.Exp-claims.Iat > 900 {
		t.Fatalf("jwt lifetime = %d seconds, want a short window", claims.Exp-claims.Iat)
	}

	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, sum[:], mustDecodeURL(t, parts[2])); err != nil {
		t.Fatalf("jwt signature: %v", err)
	}
}

func mustDecodeURL(t *testing.T, value string) []byte {
	t.Helper()
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestInstallationTokenExchangeAndCache(t *testing.T) {
	t.Parallel()

	pemBytes, _ := fixturePEM(t)
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if want := "/app/installations/7/access_tokens"; r.URL.Path != want {
			t.Errorf("path = %s, want %s", r.URL.Path, want)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ey") {
			t.Errorf("authorization = %q, want a bearer jwt", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "ghs_fixture", "expires_at": time.Now().Add(time.Hour).Format(time.RFC3339)})
	}))
	defer server.Close()

	client, err := newGitHubClient(gitHubConfig{
		APIBaseURL:    server.URL,
		AppID:         42,
		PrivateKeyPEM: pemBytes,
	})
	if err != nil {
		t.Fatalf("newGitHubClient() error = %v", err)
	}

	first, err := client.installationToken(context.Background(), 7)
	if err != nil {
		t.Fatalf("installationToken() error = %v", err)
	}
	if first != "ghs_fixture" {
		t.Fatalf("token = %q", first)
	}
	second, err := client.installationToken(context.Background(), 7)
	if err != nil {
		t.Fatalf("installationToken(cached) error = %v", err)
	}
	if second != "ghs_fixture" {
		t.Fatalf("cached token = %q", second)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("fixture server saw %d requests, want 1 (cached)", got)
	}
}

func TestInstallationTokenErrorHidesSecrets(t *testing.T) {
	t.Parallel()

	pemBytes, _ := fixturePEM(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"boom ghs_secret_value"}`))
	}))
	defer server.Close()

	client, err := newGitHubClient(gitHubConfig{APIBaseURL: server.URL, AppID: 42, PrivateKeyPEM: pemBytes})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.installationToken(context.Background(), 7); err == nil {
		t.Fatalf("installationToken() accepted a server error")
	} else if strings.Contains(err.Error(), "ghs_secret_value") {
		t.Fatalf("error leaks the response body: %v", err)
	}
}
