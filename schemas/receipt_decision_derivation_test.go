package schemas_test

import (
	"reflect"
	"sort"
	"testing"
)

func TestRejectedRequiresEstablishedUncoveredFailure(t *testing.T) {
	root := readSchema(t, "receipt.v0.schema.json")
	rule := findConditionalRule(t, root, "completion_decision", "REJECTED")
	then := mustObject(t, rule["then"], "rejected.then")
	properties := mustObject(t, then["properties"], "rejected.then.properties")
	requirementResults := mustObject(t, properties["requirement_results"], "rejected.requirement_results")

	items := mustObject(t, requirementResults["items"], "rejected.requirement_results.items")
	itemProperties := mustObject(t, items["properties"], "rejected.requirement_results.items.properties")
	assertConst(t, itemProperties["evidence_state"], "VALID", "rejected evidence_state")
	assertStringEnum(t, mustObject(t, itemProperties["outcome"], "rejected outcome"), []string{"FAILED", "PASSED"})
	assertRequiredFields(t, items, "evidence_state", "outcome")

	contains := mustObject(t, requirementResults["contains"], "rejected.requirement_results.contains")
	containsProperties := mustObject(t, contains["properties"], "rejected.requirement_results.contains.properties")
	assertConst(t, containsProperties["evidence_state"], "VALID", "rejected failure evidence_state")
	assertConst(t, containsProperties["outcome"], "FAILED", "rejected failure outcome")
	assertRequiredFields(t, contains, "evidence_state", "outcome")
	assertNumber(t, requirementResults["minContains"], 1, "rejected requirement_results.minContains")

	alternatives := mustArray(t, contains["anyOf"], "rejected uncovered failure alternatives")
	var foundNonWaivable bool
	var foundMissingValidWaiver bool
	for _, raw := range alternatives {
		alternative := mustObject(t, raw, "rejected uncovered failure alternative")
		alternativeProperties := mustObject(t, alternative["properties"], "rejected uncovered failure properties")
		if waivable, ok := alternativeProperties["waivable"]; ok {
			assertConst(t, waivable, false, "rejected non-waivable failure")
			assertRequiredFields(t, alternative, "waivable")
			foundNonWaivable = true
		}
		if waiverStatus, ok := alternativeProperties["waiver_status"]; ok {
			assertStringEnum(t, mustObject(t, waiverStatus, "rejected waiver status"), []string{"INVALID", "NOT_APPLICABLE"})
			assertRequiredFields(t, alternative, "waiver_status")
			foundMissingValidWaiver = true
		}
	}
	if !foundNonWaivable || !foundMissingValidWaiver {
		t.Fatalf("rejected uncovered-failure alternatives missing: non-waivable=%v missing-valid-waiver=%v", foundNonWaivable, foundMissingValidWaiver)
	}
}

func TestBlockedRequiresIndeterminateResultOrBlockingFinding(t *testing.T) {
	root := readSchema(t, "receipt.v0.schema.json")
	rule := findConditionalRule(t, root, "completion_decision", "BLOCKED")
	then := mustObject(t, rule["then"], "blocked.then")
	alternatives := mustArray(t, then["anyOf"], "blocked.then.anyOf")
	if len(alternatives) != 2 {
		t.Fatalf("blocked alternatives = %d, want 2", len(alternatives))
	}

	var foundIndeterminate bool
	var foundBlockingFinding bool
	for _, raw := range alternatives {
		alternative := mustObject(t, raw, "blocked alternative")
		properties := mustObject(t, alternative["properties"], "blocked alternative properties")
		if requirementResultsRaw, ok := properties["requirement_results"]; ok {
			requirementResults := mustObject(t, requirementResultsRaw, "blocked requirement_results")
			contains := mustObject(t, requirementResults["contains"], "blocked requirement_results.contains")
			assertIndeterminatePredicate(t, contains)
			assertNumber(t, requirementResults["minContains"], 1, "blocked requirement_results.minContains")
			assertRequiredFields(t, alternative, "requirement_results")
			foundIndeterminate = true
		}
		if findingsRaw, ok := properties["findings"]; ok {
			findings := mustObject(t, findingsRaw, "blocked findings")
			contains := mustObject(t, findings["contains"], "blocked findings.contains")
			containsProperties := mustObject(t, contains["properties"], "blocked findings.contains.properties")
			assertConst(t, containsProperties["blocking"], true, "blocked finding")
			assertRequiredFields(t, contains, "blocking")
			assertNumber(t, findings["minContains"], 1, "blocked findings.minContains")
			assertRequiredFields(t, alternative, "findings")
			foundBlockingFinding = true
		}
	}
	if !foundIndeterminate || !foundBlockingFinding {
		t.Fatalf("blocked alternatives missing: indeterminate=%v blocking-finding=%v", foundIndeterminate, foundBlockingFinding)
	}
}

func assertIndeterminatePredicate(t *testing.T, predicate map[string]any) {
	t.Helper()
	alternatives := mustArray(t, predicate["anyOf"], "indeterminate predicate anyOf")
	if len(alternatives) != 2 {
		t.Fatalf("indeterminate predicate alternatives = %d, want 2", len(alternatives))
	}

	var gotUnavailable []string
	var gotErrored bool
	for _, raw := range alternatives {
		alternative := mustObject(t, raw, "indeterminate predicate alternative")
		properties := mustObject(t, alternative["properties"], "indeterminate predicate properties")
		evidenceState := mustObject(t, properties["evidence_state"], "indeterminate predicate evidence_state")
		if enumRaw, ok := evidenceState["enum"]; ok {
			for _, value := range mustArray(t, enumRaw, "unavailable evidence-state enum") {
				text, ok := value.(string)
				if !ok {
					t.Fatalf("unavailable evidence-state value = %#v, want string", value)
				}
				gotUnavailable = append(gotUnavailable, text)
			}
			assertRequiredFields(t, alternative, "evidence_state")
			continue
		}
		if evidenceState["const"] == "VALID" {
			assertConst(t, properties["outcome"], "ERROR", "errored valid result")
			assertRequiredFields(t, alternative, "evidence_state", "outcome")
			gotErrored = true
		}
	}

	sort.Strings(gotUnavailable)
	wantUnavailable := []string{"INVALID", "MISSING", "STALE", "UNTRUSTED"}
	if !reflect.DeepEqual(gotUnavailable, wantUnavailable) || !gotErrored {
		t.Fatalf("indeterminate predicate = unavailable %#v, errored=%v; want unavailable %#v, errored=true", gotUnavailable, gotErrored, wantUnavailable)
	}
}
