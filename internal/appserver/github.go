package appserver

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	defaultAPIBaseURL    = "https://api.github.com"
	appJWTLifetime       = 10 * time.Minute
	installationTokenTTL = 55 * time.Minute
	tokenRefreshMargin   = 5 * time.Minute
)

// gitHubConfig carries the App's identity and the injectable API base URL.
type gitHubConfig struct {
	APIBaseURL    string
	AppID         int64
	PrivateKeyPEM []byte
}

// githubClient implements the App-side adapters over the GitHub REST API.
// The zero value is not usable; construct with newGitHubClient.
type githubClient struct {
	baseURL    string
	appID      int64
	privateKey *rsa.PrivateKey
	httpClient *http.Client

	mu                 sync.Mutex
	installationTokens map[int64]installationToken
}

type installationToken struct {
	Token     string
	ExpiresAt time.Time
}

func newGitHubClient(config gitHubConfig) (*githubClient, error) {
	base := strings.TrimRight(config.APIBaseURL, "/")
	if base == "" {
		base = defaultAPIBaseURL
	}
	key, err := parsePrivateKey(config.PrivateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("app auth: %w", err)
	}
	return &githubClient{
		baseURL:            base,
		appID:              config.AppID,
		privateKey:         key,
		httpClient:         &http.Client{Timeout: 30 * time.Second},
		installationTokens: make(map[int64]installationToken),
	}, nil
}

func parsePrivateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("private key: no PEM block")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("private key: unsupported format")
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key: not RSA")
	}
	return key, nil
}

// appJWT signs the installation-authentication JWT (RS256, iss = app id,
// short-lived per GitHub's guidance).
func appJWT(now func() time.Time, appID int64, privateKeyPEM []byte) (string, error) {
	key, err := parsePrivateKey(privateKeyPEM)
	if err != nil {
		return "", err
	}
	issued := now().UTC()
	if issued.IsZero() {
		issued = time.Now().UTC()
	}
	header := base64URL([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims := base64URL([]byte(fmt.Sprintf(
		`{"iss":%q,"iat":%d,"exp":%d}`,
		fmt.Sprintf("%d", appID), issued.Unix(), issued.Add(appJWTLifetime).Unix(),
	)))
	signingInput := header + "." + claims
	sum := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", fmt.Errorf("app auth: sign jwt: %w", err)
	}
	return signingInput + "." + base64URL(signature), nil
}

// installationToken returns a valid installation access token, caching per
// installation until the refresh margin.
func (c *githubClient) installationToken(ctx context.Context, installation int64) (string, error) {
	c.mu.Lock()
	cached, ok := c.installationTokens[installation]
	c.mu.Unlock()
	if ok && time.Until(cached.ExpiresAt) > tokenRefreshMargin {
		return cached.Token, nil
	}

	jwt, err := appJWT(func() time.Time { return time.Now().UTC() }, c.appID, privateKeyToPEM(c.privateKey))
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/app/installations/%d/access_tokens", c.baseURL, installation), nil)
	if err != nil {
		return "", fmt.Errorf("app auth: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+jwt)
	request.Header.Set("Accept", "application/vnd.github+json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("app auth: exchange installation token: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		// The response body may contain secrets; only the status is reported.
		_, _ = io.Copy(io.Discard, response.Body)
		return "", fmt.Errorf("app auth: installation token exchange failed with status %d", response.StatusCode)
	}
	var granted struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.NewDecoder(response.Body).Decode(&granted); err != nil {
		return "", fmt.Errorf("app auth: installation token response: %w", err)
	}
	if granted.Token == "" {
		return "", fmt.Errorf("app auth: installation token response carried no token")
	}
	expires := time.Now().Add(installationTokenTTL)
	if granted.ExpiresAt != "" {
		if parsed, parseErr := time.Parse(time.RFC3339, granted.ExpiresAt); parseErr == nil {
			expires = parsed
		}
	}

	c.mu.Lock()
	c.installationTokens[installation] = installationToken{Token: granted.Token, ExpiresAt: expires}
	c.mu.Unlock()
	return granted.Token, nil
}

func privateKeyToPEM(key *rsa.PrivateKey) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
}

func base64URL(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

const evidenceArtifactName = "assurectl-evidence"

// PostCheck implements CheckPoster: it posts the check run for the pull
// request's head SHA using an installation token.
func (c *githubClient) PostCheck(ctx context.Context, installation int64, repository string, prNumber int64, headSHA string, payload CheckPayload) error {
	token, err := c.installationToken(ctx, installation)
	if err != nil {
		return fmt.Errorf("app: post check: %w", err)
	}
	var body struct {
		Name       string `json:"name"`
		HeadSHA    string `json:"head_sha"`
		Conclusion string `json:"conclusion"`
		Output     struct {
			Title   string `json:"title"`
			Summary string `json:"summary"`
		} `json:"output"`
	}
	body.Name = payload.Name
	body.HeadSHA = headSHA
	body.Conclusion = payload.Conclusion
	body.Output.Title = payload.Title
	body.Output.Summary = payload.Summary

	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("app: post check: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/repos/%s/check-runs", c.baseURL, repository), bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("app: post check: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/vnd.github+json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("app: post check: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		_, _ = io.Copy(io.Discard, response.Body)
		return fmt.Errorf("app: post check: failed with status %d", response.StatusCode)
	}
	return nil
}

// EvidenceBundle implements EvidenceSource: it finds the evidence artifact
// for the head revision, downloads and materializes it into a bundle
// directory, and returns the CI OIDC token relayed inside the bundle.
func (c *githubClient) EvidenceBundle(ctx context.Context, installation int64, repository string, prNumber int64, headSHA string) (string, string, error) {
	token, err := c.installationToken(ctx, installation)
	if err != nil {
		return "", "", fmt.Errorf("app: evidence bundle: %w", err)
	}

	runID, err := c.headRunID(ctx, token, repository, headSHA)
	if err != nil {
		return "", "", fmt.Errorf("app: evidence bundle: %w", err)
	}
	artifactID, err := c.evidenceArtifactID(ctx, token, repository, runID)
	if err != nil {
		return "", "", fmt.Errorf("app: evidence bundle: %w", err)
	}
	archive, err := c.downloadArtifact(ctx, token, repository, artifactID)
	if err != nil {
		return "", "", fmt.Errorf("app: evidence bundle: %w", err)
	}

	bundle, err := os.MkdirTemp("", "assurectl-evidence-")
	if err != nil {
		return "", "", fmt.Errorf("app: evidence bundle: %w", err)
	}
	ciToken, err := materializeEvidenceBundle(bundle, archive)
	if err != nil {
		_ = os.RemoveAll(bundle)
		return "", "", fmt.Errorf("app: evidence bundle: %w", err)
	}
	return bundle, ciToken, nil
}

func (c *githubClient) headRunID(ctx context.Context, token, repository, headSHA string) (int64, error) {
	var listing struct {
		Runs []struct {
			ID int64 `json:"id"`
		} `json:"workflow_runs"`
	}
	if err := c.getJSON(ctx, token, fmt.Sprintf("/repos/%s/actions/runs?head_sha=%s", repository, headSHA), &listing); err != nil {
		return 0, err
	}
	if len(listing.Runs) == 0 {
		return 0, fmt.Errorf("no workflow run for head %s", headSHA)
	}
	return listing.Runs[0].ID, nil
}

func (c *githubClient) evidenceArtifactID(ctx context.Context, token, repository string, runID int64) (int64, error) {
	var listing struct {
		Artifacts []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"artifacts"`
	}
	if err := c.getJSON(ctx, token, fmt.Sprintf("/repos/%s/actions/runs/%d/artifacts", repository, runID), &listing); err != nil {
		return 0, err
	}
	for _, artifact := range listing.Artifacts {
		if artifact.Name == evidenceArtifactName {
			return artifact.ID, nil
		}
	}
	return 0, fmt.Errorf("artifact %q not found for run %d", evidenceArtifactName, runID)
}

func (c *githubClient) downloadArtifact(ctx context.Context, token, repository string, artifactID int64) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/repos/%s/actions/artifacts/%d/zip", c.baseURL, repository, artifactID), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, response.Body)
		return nil, fmt.Errorf("artifact download failed with status %d", response.StatusCode)
	}
	return io.ReadAll(io.LimitReader(response.Body, 64<<20))
}

func (c *githubClient) getJSON(ctx context.Context, token, path string, dst any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, response.Body)
		return fmt.Errorf("GET %s failed with status %d", path, response.StatusCode)
	}
	return json.NewDecoder(response.Body).Decode(dst)
}

// materializeEvidenceBundle extracts the evidence archive into
// <bundle>/.assurectl/evidence/ and returns the relayed CI OIDC token,
// rejecting any archive entry that escapes the bundle (zip-slip).
func materializeEvidenceBundle(bundle string, archive []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return "", fmt.Errorf("evidence archive: %w", err)
	}
	evidenceRoot := filepath.Join(bundle, ".assurectl", "evidence")
	ciToken := ""
	for _, file := range reader.File {
		clean := filepath.Clean(file.Name)
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
			return "", fmt.Errorf("evidence archive: entry %q escapes the bundle", file.Name)
		}
		target := filepath.Join(evidenceRoot, filepath.FromSlash(clean))
		if !strings.HasPrefix(target, evidenceRoot+string(filepath.Separator)) && target != evidenceRoot {
			return "", fmt.Errorf("evidence archive: entry %q escapes the bundle", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", err
		}
		source, err := file.Open()
		if err != nil {
			return "", err
		}
		destination, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			source.Close()
			return "", err
		}
		_, copyErr := io.Copy(destination, io.LimitReader(source, 64<<20))
		closeErr := destination.Close()
		source.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		if file.Name == "ci-token.txt" {
			content, readErr := os.ReadFile(target)
			if readErr != nil {
				return "", readErr
			}
			ciToken = strings.TrimSpace(string(content))
		}
	}
	return ciToken, nil
}
