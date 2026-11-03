package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/kefyusuf/assurectl/internal/cicontext"
	"github.com/kefyusuf/assurectl/internal/domain"
	"github.com/kefyusuf/assurectl/internal/verify"
)

// runVerify handles `assurectl verify`. It parses flags only, supplies its
// own UTC wall-clock reading as the explicit evaluation instant, and maps the
// outcome to output channels and exit codes; no domain decisions happen here.
func runVerify(args []string, stdout, stderr io.Writer) int {
	var base, head, repository, waivers string
	var ci bool
	for i := 0; i < len(args); i++ {
		name, value, hasValue := cutFlag(args[i])
		switch name {
		case "--base", "--head", "--repository", "--waivers":
			if !hasValue {
				if i+1 >= len(args) {
					return verifyUsageError(fmt.Sprintf("flag %s requires a value", name), stderr)
				}
				i++
				value = args[i]
			}
			switch name {
			case "--base":
				base = value
			case "--head":
				head = value
			case "--repository":
				repository = value
			case "--waivers":
				waivers = value
			}
		case "--ci":
			if hasValue {
				if value != "true" && value != "false" {
					return verifyUsageError("flag --ci accepts true or false", stderr)
				}
				ci = value == "true"
			} else {
				ci = true
			}
		default:
			return verifyUsageError(fmt.Sprintf("unexpected argument: %s", args[i]), stderr)
		}
	}
	if base == "" {
		return verifyUsageError("required flag --base is missing", stderr)
	}
	if head == "" {
		head = "HEAD"
	}

	var ciContext *cicontext.Context
	if ci {
		context, err := cicontext.Read(os.Environ())
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "verify: %v\n", err)
			return 2
		}
		ciContext = &context
	}

	result, err := verify.Run(context.Background(), ".", verify.Options{
		RepositoryURI: repository,
		BaseRef:       base,
		HeadRef:       head,
		WaiversPath:   waivers,
		CI:            ciContext,
		EvaluatedAt:   time.Now().UTC().Truncate(time.Second),
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "verify: %v\n", err)
		return 2
	}

	receiptJSON, err := result.Receipt.JSON()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "verify: render receipt: %v\n", err)
		return 2
	}
	digest, err := result.Receipt.Digest()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "verify: digest receipt: %v\n", err)
		return 2
	}

	_, _ = stdout.Write(receiptJSON)
	_, _ = fmt.Fprintln(stdout)
	_, _ = fmt.Fprintf(stderr, "verdict=%s decision=%s receipt_sha256=%s\n",
		result.Receipt.VerificationVerdict, result.Receipt.CompletionDecision, digest.Value)

	switch result.Receipt.CompletionDecision {
	case domain.DecisionApproved, domain.DecisionAcceptedWithRisk:
		return 0
	case domain.DecisionRejected:
		return 1
	default:
		return 2
	}
}

// cutFlag splits "--name=value" into name and value; a bare "--name" reports
// hasValue=false.
func cutFlag(argument string) (name, value string, hasValue bool) {
	if !strings.HasPrefix(argument, "--") {
		return argument, "", false
	}
	name = argument
	if index := strings.IndexByte(argument, '='); index >= 0 {
		return "--" + argument[2:index], argument[index+1:], true
	}
	return name, "", false
}

func verifyUsageError(message string, stderr io.Writer) int {
	_, _ = fmt.Fprintf(stderr, "%s\n\n", message)
	_, _ = io.WriteString(stderr, usage)
	return 64
}
