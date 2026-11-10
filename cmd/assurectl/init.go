package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kefyusuf/assurectl/internal/domain"
)

// runInit scaffolds .assurectl/contract.json and .assurectl/policy.json for
// a given work-unit type. Generated documents are starting points, not
// authoritative; hand-authored documents remain first-class.
func runInit(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	workType := flags.String("type", "", "work-unit type (e.g. dependency-upgrade, api-migration)")
	requirementID := flags.String("requirement-id", "", "requirement identifier for the policy")
	evidenceType := flags.String("evidence-type", "", "evidence type the requirement expects")
	outputDir := flags.String("output-dir", ".", "directory to create .assurectl/ in")
	if err := flags.Parse(args); err != nil {
		return initUsageError(err.Error(), stderr)
	}
	if *workType == "" || *requirementID == "" || *evidenceType == "" {
		return initUsageError("--type, --requirement-id and --evidence-type are required", stderr)
	}

	workUnitType := domain.WorkUnitType(underscoresToHyphens(*workType))
	if !workUnitType.Valid() {
		return initUsageError(fmt.Sprintf("unknown work-unit type %q", *workType), stderr)
	}

	assurectlDir := filepath.Join(*outputDir, ".assurectl")
	contractPath := filepath.Join(assurectlDir, "contract.json")
	policyPath := filepath.Join(assurectlDir, "policy.json")

	for _, path := range []string{contractPath, policyPath} {
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintf(stderr, "init: %s already exists; refusing to overwrite\n", path)
			return 2
		}
	}

	contract := map[string]any{
		"schema_version": "assurectl/verification-contract/v0",
		"work_unit": map[string]any{
			"id":        *requirementID + "-work-unit",
			"type":      string(workUnitType),
			"objective": "Scaffolded by assurectl init — edit for your needs",
		},
		"acceptance_criteria": []map[string]any{
			{
				"id":           *requirementID + "-criterion",
				"statement":    "The " + hyphensToSpaces(*workType) + " verification passes.",
				"requirements": []string{*requirementID},
			},
		},
	}
	policy := map[string]any{
		"schema_version": "assurectl/policy/v0",
		"requirements": []map[string]any{
			{
				"id":                     *requirementID,
				"evidence_type":          *evidenceType,
				"waivable":               false,
				"allowed_producer_types": []string{"local-user"},
			},
		},
	}

	if err := os.MkdirAll(assurectlDir, 0o755); err != nil {
		return initError("create .assurectl directory", err, stderr)
	}
	for _, pair := range []struct {
		path string
		data map[string]any
	}{{contractPath, contract}, {policyPath, policy}} {
		encoded, err := json.MarshalIndent(pair.data, "", "  ")
		if err != nil {
			return initError("encode "+pair.path, err, stderr)
		}
		if err := os.WriteFile(pair.path, append(encoded, '\n'), 0o644); err != nil {
			return initError("write "+pair.path, err, stderr)
		}
		fmt.Fprintf(stdout, "created %s\n", pair.path)
	}
	return 0
}

// underscoresToHyphens converts "dependency-upgrade" to "dependency_upgrade".
func underscoresToHyphens(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '-' {
			result[i] = '_'
		} else {
			result[i] = s[i]
		}
	}
	return string(result)
}

// hyphensToSpaces converts "dependency-upgrade" to "dependency upgrade".
func hyphensToSpaces(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '-' {
			result[i] = ' '
		} else {
			result[i] = s[i]
		}
	}
	return string(result)
}

func initError(stage string, err error, stderr io.Writer) int {
	if err != nil {
		fmt.Fprintf(stderr, "init: %s: %v\n", stage, err)
	} else {
		fmt.Fprintf(stderr, "init: %s\n", stage)
	}
	return 2
}

func initUsageError(message string, stderr io.Writer) int {
	fmt.Fprintf(stderr, "init: %s\n\n", message)
	_, _ = io.WriteString(stderr, usage)
	return 64
}
