// Command assurectl-app runs the reference AssureCTL GitHub App skeleton:
// a webhook endpoint that verifies GitHub signatures, routes pull_request
// events, and posts check runs through the offline-tested appserver core.
// Hosting, registration, and operations are managed scope (ADR-0010).
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/kefyusuf/assurectl/internal/appserver"
)

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
	listen := flags.String("listen", ":8080", "listen address")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "assurectl-app: %v\n", err)
		return 64
	}
	if *appID == "" || *privateKeyPath == "" || *webhookSecretPath == "" {
		fmt.Fprintln(os.Stderr, "assurectl-app: --app-id, --private-key-path and --webhook-secret-path are required")
		return 64
	}
	if _, err := strconv.ParseInt(*appID, 10, 64); err != nil {
		fmt.Fprintf(os.Stderr, "assurectl-app: --app-id must be numeric\n")
		return 64
	}

	secret, err := os.ReadFile(*webhookSecretPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "assurectl-app: read webhook secret: %v\n", err)
		return 2
	}

	// The real adapters (installation tokens, artifact download, check
	// posting, JWKS) are wired in a later slice; the skeleton serves the
	// webhook surface over the offline-tested core with nil ports, which
	// fail closed on any pull_request evaluation attempt.
	handler := appserver.NewHandler(appserver.Dependencies{
		WebhookSecret: secret,
	})

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
