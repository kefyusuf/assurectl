package appserver

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const fixtureSecret = "whsec-fixture"

func signBody(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func pullRequestPayload(action string) []byte {
	return []byte(`{"action":"` + action + `","installation":{"id":7},"repository":{"full_name":"acme/checkout"},"number":184,` +
		`"pull_request":{"base":{"sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},"head":{"sha":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}}}`)
}

func TestWebhookRejectsBadSignatures(t *testing.T) {
	t.Parallel()

	handler := NewHandler(Dependencies{WebhookSecret: []byte(fixtureSecret)})
	body := pullRequestPayload("opened")

	tests := []struct {
		name    string
		headers map[string]string
	}{
		{name: "missing signature", headers: map[string]string{}},
		{name: "malformed signature", headers: map[string]string{"X-Hub-Signature-256": "not-hex"}},
		{name: "wrong secret", headers: map[string]string{"X-Hub-Signature-256": signBody("other-secret", body)}},
		{name: "signature for a different body", headers: map[string]string{"X-Hub-Signature-256": signBody(fixtureSecret, []byte(`{"action":"closed"}`))}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(body)))
			for name, value := range tt.headers {
				request.Header.Set(name, value)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", recorder.Code)
			}
		})
	}
}
