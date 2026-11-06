package attestation

import (
	"testing"
)

func TestPAEEncodesExactlyPerSpecification(t *testing.T) {
	t.Parallel()

	got := string(PAE([]byte("https://example.com/TestStatement/v1"), []byte("hello")))
	want := "DSSEv1 36 https://example.com/TestStatement/v1 5 hello"
	if got != want {
		t.Fatalf("PAE = %q, want %q", got, want)
	}
}

func TestPAEHandlesEmptyAndBinaryValues(t *testing.T) {
	t.Parallel()

	if got, want := string(PAE(nil, nil)), "DSSEv1 0  0 "; got != want {
		t.Fatalf("PAE(empty) = %q, want %q", got, want)
	}
	binary := []byte{0x00, 0x01, 0xff}
	got := PAE([]byte("t"), binary)
	want := string([]byte("DSSEv1 1 t 3 \x00\x01\xff"))
	if string(got) != want {
		t.Fatalf("PAE(binary) = %q, want %q", got, want)
	}
}
