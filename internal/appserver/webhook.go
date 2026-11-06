package appserver

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Dependencies wires the App's ports to the offline-testable core. The real
// adapters arrive in a later slice; fixtures implement every port in tests.
type Dependencies struct {
	WorktreeSource WorktreeSource
	EvidenceSource EvidenceSource
	TokenValidator TokenValidator
	CheckPoster    CheckPoster
	WebhookSecret  []byte
	Now            func() time.Time
}

// NewHandler builds the webhook surface. Signature verification happens
// before anything else; unsigned or mismatched requests are rejected.
func NewHandler(deps Dependencies) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBodyBytes))
		if err != nil {
			http.Error(w, "read body", http.StatusBadRequest)
			return
		}
		if !verifySignature(deps.WebhookSecret, body, r.Header.Get("X-Hub-Signature-256")) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}

		eventType := r.Header.Get("X-GitHub-Event")
		if eventType != "pull_request" {
			// Other events are ignored, never evaluated.
			w.WriteHeader(http.StatusOK)
			return
		}

		var event PullRequestEvent
		if err := json.Unmarshal(body, &event); err != nil {
			http.Error(w, "parse event", http.StatusBadRequest)
			return
		}
		if event.Action != "opened" && event.Action != "synchronize" {
			w.WriteHeader(http.StatusOK)
			return
		}

		outcome, err := EvaluatePullRequest(r.Context(), event, deps)
		if err != nil {
			// Diagnostics name the stage, never secret material.
			fmt.Fprintf(w, "evaluate: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(outcome.Payload)
	})
}

const maxWebhookBodyBytes = 1 << 20

func verifySignature(secret, body []byte, header string) bool {
	if !strings.HasPrefix(header, "sha256=") {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(header))
}
