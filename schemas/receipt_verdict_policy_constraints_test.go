package schemas_test

import (
	"reflect"
	"sort"
	"testing"
)

func TestPassedVerdictRequiresOnlyEstablishedPasses(t *testing.T) {
	root := readSchema(t, "receipt.v0.schema.json")
	rule := findConditionalRule(t, root, "verification_verdict", "PASSED")
	then := mustObject(t, rule["then"], "passed.then")
	properties := mustObject(t, then["properties"], "passed.then.properties")
	requirementResults := mustObject(t, properties["requirement_results"], "passed.requirement_results")
	items := mustObject(t, requirementResults["items"], "passed.requirement_results.items")
	itemProperties := mustObject(t, items["properties"], "passed.requirement_results.items.properties")
	assertConst(t, itemProperties["evidence_state"], "VALID", "passed evidence_state")
	assertConst(t, itemProperties["outcome"], "PASSED", "passed outcome")
	assertRequiredFields(t, items, "evidence_state", "outcome")
}

func TestFailedVerdictRequiresEstablishedFailure(t *testing.T) {
	root := readSchema(t, "receipt.v0.schema.json")
	rule := findConditionalRule(t, root, "verification_verdict", "FAILED")
	then := mustObject(t, rule["then"], "failed.then")
	properties := mustObject(t, then["properties"], "failed.then.properties")
	requirementResults := mustObject(t, properties["requirement_results"], "failed.requirement_results")
	contains := mustObject(t, requirementResults["contains"], "failed.requirement_results.contains")
	containsProperties := mustObject(t, contains["properties"], "failed.requirement_results.contains.properties")
	assertConst(t, containsProperties["evidence_state"], "VALID", "failed evidence_state")
	assertConst(t, containsProperties["outcome"], "FAILED", "failed outcome")
	assertRequiredFields(t, contains, "evidence_state", "outcome")
	assertNumber(t, requirementResults["minContains"], 1, "failed requirement_results.minContains")
}

func TestIndeterminateVerdictRequiresIndeterminateWithoutEstablishedFailure(t *testing.T) {
	root := readSchema(t, "receipt.v0.schema.json")
	rule := findConditionalRule(t, root, "verification_verdict", "INDETERMINATE")
	then := mustObject(t, rule["then"], "indeterminate.then")
	properties := mustObject(t, then["properties"], "indeterminate.then.properties")
	requirementResults := mustObject(t, properties["requirement_results"], "indeterminate.requirement_results")

	contains := mustObject(t, requirementResults["contains"], "indeterminate.requirement_results.contains")
	alternatives := mustArray(t, contains["anyOf"], "indeterminate.requirement_results.contains.anyOf")
	if len(alternatives) != 2 {
		t.Fatalf("indeterminate alternatives = %d, want 2", len(alternatives))
	}

	var foundUnavailableState bool
	var foundErroredValidState bool
	for _, raw := range alternatives {
		alternative := mustObject(t, raw, "indeterminate alternative")
		alternativeProperties := mustObject(t, alternative["properties"], "indeterminate alternative properties")
		evidenceState := mustObject(t, alternativeProperties["evidence_state"], "indeterminate evidence_state")
		if values, ok := evidenceState["enum"]; ok {
			assertStringEnum(t, mustObject(t, map[string]any{"enum": values}, "unavailable evidence state"), []string{"INVALID", "MISSING", "STALE", "UNTRUSTED"})
			assertRequiredFields(t, alternative, "evidence_state")
			foundUnavailableState = true
			continue
		}
		if evidenceState["const"] == "VALID" {
			assertConst(t, alternativeProperties["outcome"], "ERROR", "indeterminate valid outcome")
			assertRequiredFields(t, alternative, "evidence_state", "outcome")
			foundErroredValidState = true
		}
	}
	if !foundUnavailableState || !foundErroredValidState {
		t.Fatalf("indeterminate alternatives missing: unavailable=%v errored-valid=%v", foundUnavailableState, foundErroredValidState)
	}
	assertNumber(t, requirementResults["minContains"], 1, "indeterminate requirement_results.minContains")

	notSchema := mustObject(t, requirementResults["not"], "indeterminate.requirement_results.not")
	failureContains := mustObject(t, notSchema["contains"], "indeterminate.requirement_results.not.contains")
	failureProperties := mustObject(t, failureContains["properties"], "indeterminate failure properties")
	assertConst(t, failureProperties["evidence_state"], "VALID", "indeterminate forbidden failure evidence_state")
	assertConst(t, failureProperties["outcome"], "FAILED", "indeterminate forbidden failure outcome")
	assertRequiredFields(t, failureContains, "evidence_state", "outcome")
}

func TestAuthoritativePolicySourcesAreTrustedAndLayerCompatible(t *testing.T) {
	root := readSchema(t, "receipt.v0.schema.json")
	rule := findAuthoritativeNonBlockedRule(t, root)
	then := mustObject(t, rule["then"], "authoritative.then")
	properties := mustObject(t, then["properties"], "authoritative.then.properties")
	policy := mustObject(t, properties["policy"], "authoritative.policy")
	policyProperties := mustObject(t, policy["properties"], "authoritative.policy.properties")
	assertConst(t, policyProperties["trust_status"], "TRUSTED", "authoritative policy trust_status")

	sources := mustObject(t, policyProperties["sources"], "authoritative.policy.sources")
	items := mustObject(t, sources["items"], "authoritative.policy.sources.items")
	itemProperties := mustObject(t, items["properties"], "authoritative.policy.sources.items.properties")
	assertConst(t, itemProperties["trust_status"], "TRUSTED", "authoritative source trust_status")
	assertRequiredFields(t, items, "layer", "authority_basis", "trust_status")

	got := make(map[string][]string)
	for _, raw := range mustArray(t, items["oneOf"], "authoritative source oneOf") {
		option := mustObject(t, raw, "authoritative source option")
		optionProperties := mustObject(t, option["properties"], "authoritative source option properties")
		layerSchema := mustObject(t, optionProperties["layer"], "authoritative source layer")
		layer, ok := layerSchema["const"].(string)
		if !ok || layer == "" {
			t.Fatalf("authoritative source layer const = %#v, want string", layerSchema["const"])
		}
		basisSchema := mustObject(t, optionProperties["authority_basis"], "authoritative source authority_basis")
		switch {
		case basisSchema["const"] != nil:
			basis, ok := basisSchema["const"].(string)
			if !ok {
				t.Fatalf("authority basis const = %#v, want string", basisSchema["const"])
			}
			got[layer] = []string{basis}
		case basisSchema["enum"] != nil:
			for _, value := range mustArray(t, basisSchema["enum"], "authority basis enum") {
				basis, ok := value.(string)
				if !ok {
					t.Fatalf("authority basis enum item = %#v, want string", value)
				}
				got[layer] = append(got[layer], basis)
			}
			sort.Strings(got[layer])
		default:
			t.Fatalf("authority basis for %s has neither const nor enum", layer)
		}
		assertRequiredFields(t, option, "layer", "authority_basis")
	}

	want := map[string][]string{
		"protocol_baseline":  {"BUILTIN"},
		"organization":       {"PROTECTED_CONTROL_PLANE"},
		"project":            {"TRUSTED_BASE"},
		"assurance_profile":  {"PROTECTED_CONTROL_PLANE", "TRUSTED_BASE"},
		"work_unit_contract": {"APPROVED_CONTRACT"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("authoritative policy source compatibility = %#v, want %#v", got, want)
	}
}

func findAuthoritativeNonBlockedRule(t *testing.T, root map[string]any) map[string]any {
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
		authority, ok := properties["authority"].(map[string]any)
		if !ok || authority["const"] != "AUTHORITATIVE" {
			continue
		}
		decision, ok := properties["completion_decision"].(map[string]any)
		if !ok {
			continue
		}
		notSchema, ok := decision["not"].(map[string]any)
		if ok && notSchema["const"] == "BLOCKED" {
			return rule
		}
	}
	t.Fatal("authoritative non-blocked conditional not found")
	return nil
}
