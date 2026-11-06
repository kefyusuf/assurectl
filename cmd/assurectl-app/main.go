// Command assurectl-app runs the reference AssureCTL GitHub App skeleton:
// a webhook endpoint that verifies GitHub signatures, routes pull_request
// events, and posts check runs through the offline-tested appserver core.
// Hosting, registration, and operations are managed scope (ADR-0010).
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/kefyusuf/assurectl/internal/appserver"
	"github.com/kefyusuf/assurectl/internal/gitsubject"
)

// appConfig carries the parsed configuration for the App process.
type appConfig struct {
	AppID          int64
	PrivateKeyPEM  []byte
	APIBaseURL     string
	RemoteTemplate string
	OIDCAudience   string
}

// buildDependencies wires the real GitHub adapters when the configuration
// enables them; without --api-base-url the fail-closed skeleton (nil ports)
// stays in place, matching the slice-4 behavior.
func buildDependencies(config appConfig) (appserver.Dependencies, error) {
	if config.APIBaseURL == "" {
		return appserver.Dependencies{}, nil
	}
	if config.RemoteTemplate == "" {
		return appserver.Dependencies{}, fmt.Errorf("app wiring: --remote-template is required with --api-base-url")
	}

	client, err := appserver.NewGitHubClient(appserver.GitHubConfig{
		APIBaseURL:    config.APIBaseURL,
		AppID:         config.AppID,
		PrivateKeyPEM: config.PrivateKeyPEM,
	})
	if err != nil {
		return appserver.Dependencies{}, fmt.Errorf("app wiring: %w", err)
	}

	audience := config.OIDCAudience
	if audience == "" {
		audience = fmt.Sprintf("%d", config.AppID)
	}
	return appserver.Dependencies{
		WorktreeSource: remoteTemplateWorktreeSource{template: config.RemoteTemplate, cache: defaultWorktreeCache()},
		EvidenceSource: client,
		CheckPoster:    client,
		TokenValidator: appserver.OIDCValidator{
			Issuer:   "https://token.actions.githubusercontent.com",
			Audience: audience,
			Keys:     appserver.NewJWKSKeySource("https://token.actions.githubusercontent.com/.well-known/jwks"),
		},
	}, nil
}

// defaultWorktreeCache returns the directory under the process working
// directory where fetched worktrees are cached.
func defaultWorktreeCache() string {
	directory, err := os.MkdirTemp("", "assurectl-worktrees-")
	if err != nil {
		return ""
	}
	return directory
}

// remoteTemplateWorktreeSource applies the configured remote template to
// each repository before delegating to the vetted Git path.
type remoteTemplateWorktreeSource struct {
	template string
	cache    string
}

func (s remoteTemplateWorktreeSource) Worktree(ctx context.Context, installation int64, repository, revision string) (string, error) {
	remote := fmt.Sprintf(s.template, repository)
	return gitsubject.FetchWorktree(ctx, remote, revision, s.cache)
}

func main() {
	os.Exit(run(os.Args[1:]))
}

// run parses configuration and starts the HTTP server. Configuration comes
// from flags; secret material is read from files and never logged.
func run(args []string) int {
	flags := flag.NewFlagSet("assurectl-app", flag.ContinueOnError)
	appID := flags.String("app-id", "", "GitHub App numeric id")
	privateKeyPath := flags.String("private-key-path", "", "path to the App private key PEM")
	webhookSecretPath := flags.String("webhook-secret-path", "", "path to the webhook secret file")
	apiBaseURL := flags.String("api-base-url", "", "GitHub API base URL (empty keeps the fail-closed skeleton mode)")
	remoteTemplate := flags.String("remote-template", "", "remote URL template for worktrees, e.g. https://github.com/%s.git")
	oidcAudience := flags.String("oidc-audience", "", "OIDC audience the App accepts (defaults to the app id)")
	listen := flags.String("listen", ":8080", "listen address")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "assurectl-app: %v\n", err)
		return 64
	}
	if *appID == "" || *privateKeyPath == "" || *webhookSecretPath == "" {
		fmt.Fprintln(os.Stderr, "assurectl-app: --app-id, --private-key-path and --webhook-secret-path are required")
		return 64
	}
	appIDNumber, err := strconv.ParseInt(*appID, 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "assurectl-app: --app-id must be numeric\n")
		return 64
	}

	secret, err := os.ReadFile(*webhookSecretPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "assurectl-app: read webhook secret: %v\n", err)
		return 2
	}

	var privateKeyPEM []byte
	if *apiBaseURL != "" {
		privateKeyPEM, err = os.ReadFile(*privateKeyPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "assurectl-app: read private key: %v\n", err)
			return 2
		}
	}

	dependencies, err := buildDependencies(appConfig{
		AppID:          appIDNumber,
		PrivateKeyPEM:  privateKeyPEM,
		APIBaseURL:     *apiBaseURL,
		RemoteTemplate: *remoteTemplate,
		OIDCAudience:   *oidcAudience,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "assurectl-app: %v\n", err)
		return 2
	}
	dependencies.WebhookSecret = secret

	handler := appserver.NewHandler(dependencies)

	server := &http.Server{
		Addr:              *listen,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "assurectl-app: %v\n", err)
		return 2
	}
	return 0
}
