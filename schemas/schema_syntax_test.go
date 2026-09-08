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
	root := readSchema(t, "receipt.v0.schema.json")
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

func TestReceiptConstrainsVerdictDecisionMappings(t *testing.T) {
	root := readSchema(t, "receipt.v0.schema.json")
	rulesRaw := mustArray(t, root["allOf"], "allOf")

	got := make(map[string][]string)
	for index, raw := range rulesRaw {
		rule := mustObject(t, raw, "allOf rule")
		condition, ok := rule["if"].(map[string]any)
		if !ok {
			continue
		}
		conditionProperties, ok := condition["properties"].(map[string]any)
		if !ok {
			continue
		}
		verdictSchema, ok := conditionProperties["verification_verdict"].(map[string]any)
		if !ok {
			continue
		}
		verdict, ok := verdictSchema["const"].(string)
		if !ok || verdict == "" {
			continue
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
			values := mustArray(t, decisionSchema["enum"], "completion_decision.enum")
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
		"FAILED":        {"ACCEPTED_WITH_RISK", "BLOCKED", "REJECTED"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("verdict-decision mappings = %#v, want %#v", got, want)
	}
}

func TestReceiptApprovalRequiresPassingEstablishedRequirements(t *testing.T) {
	root := readSchema(t, "receipt.v0.schema.json")
	rule := findConditionalRule(t, root, "completion_decision", "APPROVED")
	then := mustObject(t, rule["then"], "approved.then")
	properties := mustObject(t, then["properties"], "approved.then.properties")
	requirementResults := mustObject(t, properties["requirement_results"], "approved.requirement_results")
	items := mustObject(t, requirementResults["items"], "approved.requirement_results.items")
	itemProperties := mustObject(t, items["properties"], "approved.requirement_results.items.properties")
	assertConst(t, itemProperties["evidence_state"], "VALID", "approved evidence_state")
	assertConst(t, itemProperties["outcome"], "PASSED", "approved outcome")
	assertRequiredFields(t, items, "evidence_state", "outcome")
}

func TestAuthoritativeNonBlockedReceiptsRequireTrustedInputs(t *testing.T) {
	root := readSchema(t, "receipt.v0.schema.json")
	rule := findConditionalRule(t, root, "authority", "AUTHORITATIVE")
	condition := mustObject(t, rule["if"], "authoritative.if")
	conditionProperties := mustObject(t, condition["properties"], "authoritative.if.properties")
	decision := mustObject(t, conditionProperties["completion_decision"], "authoritative completion_decision")
	not := mustObject(t, decision["not"], "authoritative completion_decision.not")
	if got := not["const"]; got != "BLOCKED" {
		t.Fatalf("authoritative non-blocked condition = %#v, want BLOCKED exclusion", got)
	}

	then := mustObject(t, rule["then"], "authoritative.then")
	thenProperties := mustObject(t, then["properties"], "authoritative.then.properties")
	for _, input := range []string{"contract", "policy"} {
		inputSchema := mustObject(t, thenProperties[input], "authoritative "+input)
		inputProperties := mustObject(t, inputSchema["properties"], "authoritative "+input+".properties")
		assertConst(t, inputProperties["trust_status"], "TRUSTED", "authoritative "+input+" trust_status")
		assertRequiredFields(t, inputSchema, "trust_status")
	}
}

func TestRequirementResultValidWaiverRequiresWaivableFailure(t *testing.T) {
	root := readSchema(t, "receipt.v0.schema.json")
	defs := mustObject(t, root["$defs"], "$defs")
	requirementResult := mustObject(t, defs["requirement_result"], "$defs.requirement_result")
	rules := mustArray(t, requirementResult["allOf"], "$defs.requirement_result.allOf")

	var matched map[string]any
	for _, raw := range rules {
		rule := mustObject(t, raw, "requirement_result rule")
		condition, ok := rule["if"].(map[string]any)
		if !ok {
			continue
		}
		properties, ok := condition["properties"].(map[string]any)
		if !ok {
			continue
		}
		waiverStatus, ok := properties["waiver_status"].(map[string]any)
		if ok && waiverStatus["const"] == "VALID" {
			matched = rule
			break
		}
	}
	if matched == nil {
		t.Fatal("requirement_result has no conditional for waiver_status VALID")
	}

	then := mustObject(t, matched["then"], "valid waiver.then")
	properties := mustObject(t, then["properties"], "valid waiver.then.properties")
	assertConst(t, properties["waivable"], true, "valid waiver waivable")
	assertConst(t, properties["evidence_state"], "VALID", "valid waiver evidence_state")
	assertConst(t, properties["outcome"], "FAILED", "valid waiver outcome")
	assertRequiredFields(t, then, "waivable", "evidence_state", "outcome")
}

func TestReceiptRequiresAtLeastOneRequirementResult(t *testing.T) {
	root := readSchema(t, "receipt.v0.schema.json")
	properties := mustObject(t, root["properties"], "properties")
	requirementResults := mustObject(t, properties["requirement_results"], "properties.requirement_results")
	if got, ok := requirementResults["minItems"].(float64); !ok || got != 1 {
		t.Fatalf("requirement_results.minItems = %#v, want 1", requirementResults["minItems"])
	}
}

func TestReceiptFindingPreservesCategory(t *testing.T) {
	root := readSchema(t, "receipt.v0.schema.json")
	defs := mustObject(t, root["$defs"], "$defs")
	finding := mustObject(t, defs["finding"], "$defs.finding")
	assertRequiredFields(t, finding, "code", "category", "severity", "message", "blocking")

	properties := mustObject(t, finding["properties"], "$defs.finding.properties")
	category := mustObject(t, properties["category"], "$defs.finding.properties.category")
	assertStringEnum(t, category, []string{
		"AUTHORITY",
		"CONTRACT",
		"EVIDENCE",
		"INTERNAL",
		"POLICY",
		"PROTOCOL",
		"SUBJECT",
		"VERIFICATION",
		"WAIVER",
	})
}

func TestReceiptWaiversPreserveAuthorityAndScope(t *testing.T) {
	root := readSchema(t, "receipt.v0.schema.json")
	properties := mustObject(t, root["properties"], "properties")
	waivers := mustObject(t, properties["waivers"], "properties.waivers")
	items := mustObject(t, waivers["items"], "properties.waivers.items")
	if got := items["$ref"]; got != "#/$defs/waiver_record" {
		t.Fatalf("properties.waivers.items.$ref = %#v, want #/$defs/waiver_record", got)
	}

	defs := mustObject(t, root["$defs"], "$defs")
	waiver := mustObject(t, defs["waiver_record"], "$defs.waiver_record")
	assertRequiredFields(t, waiver,
		"id",
		"target",
		"scope",
		"reason",
		"accepted_risk",
		"approver",
		"issued_at",
		"expires_at",
		"status",
		"digest",
	)

	waiverProperties := mustObject(t, waiver["properties"], "$defs.waiver_record.properties")
	assertRef(t, waiverProperties["target"], "#/$defs/waiver_target", "waiver target")
	assertRef(t, waiverProperties["scope"], "#/$defs/waiver_scope", "waiver scope")
	assertRef(t, waiverProperties["approver"], "#/$defs/waiver_approver", "waiver approver")
	assertRef(t, waiverProperties["issued_at"], "#/$defs/rfc3339_timestamp", "waiver issued_at")
	assertRef(t, waiverProperties["expires_at"], "#/$defs/rfc3339_timestamp", "waiver expires_at")

	target := mustObject(t, defs["waiver_target"], "$defs.waiver_target")
	assertRequiredFields(t, target, "kind", "id")
	targetProperties := mustObject(t, target["properties"], "$defs.waiver_target.properties")
	targetKind := mustObject(t, targetProperties["kind"], "$defs.waiver_target.properties.kind")
	assertStringEnum(t, targetKind, []string{"FINDING", "REQUIREMENT"})

	scope := mustObject(t, defs["waiver_scope"], "$defs.waiver_scope")
	assertRequiredFields(t, scope, "repository_uri", "work_unit_id")
	scopeProperties := mustObject(t, scope["properties"], "$defs.waiver_scope.properties")
	if _, ok := scopeProperties["head_revision"]; !ok {
		t.Fatal("waiver scope does not expose optional head_revision")
	}

	approver := mustObject(t, defs["waiver_approver"], "$defs.waiver_approver")
	assertRequiredFields(t, approver, "identity", "authority")

	status := mustObject(t, waiverProperties["status"], "$defs.waiver_record.properties.status")
	assertStringEnum(t, status, []string{"INVALID", "VALID"})
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

func findConditionalRule(t *testing.T, root map[string]any, property string, value any) map[string]any {
	t.Helper()
	rules := mustArray(t, root["allOf"], "allOf")
	for _, raw := range rules {
		rule := mustObject(t, raw, "conditional rule")
		condition, ok := rule["if"].(map[string]any)
		if !ok {
			continue
		}
		properties, ok := condition["properties"].(map[string]any)
		if !ok {
			continue
		}
		propertySchema, ok := properties[property].(map[string]any)
		if ok && reflect.DeepEqual(propertySchema["const"], value) {
			return rule
		}
	}
	t.Fatalf("no conditional rule found for %s = %#v", property, value)
	return nil
}

func mustObject(t *testing.T, value any, path string) map[string]any {
	t.Helper()
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s = %#v, want object", path, value)
	}
	return object
}

func mustArray(t *testing.T, value any, path string) []any {
	t.Helper()
	array, ok := value.([]any)
	if !ok {
		t.Fatalf("%s = %#v, want array", path, value)
	}
	return array
}

func assertRequiredFields(t *testing.T, schema map[string]any, fields ...string) {
	t.Helper()
	requiredRaw := mustArray(t, schema["required"], "required")
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

func assertRef(t *testing.T, value any, want, path string) {
	t.Helper()
	object := mustObject(t, value, path)
	if got := object["$ref"]; got != want {
		t.Fatalf("%s.$ref = %#v, want %q", path, got, want)
	}
}

func assertConst(t *testing.T, value any, want any, path string) {
	t.Helper()
	object := mustObject(t, value, path)
	if got := object["const"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("%s.const = %#v, want %#v", path, got, want)
	}
}

func assertStringEnum(t *testing.T, schema map[string]any, want []string) {
	t.Helper()
	raw := mustArray(t, schema["enum"], "enum")
	got := make([]string, 0, len(raw))
	for _, value := range raw {
		text, ok := value.(string)
		if !ok {
			t.Fatalf("enum item = %#v, want string", value)
		}
		got = append(got, text)
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("enum = %#v, want %#v", got, want)
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
