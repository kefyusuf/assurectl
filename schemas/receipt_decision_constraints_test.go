package schemas_test

import (
	"reflect"
	"testing"
)

func TestAcceptedWithRiskRequiresEstablishedWaivedFailures(t *testing.T) {
	root := readSchema(t, "receipt.v0.schema.json")
	rule := findConditionalRule(t, root, "completion_decision", "ACCEPTED_WITH_RISK")
	then := mustObject(t, rule["then"], "accepted_with_risk.then")
	properties := mustObject(t, then["properties"], "accepted_with_risk.then.properties")

	requirementResults := mustObject(t, properties["requirement_results"], "accepted_with_risk.requirement_results")
	items := mustObject(t, requirementResults["items"], "accepted_with_risk.requirement_results.items")
	itemProperties := mustObject(t, items["properties"], "accepted_with_risk.requirement_results.items.properties")
	assertConst(t, itemProperties["evidence_state"], "VALID", "accepted-risk evidence_state")
	assertStringEnum(t, mustObject(t, itemProperties["outcome"], "accepted-risk outcome"), []string{"FAILED", "PASSED"})
	assertRequiredFields(t, items, "evidence_state", "outcome")

	contains := mustObject(t, requirementResults["contains"], "accepted_with_risk.requirement_results.contains")
	containsProperties := mustObject(t, contains["properties"], "accepted_with_risk.requirement_results.contains.properties")
	assertConst(t, containsProperties["evidence_state"], "VALID", "accepted-risk failure evidence_state")
	assertConst(t, containsProperties["outcome"], "FAILED", "accepted-risk failure outcome")
	assertRequiredFields(t, contains, "evidence_state", "outcome")
	assertNumber(t, requirementResults["minContains"], 1, "accepted-risk requirement_results.minContains")

	failureRule := findObjectConditionalRule(t, mustArray(t, items["allOf"], "accepted-risk item allOf"), "outcome", "FAILED")
	failureThen := mustObject(t, failureRule["then"], "accepted-risk failure.then")
	failureProperties := mustObject(t, failureThen["properties"], "accepted-risk failure.then.properties")
	assertConst(t, failureProperties["waivable"], true, "accepted-risk failure waivable")
	assertConst(t, failureProperties["waiver_status"], "VALID", "accepted-risk failure waiver_status")
	assertRequiredFields(t, failureThen, "waivable", "waiver_status")

	waivers := mustObject(t, properties["waivers"], "accepted_with_risk.waivers")
	assertNumber(t, waivers["minItems"], 1, "accepted-risk waivers.minItems")
	validWaiver := mustObject(t, waivers["contains"], "accepted-risk waivers.contains")
	validWaiverProperties := mustObject(t, validWaiver["properties"], "accepted-risk waivers.contains.properties")
	assertConst(t, validWaiverProperties["status"], "VALID", "accepted-risk waiver status")
	assertRequiredFields(t, validWaiver, "status")
	assertNumber(t, waivers["minContains"], 1, "accepted-risk waivers.minContains")
}

func TestNonBlockedDecisionsRejectBlockingFindings(t *testing.T) {
	root := readSchema(t, "receipt.v0.schema.json")
	rule := findExcludedConditionalRule(t, root, "completion_decision", "BLOCKED")
	then := mustObject(t, rule["then"], "non-blocked.then")
	properties := mustObject(t, then["properties"], "non-blocked.then.properties")
	findings := mustObject(t, properties["findings"], "non-blocked.findings")
	items := mustObject(t, findings["items"], "non-blocked.findings.items")
	itemProperties := mustObject(t, items["properties"], "non-blocked.findings.items.properties")
	assertConst(t, itemProperties["blocking"], false, "non-blocked finding")
	assertRequiredFields(t, items, "blocking")
}

func findObjectConditionalRule(t *testing.T, rules []any, property string, value any) map[string]any {
	t.Helper()
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

func findExcludedConditionalRule(t *testing.T, root map[string]any, property string, excluded any) map[string]any {
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
		propertySchema, ok := properties[property].(map[string]any)
		if !ok {
			continue
		}
		notSchema, ok := propertySchema["not"].(map[string]any)
		if ok && reflect.DeepEqual(notSchema["const"], excluded) {
			return rule
		}
	}
	t.Fatalf("no exclusion conditional found for %s != %#v", property, excluded)
	return nil
}

func assertNumber(t *testing.T, got any, want float64, path string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %#v, want %#v", path, got, want)
	}
}
