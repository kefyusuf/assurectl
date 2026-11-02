package receipt

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/kefyusuf/assurectl/internal/evidence"
	"github.com/kefyusuf/assurectl/internal/inputmeta"
)

func TestBuildIsByteIdenticalAcrossRuns(t *testing.T) {
	t.Parallel()

	first, err := Build(buildInput())
	if err != nil {
		t.Fatalf("Build(first) error = %v", err)
	}
	second, err := Build(buildInput())
	if err != nil {
		t.Fatalf("Build(second) error = %v", err)
	}

	firstBytes, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatalf("repeated builds differ in bytes")
	}
}

func TestBuildShuffledEvidenceIsByteIdentical(t *testing.T) {
	t.Parallel()

	ordered := buildInput()
	reordered := buildInput()
	reordered.Evidence = []evidence.Loaded{
		ordered.Evidence[1],
		ordered.Evidence[0],
	}

	orderedReceipt, err := Build(ordered)
	if err != nil {
		t.Fatalf("Build(ordered) error = %v", err)
	}
	reorderedReceipt, err := Build(reordered)
	if err != nil {
		t.Fatalf("Build(reordered) error = %v", err)
	}

	orderedBytes, err := json.Marshal(orderedReceipt)
	if err != nil {
		t.Fatal(err)
	}
	reorderedBytes, err := json.Marshal(reorderedReceipt)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(orderedBytes, reorderedBytes) {
		t.Fatalf("shuffled evidence input changed receipt bytes")
	}
}

func TestReceiptDigestMatchesEmittedBytes(t *testing.T) {
	t.Parallel()

	built, err := Build(buildInput())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	data, err := built.JSON()
	if err != nil {
		t.Fatalf("JSON() error = %v", err)
	}
	digest, err := built.Digest()
	if err != nil {
		t.Fatalf("Digest() error = %v", err)
	}
	if !reflect.DeepEqual(digest, inputmeta.SHA256(data)) {
		t.Fatalf("Digest() = %#v, want the digest of the emitted bytes", digest)
	}
}
