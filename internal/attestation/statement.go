package attestation

import (
	"encoding/json"
	"fmt"
)

// InTotoStatementType is the in-toto statement _type for this project.
const InTotoStatementType = "https://in-toto.io/Statement/v1"

// StatementSubject names the change set the statement is bound to.
type StatementSubject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

// Statement is the in-toto Statement v1 carrying a receipt as its
// predicate.
type Statement struct {
	Type          string             `json:"_type"`
	Subject       []StatementSubject `json:"subject"`
	PredicateType string             `json:"predicateType"`
	Predicate     json.RawMessage    `json:"predicate"`
}

// WrapReceipt wraps a receipt document as an in-toto statement bound to the
// receipt's subject change set. The predicate carries the receipt document
// unchanged; the caller passes the already-parsed receipt JSON.
func WrapReceipt(receiptDocument []byte, changeSetDigest string) (Statement, error) {
	var probe struct {
		Subject struct {
			RepositoryURI string `json:"repository_uri"`
		} `json:"subject"`
	}
	if err := json.Unmarshal(receiptDocument, &probe); err != nil {
		return Statement{}, fmt.Errorf("statement: receipt document: %w", err)
	}
	if probe.Subject.RepositoryURI == "" {
		return Statement{}, fmt.Errorf("statement: receipt document lacks subject.repository_uri")
	}

	statement := Statement{
		Type: InTotoStatementType,
		Subject: []StatementSubject{{
			Name:   probe.Subject.RepositoryURI,
			Digest: map[string]string{"gitChangeSet": changeSetDigest},
		}},
		PredicateType: PayloadTypeReceiptV0,
		Predicate:     append(json.RawMessage(nil), receiptDocument...),
	}
	return statement, nil
}
