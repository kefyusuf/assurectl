package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
)

func fixturePrivateKeyPEM(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
}

func TestBuildDependenciesKeepsSkeletonWhenUnconfigured(t *testing.T) {
	t.Parallel()

	deps, err := buildDependencies(appConfig{AppID: 1})
	if err != nil {
		t.Fatalf("buildDependencies() error = %v", err)
	}
	if deps.WorktreeSource != nil || deps.EvidenceSource != nil || deps.CheckPoster != nil || deps.TokenValidator != nil {
		t.Fatalf("unconfigured mode must keep the fail-closed skeleton (nil ports)")
	}
}

func TestBuildDependenciesWiresRealAdapters(t *testing.T) {
	t.Parallel()

	deps, err := buildDependencies(appConfig{
		AppID:          42,
		PrivateKeyPEM:  fixturePrivateKeyPEM(t),
		APIBaseURL:     "https://api.example.invalid",
		RemoteTemplate: "https://github.com/%s.git",
		OIDCAudience:   "assurectl-app",
	})
	if err != nil {
		t.Fatalf("buildDependencies() error = %v", err)
	}
	if deps.WorktreeSource == nil || deps.EvidenceSource == nil || deps.CheckPoster == nil || deps.TokenValidator == nil {
		t.Fatalf("configured mode left ports unwired: %#v", deps)
	}
}

func TestBuildDependenciesRequiresRemoteTemplate(t *testing.T) {
	t.Parallel()

	if _, err := buildDependencies(appConfig{
		AppID:         42,
		PrivateKeyPEM: fixturePrivateKeyPEM(t),
		APIBaseURL:    "https://api.example.invalid",
	}); err == nil {
		t.Fatalf("buildDependencies() accepted real mode without a remote template")
	}
}

func TestBuildDependenciesRejectsBadKey(t *testing.T) {
	t.Parallel()

	if _, err := buildDependencies(appConfig{
		AppID:          42,
		PrivateKeyPEM:  []byte("not a pem"),
		APIBaseURL:     "https://api.example.invalid",
		RemoteTemplate: "https://github.com/%s.git",
	}); err == nil {
		t.Fatalf("buildDependencies() accepted an unusable private key")
	}
}
