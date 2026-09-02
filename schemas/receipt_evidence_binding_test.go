package schemas_test

import "testing"

func TestValidRequirementResultRequiresEvidenceReferences(t *testing.T) {
	root := readSchema(t, "receipt.v0.schema.json")
	defs := mustObject(t, root["$defs"], "$defs")
	requirementResult := mustObject(t, defs["requirement_result"], "$defs.requirement_result")
	properties := mustObject(t, requirementResult["properties"], "$defs.requirement_result.properties")
	evidenceIDs := mustObject(t, properties["evidence_ids"], "$defs.requirement_result.properties.evidence_ids")

	if got := evidenceIDs["type"]; got != "array" {
		t.Fatalf("evidence_ids.type = %#v, want array", got)
	}
	assertNumber(t, evidenceIDs["minItems"], 1, "evidence_ids.minItems")
	if got, ok := evidenceIDs["uniqueItems"].(bool); !ok || !got {
		t.Fatalf("evidence_ids.uniqueItems = %#v, want true", evidenceIDs["uniqueItems"])
	}
	items := mustObject(t, evidenceIDs["items"], "evidence_ids.items")
	if got := items["pattern"]; got != canonicalIdentifierPattern {
		t.Fatalf("evidence_ids.items.pattern = %#v, want %q", got, canonicalIdentifierPattern)
	}

	rule := findObjectConditionalRule(t, mustArray(t, requirementResult["allOf"], "$defs.requirement_result.allOf"), "evidence_state", "VALID")
	then := mustObject(t, rule["then"], "valid-evidence.then")
	assertRequiredFields(t, then, "outcome", "evidence_ids")
}

func TestReceiptRequiresSuppliedEvidenceWhenAnyResultIsValid(t *testing.T) {
	root := readSchema(t, "receipt.v0.schema.json")
	rule := findContainsStateConditional(t, root, "requirement_results", "evidence_state", "VALID")
	then := mustObject(t, rule["then"], "valid-result.then")
	properties := mustObject(t, then["properties"], "valid-result.then.properties")
	evidence := mustObject(t, properties["evidence"], "valid-result.then.properties.evidence")
	assertNumber(t, evidence["minItems"], 1, "valid-result evidence.minItems")
}

func TestEvidenceReferencesUseCanonicalIdentifiers(t *testing.T) {
	root := readSchema(t, "receipt.v0.schema.json")
	defs := mustObject(t, root["$defs"], "$defs")
	reference := mustObject(t, defs["evidence_reference"], "$defs.evidence_reference")
	properties := mustObject(t, reference["properties"], "$defs.evidence_reference.properties")
	id := mustObject(t, properties["id"], "$defs.evidence_reference.properties.id")
	if got := id["pattern"]; got != canonicalIdentifierPattern {
		t.Fatalf("evidence reference id pattern = %#v, want %q", got, canonicalIdentifierPattern)
	}
}

func findContainsStateConditional(t *testing.T, root map[string]any, collection, property string, value any) map[string]any {
	t.Helper()
	for _, raw := range mustArray(t, root["allOf"], "allOf") {
		rule := mustObject(t, raw, "conditional rule")
		condition, ok := rule["if"].(map[string]any)
		if !ok {
			continue
		}
		properties, ok := condition["properties"].(map[string]any)
		if !ok {
			continue
		}
		collectionSchema, ok := properties[collection].(map[string]any)
		if !ok {
			continue
		}
		contains, ok := collectionSchema["contains"].(map[string]any)
		if !ok {
			continue
		}
		containsProperties, ok := contains["properties"].(map[string]any)
		if !ok {
			continue
		}
		propertySchema, ok := containsProperties[property].(map[string]any)
		if ok && propertySchema["const"] == value {
			return rule
		}
	}
	t.Fatalf("no conditional found for %s contains %s = %#v", collection, property, value)
	return nil
}
