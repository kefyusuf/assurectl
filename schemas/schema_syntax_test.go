package schemas_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestSchemaRootsAreStrictValidJSON(t *testing.T) {
	paths, err := filepath.Glob("*.schema.json")
	if err != nil {
		t.Fatalf("filepath.Glob() error = %v", err)
	}
	sort.Strings(paths)
	if len(paths) != 4 {
		t.Fatalf("schema count = %d, want 4: %v", len(paths), paths)
	}

	for _, path := range paths {
		path := path
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("os.ReadFile(%q) error = %v", path, err)
			}

			var root map[string]any
			if err := json.Unmarshal(data, &root); err != nil {
				t.Fatalf("json.Unmarshal(%q) error = %v", path, err)
			}
			if got, ok := root["$schema"].(string); !ok || got == "" {
				t.Fatalf("%s: $schema = %#v, want non-empty string", path, root["$schema"])
			}
			if got, ok := root["$id"].(string); !ok || got == "" {
				t.Fatalf("%s: $id = %#v, want non-empty string", path, root["$id"])
			}
			if got := root["type"]; got != "object" {
				t.Fatalf("%s: type = %#v, want object", path, got)
			}
			if got, ok := root["additionalProperties"].(bool); !ok || got {
				t.Fatalf("%s: additionalProperties = %#v, want false", path, root["additionalProperties"])
			}
		})
	}
}
