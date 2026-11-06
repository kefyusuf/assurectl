package appserver

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixtureEvidenceZip builds an in-memory zip archive with the given files.
func fixtureEvidenceZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	buffer := &bytes.Buffer{}
	writer := zip.NewWriter(buffer)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestCheckPosterPostsPayload(t *testing.T) {
	t.Parallel()

	pemBytes, _ := fixturePEM(t)
	var seenPath string
	var seenBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/app/installations/7/access_tokens" {
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"token": "ghs_fixture", "expires_at": time.Now().Add(time.Hour).Format(time.RFC3339)})
			return
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ghs_") {
			t.Errorf("authorization = %q, want an installation token", r.Header.Get("Authorization"))
		}
		seenPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &seenBody)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client, err := NewGitHubClient(GitHubConfig{APIBaseURL: server.URL, AppID: 42, PrivateKeyPEM: pemBytes})
	if err != nil {
		t.Fatal(err)
	}
	payload := CheckPayload{Name: "assurectl", Conclusion: "success", Title: "t", Summary: "s",
		ReceiptReference: ReceiptReference{ChangeSetDigest: strings.Repeat("a", 64), ReceiptSHA256: strings.Repeat("b", 64), Authority: "AUTHORITATIVE"}}

	if err := client.PostCheck(context.Background(), 7, "acme/checkout", 184, strings.Repeat("b", 40), payload); err != nil {
		t.Fatalf("PostCheck() error = %v", err)
	}
	if seenPath != "/repos/acme/checkout/check-runs" {
		t.Fatalf("path = %q", seenPath)
	}
	if seenBody["name"] != "assurectl" || seenBody["conclusion"] != "success" {
		t.Fatalf("body = %#v", seenBody)
	}
}

func TestEvidenceSourceMaterializesBundle(t *testing.T) {
	t.Parallel()

	pemBytes, _ := fixturePEM(t)
	zipBytes := fixtureEvidenceZip(t, map[string]string{
		"unit-tests.json":  `{"schema_version":"assurectl/evidence-envelope/v0"}`,
		"artifacts/ci.log": "log",
		"ci-token.txt":     "oidc-token-value",
	})
	paths := map[string]string{
		"/repos/acme/checkout/actions/runs":                `{"workflow_runs":[{"id":9001}]}`,
		"/repos/acme/checkout/actions/runs/9001/artifacts": `{"artifacts":[{"id":55,"name":"assurectl-evidence"}]}`,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/app/installations/7/access_tokens" {
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"token": "ghs_fixture", "expires_at": time.Now().Add(time.Hour).Format(time.RFC3339)})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/zip") {
			_, _ = w.Write(zipBytes)
			return
		}
		if body, ok := paths[r.URL.Path]; ok {
			_, _ = w.Write([]byte(body))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client, err := NewGitHubClient(GitHubConfig{APIBaseURL: server.URL, AppID: 42, PrivateKeyPEM: pemBytes})
	if err != nil {
		t.Fatal(err)
	}

	bundle, ciToken, err := client.EvidenceBundle(context.Background(), 7, "acme/checkout", 184, strings.Repeat("b", 40))
	if err != nil {
		t.Fatalf("EvidenceBundle() error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(bundle, ".assurectl", "evidence", "unit-tests.json")); statErr != nil {
		t.Fatalf("bundle layout wrong: %v", statErr)
	}
	if ciToken != "oidc-token-value" {
		t.Fatalf("ciToken = %q", ciToken)
	}
}

func TestEvidenceSourceRejectsZipTraversal(t *testing.T) {
	t.Parallel()

	pemBytes, _ := fixturePEM(t)
	zipBytes := fixtureEvidenceZip(t, map[string]string{
		"../../../../etc/escaped": "boom",
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/app/installations/7/access_tokens" {
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"token": "ghs_fixture", "expires_at": time.Now().Add(time.Hour).Format(time.RFC3339)})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/zip") {
			_, _ = w.Write(zipBytes)
			return
		}
		_, _ = w.Write([]byte(`{"workflow_runs":[{"id":9001}]}`))
	}))
	defer server.Close()

	client, err := NewGitHubClient(GitHubConfig{APIBaseURL: server.URL, AppID: 42, PrivateKeyPEM: pemBytes})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := client.EvidenceBundle(context.Background(), 7, "acme/checkout", 184, strings.Repeat("b", 40)); err == nil {
		t.Fatalf("EvidenceBundle() accepted a zip-slip archive")
	}
}
