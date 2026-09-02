package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run(nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run() exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Usage: assurectl <command>") {
		t.Fatalf("stdout = %q, want usage", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"version"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run() exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "assurectl ") {
		t.Fatalf("stdout = %q, want version summary", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"unknown"}, &stdout, &stderr)

	if code != 64 {
		t.Fatalf("run() exit code = %d, want 64", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "unknown command: unknown") {
		t.Fatalf("stderr = %q, want unknown command diagnostic", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Usage: assurectl <command>") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestRunRejectsTrailingArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"version", "unexpected"}, &stdout, &stderr)

	if code != 64 {
		t.Fatalf("run() exit code = %d, want 64", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "unexpected argument: unexpected") {
		t.Fatalf("stderr = %q, want trailing-argument diagnostic", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Usage: assurectl <command>") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}
