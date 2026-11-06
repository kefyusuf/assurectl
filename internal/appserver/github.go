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
	"fmt"
	"io"
	"net/http"
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
