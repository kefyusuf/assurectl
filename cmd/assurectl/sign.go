package main

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/kefyusuf/assurectl/internal/attestation"
)

// runSignReceipt wraps a receipt document as an in-toto statement, signs it
// in a DSSE envelope with an Ed25519 key, and writes the attestation.
func runSignReceipt(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("sign-receipt", flag.ContinueOnError)
	keyPath := flags.String("key", "", "path to the Ed25519 private key PEM (PKCS#8)")
	keyID := flags.String("key-id", "", "key id recorded in the envelope")
	in := flags.String("in", "", "path to the receipt document")
	out := flags.String("out", "", "path for the signed attestation")
	if err := flags.Parse(args); err != nil {
		return signUsageError(err.Error(), stderr)
	}
	if *keyPath == "" || *keyID == "" || *in == "" || *out == "" {
		return signUsageError("--key, --key-id, --in and --out are required", stderr)
	}

	receiptDocument, err := os.ReadFile(*in)
	if err != nil {
		return signError("read receipt", err, stderr)
	}
	var subject struct {
		Subject struct {
			ChangeSetDigest string `json:"change_set_digest"`
		} `json:"subject"`
	}
	if err := json.Unmarshal(receiptDocument, &subject); err != nil {
		return signError("parse receipt", err, stderr)
	}
	if subject.Subject.ChangeSetDigest == "" {
		return signError("parse receipt: change_set_digest is missing", nil, stderr)
	}

	keyPEM, err := os.ReadFile(*keyPath)
	if err != nil {
		return signError("read key", err, stderr)
	}
	private, err := parseEd25519Key(keyPEM)
	if err != nil {
		return signError("parse key", err, stderr)
	}

	statement, err := attestation.WrapReceipt(receiptDocument, subject.Subject.ChangeSetDigest)
	if err != nil {
		return signError("wrap receipt", err, stderr)
	}
	statementJSON, err := json.Marshal(statement)
	if err != nil {
		return signError("encode statement", err, stderr)
	}
	envelope := attestation.NewEnvelope(attestation.PayloadTypeReceiptV0, statementJSON)
	signed, err := attestation.SignEnvelope(envelope, *keyID, private)
	if err != nil {
		return signError("sign", err, stderr)
	}
	document, err := signed.MarshalDocument()
	if err != nil {
		return signError("encode attestation", err, stderr)
	}
	if err := os.WriteFile(*out, document, 0o600); err != nil {
		return signError("write attestation", err, stderr)
	}
	fmt.Fprintf(stdout, "signed receipt for change set %s\n", subject.Subject.ChangeSetDigest)
	return 0
}

func parseEd25519Key(pemBytes []byte) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("no PEM block")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is not Ed25519")
	}
	return key, nil
}

func signError(stage string, err error, stderr io.Writer) int {
	if err != nil {
		fmt.Fprintf(stderr, "sign-receipt: %s: %v\n", stage, err)
	} else {
		fmt.Fprintf(stderr, "sign-receipt: %s\n", stage)
	}
	return 2
}

func signUsageError(message string, stderr io.Writer) int {
	fmt.Fprintf(stderr, "sign-receipt: %s\n\n", message)
	_, _ = io.WriteString(stderr, usage)
	return 64
}
