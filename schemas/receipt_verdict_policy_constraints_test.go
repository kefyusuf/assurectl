package schemas_test

import (
	"reflect"
	"sort"
	"testing"
)

const canonicalIdentifierPattern = "^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$"

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

func TestIndeterminateVerdictRequiresIndeterminateOrBlockingFindingWithoutEstablishedFailure(t *testing.T) {
	root := readSchema(t, "receipt.v0.schema.json")
	rule := findConditionalRule(t, root, "verification_verdict", "INDETERMINATE")
	then := mustObject(t, rule["then"], "indeterminate.then")
	properties := mustObject(t, then["properties"], "indeterminate.then.properties")
	requirementResults := mustObject(t, properties["requirement_results"], "indeterminate.requirement_results")

	notSchema := mustObject(t, requirementResults["not"], "indeterminate.requirement_results.not")
	failureContains := mustObject(t, notSchema["contains"], "indeterminate.requirement_results.not.contains")
	failureProperties := mustObject(t, failureContains["properties"], "indeterminate failure properties")
	assertConst(t, failureProperties["evidence_state"], "VALID", "indeterminate forbidden failure evidence_state")
	assertConst(t, failureProperties["outcome"], "FAILED", "indeterminate forbidden failure outcome")
	assertRequiredFields(t, failureContains, "evidence_state", "outcome")

	alternatives := mustArray(t, then["anyOf"], "indeterminate.then.anyOf")
	if len(alternatives) != 2 {
		t.Fatalf("indeterminate alternatives = %d, want 2", len(alternatives))
	}

	var foundIndeterminateResult bool
	var foundBlockingFinding bool
	for _, raw := range alternatives {
		alternative := mustObject(t, raw, "indeterminate alternative")
		alternativeProperties := mustObject(t, alternative["properties"], "indeterminate alternative properties")
		if requirementResultsRaw, ok := alternativeProperties["requirement_results"]; ok {
			indeterminateResults := mustObject(t, requirementResultsRaw, "indeterminate alternative requirement_results")
			contains := mustObject(t, indeterminateResults["contains"], "indeterminate alternative requirement_results.contains")
			assertIndeterminatePredicate(t, contains)
			assertNumber(t, indeterminateResults["minContains"], 1, "indeterminate alternative minContains")
			assertRequiredFields(t, alternative, "requirement_results")
			foundIndeterminateResult = true
		}
		if findingsRaw, ok := alternativeProperties["findings"]; ok {
			findings := mustObject(t, findingsRaw, "indeterminate alternative findings")
			contains := mustObject(t, findings["contains"], "indeterminate alternative findings.contains")
			containsProperties := mustObject(t, contains["properties"], "indeterminate blocking finding properties")
			assertConst(t, containsProperties["blocking"], true, "indeterminate blocking finding")
			assertRequiredFields(t, contains, "blocking")
			assertNumber(t, findings["minContains"], 1, "indeterminate blocking finding minContains")
			assertRequiredFields(t, alternative, "findings")
			foundBlockingFinding = true
		}
	}
	if !foundIndeterminateResult || !foundBlockingFinding {
		t.Fatalf("indeterminate alternatives missing: requirement=%v blocking-finding=%v", foundIndeterminateResult, foundBlockingFinding)
	}
}

func TestRequirementResultUsesCanonicalIdentifierPattern(t *testing.T) {
	root := readSchema(t, "receipt.v0.schema.json")
	defs := mustObject(t, root["$defs"], "$defs")
	requirementResult := mustObject(t, defs["requirement_result"], "$defs.requirement_result")
	properties := mustObject(t, requirementResult["properties"], "$defs.requirement_result.properties")
	requirementID := mustObject(t, properties["requirement_id"], "$defs.requirement_result.properties.requirement_id")
	if got := requirementID["pattern"]; got != canonicalIdentifierPattern {
		t.Fatalf("requirement_id.pattern = %#v, want %q", got, canonicalIdentifierPattern)
	}
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
	baseline := mustObject(t, sources["contains"], "authoritative.policy.sources.contains")
	baselineProperties := mustObject(t, baseline["properties"], "authoritative baseline properties")
	assertConst(t, baselineProperties["layer"], "protocol_baseline", "authoritative baseline layer")
	assertConst(t, baselineProperties["authority_basis"], "BUILTIN", "authoritative baseline authority_basis")
	assertConst(t, baselineProperties["trust_status"], "TRUSTED", "authoritative baseline trust_status")
	assertRequiredFields(t, baseline, "layer", "authority_basis", "trust_status")
	assertNumber(t, sources["minContains"], 1, "authoritative baseline minContains")
	assertNumber(t, sources["maxContains"], 1, "authoritative baseline maxContains")

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
			t.Fatalf("authoritative source layer const = %#v, want non-empty string", layerSchema["const"])
		}
		basisSchema := mustObject(t, optionProperties["authority_basis"], "authoritative source authority_basis")
		basis, ok := basisSchema["const"].(string)
		if !ok || basis == "" {
			t.Fatalf("authority basis for %s = %#v, want non-empty const", layer, basisSchema["const"])
		}
		got[layer] = append(got[layer], basis)
		assertRequiredFields(t, option, "layer", "authority_basis")

		if basis == "TRUSTED_BASE" {
			assertConst(t, optionProperties["revision_binding"], "SUBJECT_BASE", "trusted-base revision_binding")
			assertRequiredFields(t, option, "revision_binding")
			assertProhibitsField(t, option, "source_revision")
		}
	}
	for layer := range got {
		sort.Strings(got[layer])
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

func TestTrustedBaseSourcesUseSubjectBaseBinding(t *testing.T) {
	root := readSchema(t, "receipt.v0.schema.json")
	defs := mustObject(t, root["$defs"], "$defs")
	policySource := mustObject(t, defs["policy_source"], "$defs.policy_source")
	properties := mustObject(t, policySource["properties"], "$defs.policy_source.properties")
	revisionBinding := mustObject(t, properties["revision_binding"], "$defs.policy_source.properties.revision_binding")
	assertStringEnum(t, revisionBinding, []string{"SUBJECT_BASE"})

	rules := mustArray(t, policySource["allOf"], "$defs.policy_source.allOf")
	trustedBaseRule := findAuthorityBasisRule(t, rules, "TRUSTED_BASE")
	trustedThen := mustObject(t, trustedBaseRule["then"], "trusted-base.then")
	trustedProperties := mustObject(t, trustedThen["properties"], "trusted-base.then.properties")
	assertConst(t, trustedProperties["revision_binding"], "SUBJECT_BASE", "trusted-base revision_binding")
	assertRequiredFields(t, trustedThen, "revision_binding")
	assertProhibitsField(t, trustedThen, "source_revision")

	candidateRule := findAuthorityBasisRule(t, rules, "CANDIDATE_HEAD")
	candidateThen := mustObject(t, candidateRule["then"], "candidate-head.then")
	assertRequiredFields(t, candidateThen, "source_revision")
	assertProhibitsField(t, candidateThen, "revision_binding")
}

func findAuthorityBasisRule(t *testing.T, rules []any, basis string) map[string]any {
	t.Helper()
	for _, raw := range rules {
		rule := mustObject(t, raw, "policy-source conditional")
		condition, ok := rule["if"].(map[string]any)
		if !ok {
			continue
		}
		properties, ok := condition["properties"].(map[string]any)
		if !ok {
			continue
		}
		authorityBasis, ok := properties["authority_basis"].(map[string]any)
		if ok && authorityBasis["const"] == basis {
			return rule
		}
	}
	t.Fatalf("no policy-source rule found for authority_basis %q", basis)
	return nil
}

func assertProhibitsField(t *testing.T, schema map[string]any, field string) {
	t.Helper()
	notSchema := mustObject(t, schema["not"], "not")
	required := mustArray(t, notSchema["required"], "not.required")
	if len(required) != 1 || required[0] != field {
		t.Fatalf("prohibited field = %#v, want %q", required, field)
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
