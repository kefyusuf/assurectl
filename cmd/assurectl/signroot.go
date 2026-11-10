package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/kefyusuf/assurectl/internal/attestation"
)

// runSignRoot signs a trusted-root payload document (the slice-1 root
// shape) with an Ed25519 root key, producing a signed-root document.
func runSignRoot(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("sign-root", flag.ContinueOnError)
	keyPath := flags.String("key", "", "path to the Ed25519 root key PEM (PKCS#8)")
	keyID := flags.String("key-id", "", "root key id recorded in the envelope")
	in := flags.String("in", "", "path to the root payload document")
	out := flags.String("out", "", "path for the signed root")
	if err := flags.Parse(args); err != nil {
		return rootUsageError(err.Error(), stderr)
	}
	if *keyPath == "" || *keyID == "" || *in == "" || *out == "" {
		return rootUsageError("--key, --key-id, --in and --out are required", stderr)
	}

	payload, err := os.ReadFile(*in)
	if err != nil {
		return rootError("read root payload", err, stderr)
	}
	var probe struct {
		RootVersion int `json:"root_version"`
	}
	if err := json.Unmarshal(payload, &probe); err != nil {
		return rootError("parse root payload", err, stderr)
	}
	if probe.RootVersion < 1 {
		return rootError("parse root payload: root_version must be positive", nil, stderr)
	}

	keyPEM, err := os.ReadFile(*keyPath)
	if err != nil {
		return rootError("read key", err, stderr)
	}
	private, err := parseEd25519Key(keyPEM)
	if err != nil {
		return rootError("parse key", err, stderr)
	}

	envelope := attestation.Envelope{
		PayloadType: attestation.RootPayloadTypeV1,
		Payload:     base64.StdEncoding.EncodeToString(payload),
	}
	signed, err := attestation.SignEnvelope(envelope, *keyID, private)
	if err != nil {
		return rootError("sign", err, stderr)
	}
	// The envelope carries the version inside its payload; re-add it at the
	// top level for callers persisting state.
	documentBytes, err := signed.MarshalDocument()
	if err != nil {
		return rootError("encode signed root", err, stderr)
	}
	var document map[string]any
	if err := json.Unmarshal(documentBytes, &document); err != nil {
		return rootError("encode signed root", err, stderr)
	}
	document["root_version"] = probe.RootVersion
	encoded, err := json.Marshal(document)
	if err != nil {
		return rootError("encode signed root", err, stderr)
	}
	if err := os.WriteFile(*out, encoded, 0o600); err != nil {
		return rootError("write signed root", err, stderr)
	}
	fmt.Fprintf(stdout, "signed trusted root version %d\n", probe.RootVersion)
	return 0
}

func rootError(stage string, err error, stderr io.Writer) int {
	if err != nil {
		fmt.Fprintf(stderr, "sign-root: %s: %v\n", stage, err)
	} else {
		fmt.Fprintf(stderr, "sign-root: %s\n", stage)
	}
	return 2
}

func rootUsageError(message string, stderr io.Writer) int {
	fmt.Fprintf(stderr, "sign-root: %s\n\n", message)
	_, _ = io.WriteString(stderr, usage)
	return 64
}
