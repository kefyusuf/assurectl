package strictjson

import (
	"strings"
	"testing"
)

type strictFixture struct {
	SchemaVersion string              `json:"schema_version"`
	Nested        strictNestedFixture `json:"nested"`
}

type strictNestedFixture struct {
	Value string `json:"value"`
}

func TestDecodeRequiresExactJSONFieldNames(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{
			name: "top level case alias",
			json: `{"Schema_Version":"v0","nested":{"value":"ok"}}`,
		},
		{
			name: "nested case alias",
			json: `{"schema_version":"v0","nested":{"Value":"ok"}}`,
		},
		{
			name: "exact field plus case alias",
			json: `{"schema_version":"trusted","Schema_Version":"override","nested":{"value":"ok"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got strictFixture
			err := Decode([]byte(tt.json), &got)
			if err == nil {
				t.Fatal("Decode() error = nil")
			}
			if !strings.Contains(err.Error(), "unknown field") {
				t.Fatalf("Decode() error = %q, want unknown field", err)
			}
		})
	}
}

func TestDecodeAcceptsExactJSONFieldNames(t *testing.T) {
	var got strictFixture
	if err := Decode([]byte(`{"schema_version":"v0","nested":{"value":"ok"}}`), &got); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if got.SchemaVersion != "v0" || got.Nested.Value != "ok" {
		t.Fatalf("Decode() result = %#v", got)
	}
}
