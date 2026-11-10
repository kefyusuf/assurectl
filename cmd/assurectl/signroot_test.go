package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const signedRootFixture = `{
  "root_version": 2,
  "payloadType": "https://assurectl.dev/trusted-root/v1",
  "payload": "PLACEHOLDER",
  "signatures": [{"keyid": "root-key", "sig": "PLACEHOLDER"}]
}`

func TestSignRootProducesVerifiableRoot(t *testing.T) {
	t.Parallel()

	rootPayloadPath := tempFile(t, "root-payload.json", `{
  "root_version": 2,
  "issuers": [{"id": "issuer-1", "keys": [{"keyid": "ik", "ed25519_public": "AAAA"}], "valid": {"from": "2026-01-01T00:00:00Z", "until": "2027-01-01T00:00:00Z"}}]
}`)
	keyPath := tempFile(t, "root-key.pem", "")
	_, _, seedPEM := fixtureKeyPair(t)
	if err := os.WriteFile(keyPath, []byte(seedPEM), 0o600); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(t.TempDir(), "signed-root.json")

	stderr := &strings.Builder{}
	if code := run([]string{
		"sign-root",
		"--key", keyPath,
		"--key-id", "root-key",
		"--in", rootPayloadPath,
		"--out", outPath,
	}, &strings.Builder{}, stderr); code != 0 {
		t.Fatalf("sign-root exit = %d, stderr: %s", code, stderr.String())
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("signed root not written: %v", err)
	}
}
