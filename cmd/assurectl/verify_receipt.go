package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/kefyusuf/assurectl/internal/attestation"
)

// runVerifyReceipt verifies a signed attestation against a trusted root,
// re-checks the receipt semantics, applies the optional replay window, and
// prints the embedded receipt on success.
func runVerifyReceipt(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("verify-receipt", flag.ContinueOnError)
	rootPath := flags.String("trusted-root", "", "path to the trusted-root document")
	in := flags.String("in", "", "path to the signed attestation")
	maxAge := flags.Duration("max-age", 0, "optional maximum age of the receipt's evaluated_at (e.g. 720h)")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintf(stderr, "verify-receipt: %v\n\n", err)
		_, _ = io.WriteString(stderr, usage)
		return 64
	}
	if *rootPath == "" || *in == "" {
		fmt.Fprintln(stderr, "verify-receipt: --trusted-root and --in are required")
		_, _ = io.WriteString(stderr, usage)
		return 64
	}

	rootDocument, err := os.ReadFile(*rootPath)
	if err != nil {
		return verifyError("read trusted root", err, stderr)
	}
	root, err := attestation.ParseTrustedRoot(rootDocument)
	if err != nil {
		return verifyError("parse trusted root", err, stderr)
	}

	attestationDocument, err := os.ReadFile(*in)
	if err != nil {
		return verifyError("read attestation", err, stderr)
	}
	envelope, err := attestation.ParseEnvelope(attestationDocument)
	if err != nil {
		return verifyError("parse attestation", err, stderr)
	}
	if envelope.PayloadType != attestation.PayloadTypeReceiptV0 {
		return verifyError("payload type", fmt.Errorf("%q is not a receipt attestation", envelope.PayloadType), stderr)
	}
	if err := attestation.VerifyEnvelope(envelope, root, time.Now()); err != nil {
		return verifyError("verify signature", err, stderr)
	}

	payload, err := envelope.DecodePayload()
	if err != nil {
		return verifyError("decode payload", err, stderr)
	}
	var statement struct {
		Predicate json.RawMessage `json:"predicate"`
	}
	if err := json.Unmarshal(payload, &statement); err != nil {
		return verifyError("decode statement", err, stderr)
	}

	var receiptHeader struct {
		SchemaVersion string `json:"schema_version"`
		Subject       struct {
			ChangeSetDigest string `json:"change_set_digest"`
		} `json:"subject"`
		EvaluatedAt string `json:"evaluated_at"`
	}
	if err := json.Unmarshal(statement.Predicate, &receiptHeader); err != nil {
		return verifyError("decode receipt predicate", err, stderr)
	}
	if receiptHeader.SchemaVersion != "assurectl/receipt/v0" {
		return verifyError("predicate", fmt.Errorf("schema_version %q is not a receipt v0", receiptHeader.SchemaVersion), stderr)
	}
	if *maxAge > 0 {
		evaluatedAt, err := time.Parse(time.RFC3339, receiptHeader.EvaluatedAt)
		if err != nil {
			return verifyError("receipt evaluated_at", err, stderr)
		}
		if time.Since(evaluatedAt) > *maxAge {
			return verifyError("replay window", fmt.Errorf("receipt is older than %s", *maxAge), stderr)
		}
	}

	if _, err := stdout.Write(statement.Predicate); err != nil {
		return verifyError("write receipt", err, stderr)
	}
	fmt.Fprintln(stdout)
	return 0
}

func verifyError(stage string, err error, stderr io.Writer) int {
	if err != nil {
		fmt.Fprintf(stderr, "verify-receipt: %s: %v\n", stage, err)
	} else {
		fmt.Fprintf(stderr, "verify-receipt: %s\n", stage)
	}
	return 2
}
