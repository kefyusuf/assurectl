package schemas_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
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

func TestReceiptPolicyRecordsAllResolvedSources(t *testing.T) {
	data, err := os.ReadFile("receipt.v0.schema.json")
	if err != nil {
		t.Fatalf("os.ReadFile(receipt schema) error = %v", err)
	}

	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("json.Unmarshal(receipt schema) error = %v", err)
	}

	properties := mustObject(t, root["properties"], "properties")
	policy := mustObject(t, properties["policy"], "properties.policy")
	if got := policy["$ref"]; got != "#/$defs/resolved_policy" {
		t.Fatalf("properties.policy.$ref = %#v, want #/$defs/resolved_policy", got)
	}

	defs := mustObject(t, root["$defs"], "$defs")
	resolvedPolicy := mustObject(t, defs["resolved_policy"], "$defs.resolved_policy")
	assertRequiredFields(t, resolvedPolicy, "sources", "effective_digest", "trust_status")

	resolvedProperties := mustObject(t, resolvedPolicy["properties"], "$defs.resolved_policy.properties")
	sources := mustObject(t, resolvedProperties["sources"], "$defs.resolved_policy.properties.sources")
	if got := sources["type"]; got != "array" {
		t.Fatalf("resolved policy sources type = %#v, want array", got)
	}
	if got, ok := sources["minItems"].(float64); !ok || got < 1 {
		t.Fatalf("resolved policy sources minItems = %#v, want at least 1", sources["minItems"])
	}
	items := mustObject(t, sources["items"], "$defs.resolved_policy.properties.sources.items")
	if got := items["$ref"]; got != "#/$defs/policy_source" {
		t.Fatalf("resolved policy source item $ref = %#v, want #/$defs/policy_source", got)
	}

	policySource := mustObject(t, defs["policy_source"], "$defs.policy_source")
	assertRequiredFields(t, policySource, "layer", "source", "digest", "trust_status", "authority_basis")
}

func mustObject(t *testing.T, value any, path string) map[string]any {
	t.Helper()
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s = %#v, want object", path, value)
	}
	return object
}

func assertRequiredFields(t *testing.T, schema map[string]any, fields ...string) {
	t.Helper()
	requiredRaw, ok := schema["required"].([]any)
	if !ok {
		t.Fatalf("required = %#v, want array", schema["required"])
	}
	required := make(map[string]bool, len(requiredRaw))
	for _, item := range requiredRaw {
		name, ok := item.(string)
		if !ok {
			t.Fatalf("required item = %#v, want string", item)
		}
		required[name] = true
	}
	for _, field := range fields {
		if !required[field] {
			t.Errorf("required fields missing %q", field)
		}
	}
}

func TestReceiptConstrainsVerdictDecisionMappings(t *testing.T) {
	root := readSchema(t, "receipt.v0.schema.json")
	rulesRaw, ok := root["allOf"].([]any)
	if !ok {
		t.Fatalf("receipt allOf = %#v, want array", root["allOf"])
	}

	got := make(map[string][]string)
	for index, raw := range rulesRaw {
		rule := mustObject(t, raw, "allOf rule")
		condition := mustObject(t, rule["if"], "allOf.if")
		conditionProperties := mustObject(t, condition["properties"], "allOf.if.properties")
		verdictSchema := mustObject(t, conditionProperties["verification_verdict"], "verification_verdict condition")
		verdict, ok := verdictSchema["const"].(string)
		if !ok || verdict == "" {
			t.Fatalf("allOf[%d] verdict const = %#v, want non-empty string", index, verdictSchema["const"])
		}

		consequence := mustObject(t, rule["then"], "allOf.then")
		consequenceProperties := mustObject(t, consequence["properties"], "allOf.then.properties")
		decisionSchema := mustObject(t, consequenceProperties["completion_decision"], "completion_decision consequence")

		switch {
		case decisionSchema["const"] != nil:
			decision, ok := decisionSchema["const"].(string)
			if !ok || decision == "" {
				t.Fatalf("allOf[%d] decision const = %#v, want non-empty string", index, decisionSchema["const"])
			}
			got[verdict] = []string{decision}
		case decisionSchema["enum"] != nil:
			values, ok := decisionSchema["enum"].([]any)
			if !ok {
				t.Fatalf("allOf[%d] decision enum = %#v, want array", index, decisionSchema["enum"])
			}
			for _, value := range values {
				decision, ok := value.(string)
				if !ok {
					t.Fatalf("allOf[%d] decision enum item = %#v, want string", index, value)
				}
				got[verdict] = append(got[verdict], decision)
			}
			sort.Strings(got[verdict])
		default:
			t.Fatalf("allOf[%d] completion_decision has neither const nor enum", index)
		}
	}

	want := map[string][]string{
		"PASSED":        {"APPROVED"},
		"INDETERMINATE": {"BLOCKED"},
		"FAILED":        {"ACCEPTED_WITH_RISK", "REJECTED"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("verdict-decision mappings = %#v, want %#v", got, want)
	}
}

func TestEvidenceTimestampPatternsRejectMalformedValues(t *testing.T) {
	root := readSchema(t, "evidence-envelope.v0.schema.json")
	properties := mustObject(t, root["properties"], "properties")
	invocation := mustObject(t, properties["invocation"], "properties.invocation")
	invocationProperties := mustObject(t, invocation["properties"], "properties.invocation.properties")

	valid := []string{
		"2026-09-02T09:10:00Z",
		"2026-09-02T09:10:00.123456789+03:00",
	}
	invalid := []string{
		"not-a-time",
		"2026-13-02T09:10:00Z",
		"2026-09-02 09:10:00Z",
		"2026-09-02T25:10:00Z",
		"2026-09-02T09:10:00+25:00",
	}

	for _, field := range []string{"started_at", "finished_at"} {
		fieldSchema := mustObject(t, invocationProperties[field], "invocation timestamp")
		if got := fieldSchema["format"]; got != "date-time" {
			t.Fatalf("%s format = %#v, want date-time", field, got)
		}
		pattern, ok := fieldSchema["pattern"].(string)
		if !ok || pattern == "" {
			t.Fatalf("%s pattern = %#v, want non-empty string", field, fieldSchema["pattern"])
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			t.Fatalf("regexp.Compile(%s) error = %v", field, err)
		}
		for _, value := range valid {
			if !re.MatchString(value) {
				t.Errorf("%s pattern rejected valid lexical RFC3339 value %q", field, value)
			}
		}
		for _, value := range invalid {
			if re.MatchString(value) {
				t.Errorf("%s pattern accepted malformed value %q", field, value)
			}
		}
	}
}

func readSchema(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", path, err)
	}
	return root
}
